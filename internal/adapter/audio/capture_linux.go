//go:build linux

package audio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"interagent/internal/port"
)

// defaultSystemDevice is the pulse special source name for the default
// output's monitor: it captures whatever the default sink plays (system
// audio) and needs no portal consent for a native (non-sandboxed) app.
const defaultSystemDevice = "@DEFAULT_SINK@.monitor"

// pulseReader is the transport seam for the cgo pulse-simple reader
// (capture_pulse_linux.go) or the honest !cgo stub (capture_pulse_fallback_linux.go).
type pulseReader interface {
	Read(buf []byte) (int, error)
	Close() error
}

// pulseSourceInfo is the raw pulse daemon source descriptor; the cgo
// enumerator (Task 2) fills it, the pure-Go mapping shapes it for the port.
type pulseSourceInfo struct {
	Name        string
	Description string
	IsDefault   bool
}

var (
	openPulseReader  func(device string, sampleRate, chunkBytes int) (pulseReader, error)
	listPulseSources func() ([]pulseSourceInfo, error)
)

// pulseChunkBytes is the ~100 ms float32-mono PCM frame size at sampleRate.
func pulseChunkBytes(sampleRate int) int {
	return sampleRate / 10 * 4
}

func pulseReaderErr() error {
	return errors.New("pulse capture is not available (built without cgo)")
}

// pulseHintErr maps a raw pulse error into an actionable message.
func pulseHintErr(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "no such entity"), strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "service"), strings.Contains(msg, "timeout"):
		return errors.New("pulse daemon unreachable — is pipewire-pulse (pulseaudio) running? start it with: systemctl --user start pipewire-pulse")
	default:
		return err
	}
}

func pulseSourcesToDevices(srcs []pulseSourceInfo) []port.AudioDevice {
	out := make([]port.AudioDevice, 0, len(srcs))
	for _, s := range srcs {
		out = append(out, port.AudioDevice{ID: s.Name, Name: s.Description, IsDefault: s.IsDefault})
	}
	return out
}

// pulseReachable checks the pulse runtime sockets (pure Go): pipewire-pulse
// listens on $XDG_RUNTIME_DIR/pulse/native; legacy pulse also tries /tmp/pulse/native.
func pulseReachable() error {
	var candidates []string
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		candidates = append(candidates, filepath.Join(rt, "pulse", "native"))
	}
	candidates = append(candidates, "/tmp/pulse/native")
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && fi.Mode()&os.ModeSocket != 0 {
			return nil
		}
	}
	return errors.New("pulse daemon not reachable (is pipewire-pulse running?)")
}

// MicrophoneCapture pumps one pulse-simple reader in a dedicated goroutine
// that OWNS the reader's lifecycle: the pump reads, and on cancellation or
// read error it closes the reader itself, then closes done, and only then
// returns. Stop()/SetDevice() never free the reader from the caller thread —
// pulse streams are not thread-safe, and C.pa_simple_read blocks for a full
// ~100 ms frame, so a caller-side Close would free the reader while the pump
// is mid-Read (use-after-free).
type MicrophoneCapture struct {
	mu      sync.Mutex
	device  string
	started bool
	done    chan struct{} // closed by the pump goroutine when it fully exits
	cancel  context.CancelFunc
	onChunk func([]byte)
}

func NewMicrophoneCapture() *MicrophoneCapture { return &MicrophoneCapture{} }

func (m *MicrophoneCapture) Start(onChunk func([]byte)) error {
	if openPulseReader == nil {
		return pulseReaderErr()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		return nil
	}
	r, err := openPulseReader(m.device, CaptureSampleRate, pulseChunkBytes(CaptureSampleRate))
	if err != nil {
		return pulseHintErr(err)
	}
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.done = done
	m.onChunk = onChunk
	m.started = true
	go pulsePump(ctx, r, onChunk, done)
	return nil
}

// pulsePump reads with the LOCAL reader reference it was handed, so it never
// reads a field the caller might reassign (no data race). On cancellation or
// read error it stops reading, closes the reader via defer, and only then
// closes done — a joiner of <-done is guaranteed the reader was already freed.
// Deferred order matters (LIFO): Close runs before close(done).
func pulsePump(ctx context.Context, r pulseReader, onChunk func([]byte), done chan struct{}) {
	buf := make([]byte, pulseChunkBytes(CaptureSampleRate))
	defer func() { close(done) }()
	defer func() { _ = r.Close() }()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, err := r.Read(buf)
		if n > 0 && onChunk != nil {
			onChunk(buf[:n])
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err != nil {
			return
		}
	}
}

// Stop cancels the pump and JOINS it (<-done) before returning. On a live mic
// pa_simple_read returns within one ~100 ms frame, so the join is quick; if a
// dead daemon ever made pa_simple_read block indefinitely, Stop would wait for
// the pump rather than free the reader under it — correctness over liveness.
func (m *MicrophoneCapture) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return nil
	}
	m.started = false
	if m.cancel != nil {
		m.cancel()
	}
	done := m.done
	m.cancel = nil
	m.done = nil
	<-done // pump closed the reader itself before closing done
	return nil
}

func (m *MicrophoneCapture) Devices() ([]port.AudioDevice, error) {
	if listPulseSources == nil {
		return nil, pulseReaderErr()
	}
	srcs, err := listPulseSources()
	if err != nil {
		return nil, pulseHintErr(err)
	}
	return pulseSourcesToDevices(srcs), nil
}

func (m *MicrophoneCapture) SetDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.device == id {
		return nil
	}
	if m.started {
		// Same cancel+join as Stop(): the in-use reader is closed by the pump
		// goroutine, never by us while it could be blocked in Read.
		if m.cancel != nil {
			m.cancel()
		}
		done := m.done
		m.cancel = nil
		m.done = nil
		<-done
		if openPulseReader == nil {
			m.started = false
			m.device = id
			return nil
		}
		r, err := openPulseReader(id, CaptureSampleRate, pulseChunkBytes(CaptureSampleRate))
		if err != nil {
			// The old reader is already freed by the pump; stop cleanly rather
			// than claim a live capture that has no reader.
			m.started = false
			return pulseHintErr(err)
		}
		ndone := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		m.done = ndone
		go pulsePump(ctx, r, m.onChunk, ndone)
	}
	m.device = id
	return nil
}

// Reachable reports whether a pulse daemon socket exists (Linux probe, no cgo).
// Honest transport probe — not a consent claim.
func (m *MicrophoneCapture) Reachable() error {
	return pulseReachable()
}

// PulseReachable is the exported transport probe used by the system permissions
// adapter, wired from app.go on every platform. The non-linux stub lives in
// pulse_other.go and is ignored by the darwin/windows permission constructors.
func PulseReachable() error { return pulseReachable() }

// SystemCapture pumps the default sink monitor (system audio) with the same
// pulse-simple reader + owned-pump lifecycle as MicrophoneCapture. Native
// (non-sandboxed) Linux apps need no consent to capture system audio (ADR-013
// amendment), so Start never raises a picker — the only errors are honest
// transport errors (no pulse daemon, unreachable).
type SystemCapture struct {
	mu      sync.Mutex
	device  string
	started bool
	done    chan struct{} // closed by the pump goroutine when it fully exits
	cancel  context.CancelFunc
	onChunk func([]byte)
}

func NewSystemCapture() *SystemCapture {
	return &SystemCapture{device: defaultSystemDevice}
}

func (s *SystemCapture) Start(onChunk func([]byte)) error {
	if openPulseReader == nil {
		return pulseReaderErr()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	device := s.device
	if device == "" {
		device = defaultSystemDevice
	}
	r, err := openPulseReader(device, CaptureSampleRate, pulseChunkBytes(CaptureSampleRate))
	if err != nil {
		return pulseHintErr(err)
	}
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = done
	s.onChunk = onChunk
	s.started = true
	go pulsePump(ctx, r, onChunk, done)
	return nil
}

// Stop cancels the pump and JOINS it (<-done) before returning. Same
// correctness note as MicrophoneCapture: on a live monitor the block lasts one
// ~100 ms frame; Stop never frees the reader from the caller thread.
func (s *SystemCapture) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}
	s.started = false
	if s.cancel != nil {
		s.cancel()
	}
	done := s.done
	s.cancel = nil
	s.done = nil
	<-done // pump closed the reader itself before closing done
	return nil
}

// Devices reports the pulse monitor sources only (the sinks' .monitor
// outputs) — the honest set of system-audio targets. A missing pulse seam
// yields an honest error, never an empty fabricated list.
func (s *SystemCapture) Devices() ([]port.AudioDevice, error) {
	if listPulseSources == nil {
		return nil, pulseReaderErr()
	}
	srcs, err := listPulseSources()
	if err != nil {
		return nil, pulseHintErr(err)
	}
	out := make([]port.AudioDevice, 0, len(srcs))
	for _, src := range srcs {
		if !isMonitorSource(src.Name) {
			continue
		}
		out = append(out, port.AudioDevice{ID: src.Name, Name: src.Description, IsDefault: src.Name == s.device})
	}
	return out, nil
}

// isMonitorSource reports whether a pulse source name is a sink monitor (the
// pulse convention appends ".monitor" to the owning sink's name).
func isMonitorSource(name string) bool {
	return strings.HasSuffix(name, ".monitor")
}

func (s *SystemCapture) SetDevice(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.device == id {
		return nil
	}
	if s.started {
		// Same cancel+join as Stop(): the in-use reader is closed by the pump
		// goroutine, never by us while it could be blocked in Read.
		if s.cancel != nil {
			s.cancel()
		}
		done := s.done
		s.cancel = nil
		s.done = nil
		<-done
		if openPulseReader == nil {
			s.started = false
			s.device = id
			return nil
		}
		r, err := openPulseReader(id, CaptureSampleRate, pulseChunkBytes(CaptureSampleRate))
		if err != nil {
			// The old reader is already freed by the pump; stop cleanly rather
			// than claim a live capture that has no reader.
			s.started = false
			return pulseHintErr(err)
		}
		ndone := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		s.done = ndone
		go pulsePump(ctx, r, s.onChunk, ndone)
	}
	s.device = id
	return nil
}
