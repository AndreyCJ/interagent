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

// streamParams are the silence-gate timings, RMS thresholds and window cap.
// Single default set today; kept as a struct so calibration can adjust without
// touching the logic.
type streamParams struct {
	gateTail    time.Duration // trailing RMS window at model rate
	finalize    time.Duration // quiet run length that triggers a phrase-end Process
	backstop    time.Duration // Process cadence while speech is continuous
	maxInterval time.Duration // hard cap between Processes when the window has energy
	maxWindow   time.Duration // window cap; force-commit fallback beyond it

	absFloor    float32       // absolute noise floor — gates never drop below it
	enterFactor float32       // noise-floor multiple to ENTER speech (speech gate)
	pauseFactor float32       // noise-floor multiple to ENTER pause (pause gate)
	floorRise   float32       // noise-floor recovery fraction/sec while idle
	coldRecover time.Duration // flat-tail run that ends a cold (unlearned-floor) phrase
	varFloor    float32       // tail-RMS spread below which a flat "phrase" is ambient

	minWord time.Duration // floor of window advance per committed word (degenerate-timestamp guard)
}

var streamDefaults = streamParams{
	gateTail:    200 * time.Millisecond,
	finalize:    finalizeSilence,
	backstop:    3 * time.Second,
	maxInterval: 4 * time.Second,
	maxWindow:   10 * time.Second,

	absFloor:    0.005,
	enterFactor: 1.6,
	pauseFactor: 1.3,
	floorRise:   2.5,
	coldRecover: 400 * time.Millisecond,
	varFloor:    0.004,
	minWord:     300 * time.Millisecond,
}

// silenceGate is a pure, adaptive silence gate.
//
// The noise floor is the anchor: it drops instantly, rises fast while IDLE
// (ambient learning), and is FROZEN while speaking — so sustained speech is
// never consumed as ambient. Two thresholds derive from it:
//
//	speechGate = max(floor*enterFactor, absFloor)  — enter speaking above it
//	pauseGate   = max(floor*pauseFactor, absFloor) — finalize after a quiet run
//	                below it; between the two is a hysteresis band that freezes
//	                the quiet timer (mid-levels don't advance a phrase end)
//
// evaluate is deterministic and driven by (now, tail); it never calls the wall
// clock. dt comes from the previous `last` and is clamped to 1s so wall-clock
// outliers after long whisper runs can't skew it.
type silenceGate struct {
	noiseFloor float32
	prevTail   float32 // previous tail sample — the floor only reacts to it (idle)
	speaking   bool
	quietSince time.Time
	flatSince  time.Time
	last       time.Time

	tails    []float32 // recent tail-RMS values (flat-detection ring)
	tailsPos int
	tailsN   int
	params   streamParams
}

func newSilenceGate(params streamParams) *silenceGate {
	return &silenceGate{params: params, tails: make([]float32, 8)}
}

// speechGate is the adaptive enter-speech threshold: max(noise×enterFactor,
// absFloor). The tail must exceed it to transition from idle to speaking.
func (g *silenceGate) speechGate() float32 {
	sg := g.noiseFloor * g.params.enterFactor
	if g.params.absFloor > sg {
		sg = g.params.absFloor
	}
	return sg
}

// pauseGate is the adaptive enter-pause threshold: max(noise×pauseFactor,
// absFloor). The tail must drop below it to start the finalize timer.
func (g *silenceGate) pauseGate() float32 {
	pg := g.noiseFloor * g.params.pauseFactor
	if g.params.absFloor > pg {
		pg = g.params.absFloor
	}
	return pg
}

// tailRange is the spread of the recent tail-RMS ring. A near-zero spread over
// a full ring means the signal is a constant plateau (ambient, or a test tone)
// rather than speech, whose RMS always pulses against the floor.
func (g *silenceGate) tailRange() float32 {
	if g.tailsN == 0 {
		return 0
	}
	mn, mx := g.tails[0], g.tails[0]
	for _, v := range g.tails[:g.tailsN] {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	return mx - mn
}

func (g *silenceGate) evaluate(now time.Time, tail float32) processTrigger {
	if g.last.IsZero() {
		g.last = now
	}
	dt := now.Sub(g.last)
	g.last = now
	if dt > time.Second {
		dt = time.Second
	}
	dtf := float32(dt.Seconds())

	// The floor adapts ONLY while idle and reacts to the PREVIOUS sample, so a
	// loud voice onset can never raise its own gate before the enter decision.
	// While speaking it is fully frozen (no down-track either) — mid-phrase
	// dips must not collapse the pause threshold.
	if !g.speaking {
		if tail < g.noiseFloor {
			g.noiseFloor = tail
		} else if g.prevTail != g.noiseFloor {
			g.noiseFloor += (g.prevTail - g.noiseFloor) * dtf * g.params.floorRise
		}
		g.noiseFloor = max(g.noiseFloor, g.params.absFloor)
	}
	g.prevTail = tail

	trig := trigNone
	if !g.speaking {
		if tail > g.speechGate() {
			g.speaking = true
			g.flatSince = time.Time{}
			// Phrase onset wipes the pre-speech history, so the flat-recovery
			// window measures only IN-phrase samples — stale ambient (or idle
			// silence) in the ring must not fake variation at the margins.
			g.tailsPos = 0
			g.tailsN = 0
		}
	} else {
		// Cold recovery: if the floor was never learned (e.g. ambient noise is
		// the first audio the gate sees) the "phrase" is really a flat plateau
		// — a level-only gate cannot tell it from speech until it walks the
		// floor. A zero-spread tail for coldRecover ends the phantom phrasing
		// WITHOUT finalizing, so the floor catches up and the ambient is then
		// correctly classified as idle. Real speech always pulses its RMS, so a
		// genuine phrase is never flat for that long.
		if g.tailRange() < g.params.varFloor {
			if g.flatSince.IsZero() {
				g.flatSince = now
			}
			if now.Sub(g.flatSince) >= g.params.coldRecover {
				g.speaking = false
				g.quietSince = time.Time{}
				g.flatSince = time.Time{}
				return trigNone
			}
		} else {
			g.flatSince = time.Time{}
		}

		switch {
		case tail >= g.speechGate():
			g.quietSince = time.Time{} // definitely speech — reset
		case tail < g.pauseGate():
			if g.quietSince.IsZero() {
				g.quietSince = now
			}
			if now.Sub(g.quietSince) >= g.params.finalize {
				trig = trigFinalize
			}
		}
		// else: [pauseGate, speechGate) — hysteresis band, freeze quietSince
	}

	// Recent-tail ring, pushed AFTER the decision so the enter reset keeps the
	// phrase's first sample in the window (it drives the flat-recovery range).
	g.tails[g.tailsPos] = tail
	g.tailsPos = (g.tailsPos + 1) % len(g.tails)
	if g.tailsN < len(g.tails) {
		g.tailsN++
	}

	return trig
}

func (g *silenceGate) reset() {
	g.speaking = false
	g.quietSince = time.Time{}
	g.flatSince = time.Time{}
}

// sttDebugRMS logs the gate state to /tmp/interagent-stt-rms.log (1 line/sec)
// when INTERAGENT_STT_DEBUG_RMS=1 — used for calibrating the adaptive gate
// against real system audio.
var sttDebugRMS = os.Getenv("INTERAGENT_STT_DEBUG_RMS") != ""

// sttDebugCommits logs each commit/finalize decision to
// /tmp/interagent-stt-commits.log when INTERAGENT_STT_DEBUG_COMMITS=1 — used to
// confirm exactly-once emission on real audio (the duplicate-sentence bug).
var sttDebugCommits = os.Getenv("INTERAGENT_STT_DEBUG_COMMITS") != ""

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
	onStatus    func(bool)
	params      streamParams

	window    []float32 // audio from the commit point forward; never dropped before commit
	partial   string    // live draft of the current window (optional UI)
	prevWords []word    // words of the last run, re-based to the current window

	// Exactly-once emission guard (degenerate-timestamp path): if whisper
	// under-reports token ends the commit trim can degenerate, leaving already
	// committed audio at the window front; a later agreeing run — or the
	// finalize — would then re-emit it verbatim. `laggy` latches onto any such
	// phrase (VAD time mapping is stable within a session) and `emitted` holds
	// every word already handed out, so re-transcribed prefixes are stripped at
	// the next emission points instead of duplicating in the transcript.
	emitted []word
	laggy   bool

	// Silence gate.
	ring               []float32 // trailing model-rate samples (gateTail)
	ringPos            int
	ringCount          int
	gate               silenceGate
	lastProcess        time.Time
	energySinceProcess float64 // sum of |sample| appended since the last Process
	lastDebug          time.Time
	lastStatus         bool // mirrors gate.speaking for onStatus transition detection
}

func newStreamState(ctx sttContext, sampleRate int, onPartial func(string), onCommitted func(string), onDone func(string, float64, string), onStatus func(bool)) *streamState {
	ringLen := int(math.Ceil(streamDefaults.gateTail.Seconds() * float64(modelSampleRate)))
	if ringLen < 1 {
		ringLen = 1
	}
	return &streamState{
		ctx: ctx, sampleRate: sampleRate, onPartial: onPartial, onCommitted: onCommitted, onDone: onDone, onStatus: onStatus,
		params: streamDefaults, ring: make([]float32, ringLen),
		gate: *newSilenceGate(streamDefaults), lastProcess: time.Now(),
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
	s.gate.reset()
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
	defer s.syncStatus()
	now := time.Now()
	tail := s.tailRMS()
	trig := s.gate.evaluate(now, tail)
	if trig == trigNone && s.gate.speaking && now.Sub(s.lastProcess) >= s.params.backstop {
		trig = trigCommit
	}
	if trig == trigNone && s.energySinceProcess > 0 && now.Sub(s.lastProcess) >= s.params.maxInterval {
		trig = trigCommit
	}
	if trig == trigNone && len(s.window) > s.capFrames() {
		trig = trigCap
	}
	if sttDebugRMS && now.Sub(s.lastDebug) >= time.Second {
		s.lastDebug = now
		f, err := os.OpenFile("/tmp/interagent-stt-rms.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			quiet := ""
			if !s.gate.quietSince.IsZero() {
				quiet = now.Sub(s.gate.quietSince).Round(time.Millisecond).String()
			}
			trigLabel := "<none>"
			if trig != trigNone {
				trigLabel = []string{"finalize", "commit", "cap"}[int(trig)-1]
			}
			_, _ = fmt.Fprintf(f, "%s tail_rms=%.5f floor=%.5f speech_gate=%.5f pause_gate=%.5f speaking=%v quiet=%s since_process=%v energy=%.6f window_s=%d trig=%s\n",
				now.Format("15:04:05.000"), tail, s.gate.noiseFloor, s.gate.speechGate(), s.gate.pauseGate(), s.gate.speaking, quiet, now.Sub(s.lastProcess).Round(time.Millisecond), s.energySinceProcess, len(s.window)/modelSampleRate, trigLabel)
			_ = f.Close()
		}
	}
	if trig == trigNone {
		return nil
	}
	return s.process(now, trig)
}

// syncStatus reports gate.speaking transitions to onStatus, so the UI can
// show a stable "transcribing" indicator while speech is detected and hide it
// as soon as the phrase is finalized or the gate returns to idle.
func (s *streamState) syncStatus() {
	if s.gate.speaking == s.lastStatus {
		return
	}
	s.lastStatus = s.gate.speaking
	if s.onStatus != nil {
		s.onStatus(s.gate.speaking)
	}
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
	if sttDebugCommits {
		debugf("process", "trig=%s window_s=%d text=%q", trigName(trig), len(s.window)/modelSampleRate, joinWords(words))
	}
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
	// Degenerate-timestamp guard, the mirror of the overshoot above: the VAD
	// time mapping can also UNDERREPORT the committed block's last end down to
	// ~the window origin. Then the trim is a no-op, the already-committed audio
	// stays at the window front, and the phrase accumulates duplicated sentences
	// (the reported bug): a later agreeing run re-commits the same prefix, and
	// the finalize re-emits the whole window. Floor the trim point at minWord
	// per committed word so the window ALWAYS advances past committed audio, and
	// latch the phrase as laggy so every later run strips any re-transcribed
	// prefix from being emitted twice.
	advance := commit[0].start + commitN*minWordSamples(s.params)
	if endSample < advance {
		s.laggy = true
		if advance < len(s.window) {
			endSample = advance
		} else {
			endSample = len(s.window)
		}
	}
	if sttDebugCommits {
		debugf("commit", "trig=%s prev_words=%d stable=%d commit_n=%d end_sample=%d advance=%d laggy=%v window_s=%d emitted=%d text=%q",
			trigName(trig), len(s.prevWords), stable, commitN, endSample, advance, s.laggy, len(s.window)/modelSampleRate, len(s.emitted), joinWords(commit))
	}
	commit = stripReplay(s.emitted, commit)
	if len(commit) > 0 {
		s.emitted = append(s.emitted, commit...)
		if s.onCommitted != nil {
			s.onCommitted(joinWords(commit))
		}
	}
	s.window = s.window[endSample:]
	s.prevWords = projectWords(cur, endSample)
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
		s.emitted = nil
		s.laggy = false
		return nil
	}
	// Degenerate-timestamp phrase (laggy): the finalize re-transcribed audio
	// that was already committed. Strip the replayed prefix so onDone carries
	// only genuinely new tail words.
	if s.laggy {
		words = stripReplay(s.emitted, words)
	}
	if s.laggy {
		// The whole phrase has now been emitted (committed ∪ done). Timestamps
		// say nothing trustworthy about the true speech boundary, so drop the
		// leftover window outright — otherwise the next phrase re-transcribes
		// and re-emits it again.
		s.window = nil
	} else if len(words) > 0 {
		// Trim trailing silence past the last spoken word so the next phrase
		// starts clean at the speech boundary.
		if end := words[len(words)-1].end; end > 0 && end < len(s.window) {
			s.window = s.window[end:]
		}
	}
	if sttDebugCommits {
		debugf("finalize", "laggy=%v window_s=%d emitted=%d words=%d text=%q",
			s.laggy, len(s.window)/modelSampleRate, len(s.emitted), len(words), joinWords(words))
	}
	if len(words) > 0 && s.onDone != nil {
		s.onDone(joinWords(words), confidenceOf(segs), s.ctx.DetectedLanguage())
	}
	s.emitted = nil
	s.laggy = false
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

// minWordSamples is streamParams.minWord expressed in model-rate samples,
// clamped to at least one sample.
func minWordSamples(params streamParams) int {
	n := int(math.Round(params.minWord.Seconds() * float64(modelSampleRate)))
	if n < 1 {
		n = 1
	}
	return n
}

// replayedPrefix returns the length of the longest prefix of cur that appears
// as a contiguous run of already-emitted words, matched case-insensitively —
// whisper re-capitalizes boundary words at segment starts ("И" after committed
// "и"). Only consulted under the laggy flag: a healthy-timestamp phrase
// re-transcribing itself from the front is a legitimately repeated utterance
// and must NOT be stripped.
func replayedPrefix(emitted, cur []word) int {
	best := 0
	for i := 0; i < len(emitted); i++ {
		k := 0
		for k < len(cur) && i+k < len(emitted) && equalFoldWord(emitted[i+k], cur[k]) {
			k++
		}
		if k > best {
			best = k
		}
		if best == len(cur) {
			break
		}
	}
	return best
}

// stripReplay removes the replayed prefix of a degraded re-transcription so
// already-emitted words are never handed out twice.
func stripReplay(emitted, cur []word) []word {
	n := replayedPrefix(emitted, cur)
	if n == 0 {
		return cur
	}
	return append([]word(nil), cur[n:]...)
}

// equalFoldWord compares two word spans ignoring letter case.
func equalFoldWord(a, b word) bool { return strings.EqualFold(a.text, b.text) }

// debugf appends a timestamped line to /tmp/interagent-stt-commits.log when
// INTERAGENT_STT_DEBUG_COMMITS=1, so exactly-once emission can be confirmed
// against real audio (the reported duplicate-sentence bug).
func debugf(label, format string, args ...any) {
	if !sttDebugCommits {
		return
	}
	f, err := os.OpenFile("/tmp/interagent-stt-commits.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(f, "%s %-8s "+format+"\n", append([]any{time.Now().Format("15:04:05.000"), label}, args...)...)
	_ = f.Close()
}

// trigName labels a processTrigger for the debug log.
func trigName(t processTrigger) string {
	switch t {
	case trigFinalize:
		return "finalize"
	case trigCommit:
		return "commit"
	case trigCap:
		return "cap"
	}
	return "none"
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
