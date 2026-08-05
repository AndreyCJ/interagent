package whisper

import (
	"math"
	"time"

	whispercpp "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

// finalizeSilence is the trailing silence (model time) required before the last
// segment counts as a finished phrase. Tunable; validated by the CI integration
// test.
const finalizeSilence = 400 * time.Millisecond

// streamState is the whisper streaming state machine: a rolling window at the
// model sample rate and single-segment re-transcription with phrase
// finalization on trailing silence.
//
// The Go binding forces single_segment mode whenever a SegmentCallback is set,
// so each Process call re-transcribes the WHOLE window into one segment (fresh
// results every call — whisper_full_with_state clears result_all up front).
// Partial text is therefore REPLACED, never appended. Phrase boundaries are
// detected by trailing silence: when the last segment's end leaves >=
// finalizeSilence of audio at the window tail, the speaker has stopped and we
// finalize the phrase. (Verified against whisper.cpp v1.9.1 whisper_full /
// whisper_full_with_state.)
type streamState struct {
	ctx        sttContext
	sampleRate int
	onPartial  func(string)
	onDone     func(string, float64, string)

	window   []float32
	partial  string
	segments []whispercpp.Segment
	// lastSegEnd is the end sample (model rate) of the most recent segment and
	// windowLen the window length at the time that segment was produced.
	lastSegEnd int
	windowLen  int
	first      bool // false after the first Process call
}

func newStreamState(ctx sttContext, sampleRate int, onPartial func(string), onDone func(string, float64, string)) *streamState {
	return &streamState{ctx: ctx, sampleRate: sampleRate, onPartial: onPartial, onDone: onDone, first: true}
}

func (s *streamState) ingest(chunk []byte) error {
	samples := bytesToFloat32(chunk)
	rs := resample(samples, s.sampleRate, modelSampleRate)
	s.window = append(s.window, rs...)
	if max := modelSampleRate * windowSeconds; len(s.window) > max {
		s.window = s.window[len(s.window)-max:]
	}
	return s.ctx.Process(s.window, s.newEncoderBeginCb(), s.newSegmentCb(), nil)
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
