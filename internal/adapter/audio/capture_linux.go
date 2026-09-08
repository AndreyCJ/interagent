//go:build linux

package audio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"interagent/internal/adapter/portal"
	"interagent/internal/port"
)

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

type MicrophoneCapture struct {
	mu      sync.Mutex
	reader  pulseReader
	device  string
	started bool
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
	m.reader = r
	m.onChunk = onChunk
	m.started = true
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go m.pump(ctx)
	return nil
}

func (m *MicrophoneCapture) pump(ctx context.Context) {
	buf := make([]byte, pulseChunkBytes(CaptureSampleRate))
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, err := m.reader.Read(buf)
		if n > 0 && m.onChunk != nil {
			m.onChunk(buf[:n])
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
	if m.reader != nil {
		return m.reader.Close()
	}
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
	if m.started && openPulseReader != nil {
		if m.reader != nil {
			if err := m.reader.Close(); err != nil {
				return err
			}
		}
		r, err := openPulseReader(id, CaptureSampleRate, pulseChunkBytes(CaptureSampleRate))
		if err != nil {
			return pulseHintErr(err)
		}
		m.reader = r
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

type SystemCapture struct {
	mu       sync.Mutex
	portal   *portal.ScreenCast
	stream   *portal.Stream
	consumer *pwConsumer
	started  bool
}

func NewSystemCapture(portalAdapter *portal.ScreenCast) *SystemCapture {
	return &SystemCapture{portal: portalAdapter}
}

func (s *SystemCapture) Start(onChunk func([]byte)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	if s.portal == nil {
		return errors.New("system sound capture requires the xdg ScreenCast portal")
	}
	ctx := context.Background()
	if !s.portal.Available(ctx) {
		return errors.New("xdg desktop portal ScreenCast is not available — install xdg-desktop-portal and a backend (hyprland/gtk/wlr)")
	}
	st, err := s.portal.OpenStream(ctx)
	if err != nil {
		return err // includes honest "selection was cancelled" for a dismissed picker
	}
	// TakeFD transfers the fd to the PipeWire consumer; the portal Stream no
	// longer owns it (Ruling 1 — Stream.Close must not double-close a
	// PipeWire-owned descriptor).
	fd, nodeID := st.TakeFD(), int(st.NodeID)
	c, err := newPWConsumer(fd, nodeID, CaptureSampleRate, onChunk)
	if err != nil {
		_ = st.Close()
		return err
	}
	s.stream = st
	s.consumer = c
	s.started = true
	return c.Start()
}

func (s *SystemCapture) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		return nil
	}
	s.started = false
	if s.consumer != nil {
		_ = s.consumer.Close()
		s.consumer = nil
	}
	if s.stream != nil {
		_ = s.stream.Close()
		s.stream = nil
	}
	return nil
}

func (s *SystemCapture) Devices() ([]port.AudioDevice, error) { return nil, nil }
func (s *SystemCapture) SetDevice(id string) error            { return nil }
