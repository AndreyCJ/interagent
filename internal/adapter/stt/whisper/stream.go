package whisper

import (
	"fmt"
	"math"
	"os"
	"time"

	whispercpp "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

// finalizeSilence is the trailing silence (model time) required before the last
// segment counts as a finished phrase.
const finalizeSilence = 400 * time.Millisecond

// streamParams are the silence-gate timings and RMS threshold. Single default
// set today; kept as a struct so calibration can adjust the threshold without
// touching the logic.
type streamParams struct {
	gateTail      time.Duration // trailing RMS window at model rate
	finalize      time.Duration // quiet run length that triggers a phrase-end Process
	backstop      time.Duration // Process cadence while speech is continuous
	maxInterval   time.Duration // hard cap between Processes when the window has energy
	gateThreshold float32       // RMS below which the tail counts as quiet
}

var streamDefaults = streamParams{
	gateTail:      200 * time.Millisecond,
	finalize:      finalizeSilence,
	backstop:      3 * time.Second,
	maxInterval:   4 * time.Second,
	gateThreshold: 0.005,
}

// sttDebugRMS logs the gate state to /tmp/interagent-stt-rms.log (1 line/sec)
// when INTERAGENT_STT_DEBUG_RMS=1 — used for calibrating gateThreshold against
// real system audio.
var sttDebugRMS = os.Getenv("INTERAGENT_STT_DEBUG_RMS") != ""

// streamState is the whisper streaming state machine: a rolling window at the
// model sample rate and single-segment re-transcription driven by a silence
// gate.
//
// The Go binding forces single_segment mode whenever a SegmentCallback is set,
// so each Process call re-transcribes the WHOLE window into one segment (fresh
// results every call — whisper_full_with_state clears result_all up front).
// Process is therefore run ONLY at phrase boundaries: when trailing audio has
// been quiet for finalizeSilence after speech, or as a cadence backstop while
// speech is continuous. Whisper's own VAD (via finalizePhrase in the
// encoder-begin callback) remains the phrase-end authority.
type streamState struct {
	ctx        sttContext
	sampleRate int
	onPartial  func(string)
	onDone     func(string, float64, string)
	params     streamParams

	window   []float32
	partial  string
	segments []whispercpp.Segment
	// lastSegEnd is the end sample (model rate) of the most recent segment and
	// windowLen the window length at the time that segment was produced.
	lastSegEnd int
	windowLen  int
	first      bool // false after the first Process call

	// Silence gate.
	ring               []float32 // trailing model-rate samples (gateTail)
	ringPos            int
	ringCount          int
	speaking           bool
	quietSince         time.Time
	lastProcess        time.Time
	energySinceProcess float64 // sum of |sample| appended since the last Process
	lastDebug          time.Time
}

func newStreamState(ctx sttContext, sampleRate int, onPartial func(string), onDone func(string, float64, string)) *streamState {
	ringLen := int(math.Ceil(streamDefaults.gateTail.Seconds() * float64(modelSampleRate)))
	if ringLen < 1 {
		ringLen = 1
	}
	return &streamState{
		ctx: ctx, sampleRate: sampleRate, onPartial: onPartial, onDone: onDone,
		params: streamDefaults, ring: make([]float32, ringLen), first: true,
		lastProcess: time.Now(),
	}
}

func (s *streamState) ingest(chunk []byte) error {
	samples := bytesToFloat32(chunk)
	rs := resample(samples, s.sampleRate, modelSampleRate)
	s.window = append(s.window, rs...)
	if max := modelSampleRate * windowSeconds; len(s.window) > max {
		s.window = s.window[len(s.window)-max:]
	}
	var energy float64
	for _, v := range rs {
		energy += math.Abs(float64(v))
	}
	s.energySinceProcess += energy
	s.pushTail(rs)
	return s.maybeProcess()
}

// pushTail feeds the gate ring with the newest model-rate samples.
func (s *streamState) pushTail(rs []float32) {
	for _, v := range rs {
		s.ring[s.ringPos] = v
		s.ringPos = (s.ringPos + 1) % len(s.ring)
		if s.ringCount < len(s.ring) {
			s.ringCount++
		}
	}
}

func (s *streamState) resetGate() {
	s.ringPos = 0
	s.ringCount = 0
	s.speaking = false
	s.quietSince = time.Time{}
	s.energySinceProcess = 0
}

// tailRMS is the RMS of the gateTail ring.
func (s *streamState) tailRMS() float32 {
	if s.ringCount == 0 {
		return 0
	}
	var sum float64
	for _, v := range s.ring {
		sum += float64(v) * float64(v)
	}
	return float32(math.Sqrt(sum / float64(s.ringCount)))
}

// maybeProcess runs Process only when the silence gate or a cadence timer says
// a phrase boundary (or stall) happened. During continuous speech or idle
// silence it does nothing — the per-chunk full-window re-transcription that
// caused the 10s latency / 99% CPU is gone.
func (s *streamState) maybeProcess() error {
	now := time.Now()
	tail := s.tailRMS()
	if sttDebugRMS && now.Sub(s.lastDebug) >= time.Second {
		s.lastDebug = now
		f, err := os.OpenFile("/tmp/interagent-stt-rms.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%s tail_rms=%.5f speaking=%v since_process=%v energy=%v\n",
				now.Format("15:04:05.000"), tail, s.speaking, now.Sub(s.lastProcess).Round(time.Millisecond), s.energySinceProcess)
			_ = f.Close()
		}
	}
	switch {
	case tail >= s.params.gateThreshold:
		s.speaking = true
		s.quietSince = time.Time{}
	case s.speaking:
		if s.quietSince.IsZero() {
			s.quietSince = now
		}
		if now.Sub(s.quietSince) >= s.params.finalize {
			return s.process(now)
		}
	}
	if s.speaking && now.Sub(s.lastProcess) >= s.params.backstop {
		return s.process(now)
	}
	if s.energySinceProcess > 0 && now.Sub(s.lastProcess) >= s.params.maxInterval {
		return s.process(now)
	}
	return nil
}

func (s *streamState) process(now time.Time) error {
	s.lastProcess = now
	s.energySinceProcess = 0
	if err := s.ctx.Process(s.window, s.newEncoderBeginCb(), s.newSegmentCb(), nil); err != nil {
		return err
	}
	// The gate runs Process at most once per phrase: this call IS the
	// phrase-end transcription, so the phrase finalizes here — the segment's
	// trailing silence in the window has already elapsed in real time.
	s.finalizePhrase()
	return nil
}

// newEncoderBeginCb fires once per Process call (single_segment mode runs the
// encoder exactly once). Each new run is the moment to check whether the
// previous phrase ended with enough trailing silence to be finalized.
func (s *streamState) newEncoderBeginCb() whispercpp.EncoderBeginCallback {
	return func() bool {
		if s.first {
			s.first = false
			return true
		}
		s.finalizePhrase()
		return true
	}
}

// newSegmentCb replaces the partial with the latest window transcription.
func (s *streamState) newSegmentCb() whispercpp.SegmentCallback {
	return func(seg whispercpp.Segment) {
		s.partial = seg.Text
		s.segments = []whispercpp.Segment{seg}
		s.lastSegEnd = int(math.Ceil(float64(seg.End) * float64(modelSampleRate) / float64(time.Second)))
		s.windowLen = len(s.window)
		if s.onPartial != nil {
			s.onPartial(s.partial)
		}
	}
}

// finalizePhrase emits a done event for the previous phrase when its last
// segment ended with at least finalizeSilence of trailing silence, then
// truncates the window just past the speech so the next phrase starts clean.
func (s *streamState) finalizePhrase() {
	if s.partial == "" {
		return
	}
	if s.lastSegEnd == 0 || s.windowLen-s.lastSegEnd < int(math.Ceil(finalizeSilence.Seconds()*float64(modelSampleRate))) {
		return // still talking
	}
	text := s.partial
	lang := s.ctx.DetectedLanguage()
	conf := s.confidence()
	s.partial = ""
	s.segments = nil
	if s.lastSegEnd > 0 && s.lastSegEnd < len(s.window) {
		s.window = s.window[s.lastSegEnd:]
	}
	s.lastSegEnd = 0
	s.resetGate()
	if s.onDone != nil {
		s.onDone(text, conf, lang)
	}
}

// confidence is the mean token probability over the last segment.
func (s *streamState) confidence() float64 {
	var sum float64
	var n int
	for _, seg := range s.segments {
		for _, tok := range seg.Tokens {
			sum += float64(tok.P)
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}
