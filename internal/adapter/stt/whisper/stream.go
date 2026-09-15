package whisper

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	whispercpp "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

// finalizeSilence is the trailing silence (model time) required before the last
// segment counts as a finished phrase.
const finalizeSilence = 400 * time.Millisecond

// processTrigger says why Process is being run. Process runs the whole window
// through whisper and the result is (a) committed under agreement or cap, or
// (b) finalized as the tail of a finished phrase.
type processTrigger int

const (
	trigNone     processTrigger = iota
	trigFinalize                // trailing quiet >= finalize: phrase end — emit onDone (tail)
	trigCommit                  // backstop/catch-all while speech continues: commit by agreement
	trigCap                     // window hit maxWindow: force-commit to bound latency
)

// streamParams are the silence-gate timings, RMS threshold and window cap.
// Single default set today; kept as a struct so calibration can adjust without
// touching the logic.
type streamParams struct {
	gateTail      time.Duration // trailing RMS window at model rate
	finalize      time.Duration // quiet run length that triggers a phrase-end Process
	backstop      time.Duration // Process cadence while speech is continuous
	maxInterval   time.Duration // hard cap between Processes when the window has energy
	maxWindow     time.Duration // window cap; force-commit fallback beyond it
	gateThreshold float32       // RMS below which the tail counts as quiet
}

var streamDefaults = streamParams{
	gateTail:      200 * time.Millisecond,
	finalize:      finalizeSilence,
	backstop:      3 * time.Second,
	maxInterval:   4 * time.Second,
	maxWindow:     10 * time.Second,
	gateThreshold: 0.005,
}

// sttDebugRMS logs the gate state to /tmp/interagent-stt-rms.log (1 line/sec)
// when INTERAGENT_STT_DEBUG_RMS=1 — used for calibrating gateThreshold against
// real system audio.
var sttDebugRMS = os.Getenv("INTERAGENT_STT_DEBUG_RMS") != ""

// word is a transcribed word with its span (model-rate samples) relative to
// the current window start. The window always starts at the commit point, so a
// word's span double as its absolute position in the uncommitted region and
// timestamps stay comparable between runs.
type word struct {
	text       string
	start, end int
}

// streamState is the whisper streaming state machine: a rolling window that is
// trimmed ONLY at emission points (never silently dropped from the front), and
// a two-pass agreement flush that commits words stable across consecutive
// transcriptions while speech continues.
//
// The previous implementation passed a SegmentCallback, which forces
// single_segment mode and re-transcribes the WHOLE window into one segment
// every call (fresh results each time). Process is therefore run ONLY at gate
// triggers — never per chunk: phrase end on trailing silence, cadence while
// speech is continuous, or a hard latency cap.
type streamState struct {
	ctx         sttContext
	sampleRate  int
	onPartial   func(string)
	onCommitted func(string)
	onDone      func(string, float64, string)
	params      streamParams

	window    []float32 // audio from the commit point forward; never dropped before commit
	partial   string    // live draft of the current window (optional UI)
	prevWords []word    // words of the last run, re-based to the current window

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

func newStreamState(ctx sttContext, sampleRate int, onPartial func(string), onCommitted func(string), onDone func(string, float64, string)) *streamState {
	ringLen := int(math.Ceil(streamDefaults.gateTail.Seconds() * float64(modelSampleRate)))
	if ringLen < 1 {
		ringLen = 1
	}
	return &streamState{
		ctx: ctx, sampleRate: sampleRate, onPartial: onPartial, onCommitted: onCommitted, onDone: onDone,
		params: streamDefaults, ring: make([]float32, ringLen),
		lastProcess: time.Now(),
	}
}

func (s *streamState) ingest(chunk []byte) error {
	samples := bytesToFloat32(chunk)
	rs := resample(samples, s.sampleRate, modelSampleRate)
	// Append without trimming: the window front is never dropped before the
	// audio it contains has been committed or finalized (maxWindow handles the
	// pathological case separately).
	s.window = append(s.window, rs...)
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

func (s *streamState) capFrames() int {
	return int(math.Ceil(s.params.maxWindow.Seconds() * float64(modelSampleRate)))
}

// maybeProcess runs Process only when the silence gate, a cadence timer, or
// the window cap says so. Per-chunk re-transcription (the 10s latency / 99%
// CPU problem) is gone.
func (s *streamState) maybeProcess() error {
	now := time.Now()
	tail := s.tailRMS()
	if sttDebugRMS && now.Sub(s.lastDebug) >= time.Second {
		s.lastDebug = now
		f, err := os.OpenFile("/tmp/interagent-stt-rms.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%s tail_rms=%.5f speaking=%v since_process=%v energy=%v window=%ds\n",
				now.Format("15:04:05.000"), tail, s.speaking, now.Sub(s.lastProcess).Round(time.Millisecond), s.energySinceProcess, len(s.window)/modelSampleRate)
			_ = f.Close()
		}
	}
	var trig processTrigger
	switch {
	case tail >= s.params.gateThreshold:
		s.speaking = true
		s.quietSince = time.Time{}
	case s.speaking:
		if s.quietSince.IsZero() {
			s.quietSince = now
		}
		if now.Sub(s.quietSince) >= s.params.finalize {
			trig = trigFinalize
		}
	}
	if trig == trigNone && s.speaking && now.Sub(s.lastProcess) >= s.params.backstop {
		trig = trigCommit
	}
	if trig == trigNone && s.energySinceProcess > 0 && now.Sub(s.lastProcess) >= s.params.maxInterval {
		trig = trigCommit
	}
	if trig == trigNone && len(s.window) > s.capFrames() {
		trig = trigCap
	}
	if trig == trigNone {
		return nil
	}
	return s.process(now, trig)
}

// process runs whisper over the window and either commits stable words
// (onCommitted) or finalizes the phrase tail (onDone).
func (s *streamState) process(now time.Time, trig processTrigger) error {
	s.lastProcess = now
	s.energySinceProcess = 0
	// No segment callback → the binding does NOT force single_segment; the
	// full segment set is read back with NextSegment after the run.
	if err := s.ctx.Process(s.window, nil, nil, nil); err != nil {
		return err
	}
	segs, err := drainSegments(s.ctx)
	if err != nil {
		return err
	}
	words := wordsFromSegments(segs, s.ctx.IsText)
	if s.onPartial != nil {
		s.partial = joinWords(words)
		s.onPartial(s.partial)
	}
	switch trig {
	case trigFinalize:
		return s.finalize(words, segs)
	default:
		return s.commitRun(words, trig)
	}
}

// commitRun compares the current transcription with the previous run and
// commits the stable prefix (all but one lookahead word) via onCommitted, then
// trims the window to the commit point and re-bases prevWords. Under the cap
// trigger the whole current transcription is force-committed so latency stays
// bounded even if whisper keeps disagreeing with itself.
func (s *streamState) commitRun(cur []word, trig processTrigger) error {
	stable := lcp(s.prevWords, cur)
	commitN := 0
	switch trig {
	case trigCap:
		commitN = len(cur)
	case trigCommit:
		if stable > 1 {
			commitN = stable - 1 // keep one word of lookahead for finalize
		}
	}
	if commitN == 0 {
		if trig == trigCap {
			// Nothing transcribed in a window that overflowed: the front of the
			// window is silence/noise, so dropping it loses no committed speech.
			s.trimWindowToCap()
		}
		s.prevWords = cur
		return nil
	}
	commit := cur[:commitN]
	endSample := commit[len(commit)-1].end
	if endSample > len(s.window) {
		// whisper's VAD path time-maps timestamps back to original coordinates
		// and can report the last token's end past the fed audio (orig_end snaps
		// to the VAD chunk grid, e.g. 160960 samples vs a 160800-sample window).
		// Clamp to the window so trimming can't panic; the word itself was still
		// transcribed in full, so committing it is correct.
		endSample = len(s.window)
	}
	text := joinWords(commit)
	s.window = s.window[endSample:]
	s.prevWords = projectWords(cur, endSample)
	if s.onCommitted != nil {
		s.onCommitted(text)
	}
	return nil
}

func (s *streamState) trimWindowToCap() {
	max := s.capFrames()
	if len(s.window) > max {
		s.window = s.window[len(s.window)-max:]
	}
}

// finalize emits the uncommitted tail of a finished phrase. Committed words
// were already trimmed out of the window, so onDone never repeats them (ADR-007
// "final" history is the union of committed + done, each span exactly once).
func (s *streamState) finalize(words []word, segs []whispercpp.Segment) error {
	s.prevWords = nil
	s.resetGate()
	if len(words) == 0 {
		return nil
	}
	// Trim trailing silence past the last spoken word so the next phrase
	// starts clean at the speech boundary.
	if end := words[len(words)-1].end; end > 0 && end < len(s.window) {
		s.window = s.window[end:]
	}
	if s.onDone != nil {
		s.onDone(joinWords(words), confidenceOf(segs), s.ctx.DetectedLanguage())
	}
	return nil
}

// wordsFromSegments token-walks segments into word spans. Whisper marks the
// start of a word with a leading space on the token text; the first text token
// after a special token also starts a word. Control/timestamp tokens are
// skipped: the binding's IsText decides textness by token id ([_BEG_],
// [_TT_NNN]), the text-shape check catches legacy <|...|> / |>-style markers,
// and the bracketed check catches [BLANK_AUDIO] / [POS_EMBED_*] from the
// n-token vocab range.
func wordsFromSegments(segs []whispercpp.Segment, isText func(whispercpp.Token) bool) []word {
	var out []word
	for _, seg := range segs {
		for _, tok := range seg.Tokens {
			text := strings.TrimSpace(tok.Text)
			if text == "" || !isText(tok) || isBracketToken(tok.Text) || strings.HasPrefix(tok.Text, "<|") || strings.HasSuffix(tok.Text, "|>") {
				continue
			}
			if len(out) == 0 || strings.HasPrefix(tok.Text, " ") {
				out = append(out, word{
					text:  text,
					start: sampleOf(tok.Start),
					end:   sampleOf(tok.End),
				})
			} else {
				w := &out[len(out)-1]
				w.text += text
				w.end = sampleOf(tok.End)
			}
		}
	}
	return out
}

// isBracketToken reports whether a token text is a bracketed special marker
// like [BLANK_AUDIO] or [_BEG_]. Whisper's vocabulary has no legitimate
// space-free all-bracketed word tokens, so these are always control markers.
func isBracketToken(text string) bool {
	if len(text) < 3 || text[0] != '[' || text[len(text)-1] != ']' {
		return false
	}
	return !strings.Contains(text, " ")
}

// lcp returns how many leading words of prev and cur agree exactly.
func lcp(prev, cur []word) int {
	k := 0
	for k < len(prev) && k < len(cur) && prev[k].text == cur[k].text {
		k++
	}
	return k
}

// projectWords re-bases words to a new window start offset, dropping the words
// that were committed away.
func projectWords(cur []word, offset int) []word {
	out := make([]word, 0, len(cur))
	for _, w := range cur {
		if w.end <= offset {
			continue
		}
		w.start -= offset
		w.end -= offset
		if w.start < 0 {
			w.start = 0
		}
		out = append(out, w)
	}
	return out
}

func sampleOf(d time.Duration) int {
	n := int(math.Round(float64(d) * float64(modelSampleRate) / float64(time.Second)))
	if n < 0 {
		return 0
	}
	return n
}

func joinWords(words []word) string {
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	for i, w := range words {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(w.text)
	}
	return b.String()
}

func confidenceOf(segs []whispercpp.Segment) float64 {
	var sum float64
	var n int
	for _, seg := range segs {
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

// drainSegments reads all segments produced by the last Process call (the
// binding resets the cursor on each Process).
func drainSegments(ctx sttContext) ([]whispercpp.Segment, error) {
	var segs []whispercpp.Segment
	for {
		seg, err := ctx.NextSegment()
		if err == io.EOF {
			return segs, nil
		}
		if err != nil {
			return nil, err
		}
		segs = append(segs, seg)
	}
}
