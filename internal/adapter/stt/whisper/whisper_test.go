package whisper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	whispercpp "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

//
// Fake context seam.
//
// The streaming state machine reads full segment results after each Process
// via NextSegment (no segment callback — the binding's single_segment forcing
// is gone). The fake produces segments the same way the stream consumes them.
//

type fakeContext struct {
	detected   string
	setLang    []string
	threads    uint
	vad        bool
	vadModel   string
	vadThresh  float32
	tokenTS    bool
	processes  int
	windows    [][]float32
	results    []whispercpp.Segment
	processFn  func(window []float32, seq int) []whispercpp.Segment
	setTokenTS []bool
}

func (f *fakeContext) SetLanguage(l string) error { f.setLang = append(f.setLang, l); return nil }
func (f *fakeContext) SetThreads(v uint)          { f.threads = v }
func (f *fakeContext) SetVAD(v bool)              { f.vad = v }
func (f *fakeContext) SetVADModelPath(p string)   { f.vadModel = p }
func (f *fakeContext) SetVADThreshold(t float32)  { f.vadThresh = t }
func (f *fakeContext) SetTokenTimestamps(b bool)  { f.tokenTS = b }

func (f *fakeContext) Process(window []float32, _ whispercpp.EncoderBeginCallback, _ whispercpp.SegmentCallback, _ whispercpp.ProgressCallback) error {
	f.windows = append(f.windows, window)
	seq := f.processes
	f.processes++
	if f.processFn != nil {
		f.results = f.processFn(window, seq)
	}
	return nil
}

func (f *fakeContext) NextSegment() (whispercpp.Segment, error) {
	if len(f.results) == 0 {
		return whispercpp.Segment{}, io.EOF
	}
	s := f.results[0]
	f.results = f.results[1:]
	return s, nil
}

func (f *fakeContext) DetectedLanguage() string { return f.detected }

// IsText mirrors the binding's token-textness check: control tokens ([_BEG_],
// [_TT_NNN], …) have ids at/above eot and are not text.
func (f *fakeContext) IsText(tok whispercpp.Token) bool { return tok.Id < 500 }

type fakeModel struct {
	ctx    *fakeContext
	closed bool
}

func (f *fakeModel) NewContext() (sttContext, error) { return f.ctx, nil }
func (f *fakeModel) Close() error                    { f.closed = true; return nil }

func newFakeWhisper(t *testing.T, model *fakeModel) *Whisper {
	t.Helper()
	w := newWithOpener("model.bin", "vad.onnx", "auto", func(string) (sttModel, error) { return model, nil })
	return w
}

func TestPreload_ReturnsOpenerError(t *testing.T) {
	w := newWithOpener("model.bin", "vad.onnx", "auto", func(string) (sttModel, error) {
		return nil, errors.New("cannot load model")
	})
	if err := w.Preload(); err == nil {
		t.Fatal("Preload() must surface the model load error synchronously")
	}
}

func TestPreload_LoadsModelOnce(t *testing.T) {
	model := &fakeModel{}
	calls := 0
	w := newWithOpener("model.bin", "vad.onnx", "auto", func(string) (sttModel, error) {
		calls++
		return model, nil
	})
	if err := w.Preload(); err != nil {
		t.Fatalf("Preload() error: %v", err)
	}
	if err := w.Preload(); err != nil {
		t.Fatalf("second Preload() error: %v", err)
	}
	if calls != 1 {
		t.Errorf("opener called %d times, want 1 (model cached after first load)", calls)
	}
}

//
// Audio helpers.
//
// Speech is encoded as per-word DC levels: each word occupies exactly one
// model second at amplitude 0.05 + 0.01*id. Any slice of a window decodes to
// the same words the stream operates on, so truncation / trimming never hides
// or renames an uncommitted word — which is exactly the property the streaming
// tests assert. Silence is amplitude 0 and decodes to no words.
//

const wordAmpStep = 0.01

func feedWord(t *testing.T, w *Whisper, sampleRate, wordID int) {
	t.Helper()
	samples := make([]float32, sampleRate)
	amp := float32(0.05) + float32(wordID)*wordAmpStep
	for i := range samples {
		samples[i] = amp
	}
	if err := w.Feed(float32ToBytes(samples)); err != nil {
		t.Fatal(err)
	}
}

func meanAmp(samples []float32) float32 {
	var sum float64
	for _, v := range samples {
		sum += float64(v)
	}
	if len(samples) == 0 {
		return 0
	}
	return float32(sum / float64(len(samples)))
}

func wordIDFromAmp(amp float32) int {
	if amp < 0.03 { // silence / below-speech level
		return 0
	}
	return int((amp-0.05)/wordAmpStep + 0.5)
}

// wordSegments returns one segment whose tokens carry the words decoded from
// the window's DC levels, with token timestamps spread one word per 1s.
func wordSegments(window []float32) []whispercpp.Segment {
	const wordSamples = modelSampleRate
	var toks []whispercpp.Token
	var texts []string
	var end time.Duration
	for off := 0; off < len(window); off += wordSamples {
		e := off + wordSamples
		if e > len(window) {
			if len(window)-off < wordSamples/2 {
				break // trailing sub-half frame is never speech-sized
			}
			e = len(window)
		}
		id := wordIDFromAmp(meanAmp(window[off:e]))
		if id == 0 {
			continue
		}
		texts = append(texts, "word"+strconv.Itoa(id))
		toks = append(toks, whispercpp.Token{
			Id:    id,
			Text:  " word" + strconv.Itoa(id), // leading space = word start in whisper tokens
			P:     0.95,
			Start: time.Duration(off) * time.Second / modelSampleRate,
			End:   time.Duration(e) * time.Second / modelSampleRate,
		})
		end = time.Duration(e) * time.Second / modelSampleRate
	}
	if len(toks) == 0 {
		return nil
	}
	return []whispercpp.Segment{{
		Num:    0,
		Text:   strings.Join(texts, " "),
		Start:  0,
		End:    end,
		Tokens: toks,
	}}
}

// decodeWords is the pure decoding used both by the fake transcription and by
// test assertions, so expectations always match what a process would emit.
func decodeWords(window []float32) []string {
	segs := wordSegments(window)
	if len(segs) == 0 {
		return nil
	}
	return strings.Fields(segs[0].Text)
}

//
// Shared streaming helpers.
//

func startStream(t *testing.T, w *Whisper, onPartial func(string), onCommitted func(string), onDone func(string, float64, string)) chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- w.Stream(48000, onPartial, onCommitted, onDone, nil) }()
	time.Sleep(20 * time.Millisecond) // let Stream reach the ingest loop
	return done
}

type emissionCollector struct {
	commits []string
	dones   []string
}

func (c *emissionCollector) onCommitted() func(string) {
	return func(text string) { c.commits = append(c.commits, text) }
}

func (c *emissionCollector) onDone() func(string, float64, string) {
	return func(text string, _ float64, _ string) { c.dones = append(c.dones, text) }
}

func TestStream_ConfiguresContext(t *testing.T) {
	ctx := &fakeContext{}
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	done := startStream(t, w, nil, nil, nil)
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if len(ctx.setLang) != 1 || ctx.setLang[0] != "auto" {
		t.Errorf("SetLanguage = %v, want [auto]", ctx.setLang)
	}
	if ctx.threads != 4 {
		t.Errorf("SetThreads = %d, want 4", ctx.threads)
	}
	if !ctx.vad || ctx.vadModel != "vad.onnx" || ctx.vadThresh != 0.6 {
		t.Errorf("VAD config = (%v, %q, %v)", ctx.vad, ctx.vadModel, ctx.vadThresh)
	}
	if !ctx.tokenTS {
		t.Error("SetTokenTimestamps not enabled (word-level commit points need token timestamps)")
	}
}

func TestStream_ConfiguredLanguage(t *testing.T) {
	ctx := &fakeContext{}
	w := newWithOpener("model.bin", "vad.onnx", "ru", func(string) (sttModel, error) {
		return &fakeModel{ctx: ctx}, nil
	})
	done := startStream(t, w, nil, nil, nil)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if len(ctx.setLang) != 1 || ctx.setLang[0] != "ru" {
		t.Errorf("SetLanguage = %v, want [ru]", ctx.setLang)
	}
}

func TestStream_LoadError(t *testing.T) {
	w := newWithOpener("model.bin", "vad.onnx", "auto", func(string) (sttModel, error) {
		return nil, errors.New("cannot open model")
	})
	if err := w.Stream(48000, nil, nil, nil, nil); err == nil {
		t.Fatal("Stream() should return model load error")
	}
}

func quietChunk(samples int) []byte {
	return float32ToBytes(make([]float32, samples))
}

// TestStream_SilenceAfterSpeech_FinalizesOnce: a single phrase followed by
// trailing silence must produce exactly one Process (the finalize) and one
// onDone carrying that phrase.
func TestStream_SilenceAfterSpeech_FinalizesOnce(t *testing.T) {
	ctx := &fakeContext{detected: "en"}
	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment { return wordSegments(window) }
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	col := &emissionCollector{}
	done := startStream(t, w, nil, col.onCommitted(), col.onDone())

	if err := w.Feed(loudChunk(48000)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		time.Sleep(200 * time.Millisecond)
		if err := w.Feed(quietChunk(48000)); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	if ctx.processes != 1 {
		t.Errorf("Process called %d times, want exactly 1 (silence-gated)", ctx.processes)
	}
	if len(col.dones) != 1 {
		t.Fatalf("onDone = %v, want one phrase", col.dones)
	}
	want := strings.Join(decodeWords(ctx.windows[0]), " ")
	if col.dones[0] != want {
		t.Errorf("onDone = %q, want %q", col.dones[0], want)
	}
	if len(col.commits) != 0 {
		t.Errorf("onCommitted fired for a short phrase: %v", col.commits)
	}
}

func loudChunk(samples int) []byte {
	in := make([]float32, samples)
	for i := range in {
		in[i] = 0.1
	}
	return float32ToBytes(in)
}

// TestStream_ContinuousSpeech_CommitsBeforeSilence: during continuous speech
// the committed words are pushed before any phrase boundary, and the front of
// the turn (word1..) is never silently dropped. The commits are a strict
// ascending prefix of the spoken words — robust to the exact number of
// backstop processes the wall clock produces.
func TestStream_ContinuousSpeech_CommitsBeforeSilence(t *testing.T) {
	ctx := &fakeContext{}
	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment { return wordSegments(window) }
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	col := &emissionCollector{}
	done := startStream(t, w, nil, col.onCommitted(), col.onDone())

	const words = 8
	for i := 1; i <= words; i++ {
		feedWord(t, w, 48000, i)
		time.Sleep(1100 * time.Millisecond)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	if len(col.dones) != 0 {
		t.Errorf("onDone fired before any silence: %v", col.dones)
	}
	if len(col.commits) == 0 {
		t.Fatal("continuous speech produced no onCommitted output")
	}
	seq := strings.Fields(strings.Join(col.commits, " "))
	if seq[0] != "word1" {
		t.Errorf("front word lost: first committed = %q, want word1", seq[0])
	}
	for i, wd := range seq {
		if wd != fmt.Sprintf("word%d", i+1) {
			t.Errorf("committed words not a strict prefix: got %q at position %d, want word%d", wd, i, i+1)
		}
	}
}

// TestStream_PhraseFinalize_NoDoubleEmission: committed mid-speech words must
// never be re-emitted by onDone; onDone carries only the uncommitted tail. The
// committed + done output reassembles the whole spoken turn exactly once.
func TestStream_PhraseFinalize_NoDoubleEmission(t *testing.T) {
	ctx := &fakeContext{detected: "en"}
	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment { return wordSegments(window) }
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	col := &emissionCollector{}
	done := startStream(t, w, nil, col.onCommitted(), col.onDone())

	const words = 6
	for i := 1; i <= words; i++ {
		feedWord(t, w, 48000, i)
		time.Sleep(1100 * time.Millisecond)
	}
	for i := 0; i < 6; i++ {
		time.Sleep(200 * time.Millisecond)
		if err := w.Feed(quietChunk(48000)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done

	if len(col.dones) != 1 {
		t.Fatalf("onDone = %v, want exactly one finalize", col.dones)
	}
	reassembled := strings.Fields(strings.Join(append(append([]string{}, col.commits...), col.dones...), " "))
	if len(reassembled) != words {
		t.Errorf("reassembled turn = %v, want %d words", reassembled, words)
	}
	for i, wd := range reassembled {
		if wd != fmt.Sprintf("word%d", i+1) {
			t.Fatalf("turn reassembly broken at %d: %q (commits=%v dones=%v)", i, wd, col.commits, col.dones)
		}
	}
}

// TestStream_ContinuousSpeech_NoFrontWordLoss: with a small maxWindow cap the
// force-commit fallback still streams the front words out exactly once and in
// order — nothing is dropped from the window uncommitted.
func TestStream_ContinuousSpeech_NoFrontWordLoss(t *testing.T) {
	old := streamDefaults
	defer func() { streamDefaults = old }()
	streamDefaults.maxWindow = 2 * time.Second

	ctx := &fakeContext{}
	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment { return wordSegments(window) }
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	col := &emissionCollector{}
	done := startStream(t, w, nil, col.onCommitted(), col.onDone())

	const words = 8
	for i := 1; i <= words; i++ {
		feedWord(t, w, 48000, i)
		time.Sleep(1100 * time.Millisecond)
	}
	// trailing silence finalizes the remaining tail
	for i := 0; i < 8; i++ {
		time.Sleep(200 * time.Millisecond)
		if err := w.Feed(quietChunk(48000)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done

	reassembled := strings.Fields(strings.Join(append(append([]string{}, col.commits...), col.dones...), " "))
	if len(reassembled) != words {
		t.Errorf("reassembled turn = %v, want %d words", reassembled, words)
	}
	for i, wd := range reassembled {
		if wd != fmt.Sprintf("word%d", i+1) {
			t.Fatalf("turn reassembly broken at %d: %q (commits=%v dones=%v)", i, wd, col.commits, col.dones)
		}
	}
}

// TestStream_WindowTrimToCap: the window is bounded by maxWindow even during
// uninterrupted speech.
func TestStream_WindowTrimToCap(t *testing.T) {
	old := streamDefaults
	defer func() { streamDefaults = old }()
	streamDefaults.maxWindow = 2 * time.Second
	chunkSlack := time.Second // cap is checked on ingest; window may overshoot by one chunk

	ctx := &fakeContext{}
	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment { return wordSegments(window) }
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	done := startStream(t, w, nil, nil, nil)

	for i := 0; i < 6; i++ {
		if err := w.Feed(loudChunk(48000)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(1 * time.Second)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	if len(ctx.windows) == 0 {
		t.Fatal("Process never ran")
	}
	capSamples := int(streamDefaults.maxWindow.Seconds()*float64(modelSampleRate)) + int(chunkSlack.Seconds()*float64(modelSampleRate))
	for i, win := range ctx.windows {
		if len(win) > capSamples {
			t.Errorf("Process window[%d] = %d samples, want <= %d", i, len(win), capSamples)
		}
	}
}

func TestStream_IdleSilence_NoProcessing(t *testing.T) {
	ctx := &fakeContext{}
	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment { return wordSegments(window) }
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	done := startStream(t, w, nil, nil, nil)

	for i := 0; i < 3; i++ {
		time.Sleep(250 * time.Millisecond)
		if err := w.Feed(quietChunk(48000)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	if ctx.processes != 0 {
		t.Errorf("Process called %d times during idle silence, want 0", ctx.processes)
	}
}

func ampChunk(samples int, amp float32) []byte {
	in := make([]float32, samples)
	for i := range in {
		in[i] = amp
	}
	return float32ToBytes(in)
}

// TestStream_NoiseFloor_PauseStillFinalizes is the regression for the reported
// giant-message bug: a phrase spoken over ambient noise (amp 0.02) and ended
// with a pause back DOWN to that same ambient must finalize via onDone. The old
// gate used an absolute 0.005 threshold, so the ambient kept the tail "speaking"
// forever, onDone never fired, and the phrase buffer grew without bound. The
// adaptive gate's pause threshold (≈1.3× ambient) clears the ambient, so the
// pause registers and the phrase ends. The pre-word ambient runs long enough
// for the idle floor to settle on it (cold-start recovery + 2.5/s rise), which
// is what the gate needs to place the pause gate above the ambient.
func TestStream_NoiseFloor_PauseStillFinalizes(t *testing.T) {
	ctx := &fakeContext{detected: "en"}
	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment { return wordSegments(window) }
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	col := &emissionCollector{}
	done := startStream(t, w, nil, col.onCommitted(), col.onDone())

	ambient := ampChunk(48000, 0.02)
	for i := 0; i < 6; i++ {
		if err := w.Feed(ambient); err != nil {
			t.Fatal(err)
		}
		time.Sleep(250 * time.Millisecond)
	}
	feedWord(t, w, 48000, 1)
	time.Sleep(250 * time.Millisecond)
	for i := 0; i < 4; i++ {
		time.Sleep(200 * time.Millisecond)
		if err := w.Feed(ambient); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done

	if ctx.processes != 1 {
		t.Errorf("Process called %d times, want exactly 1 (the finalize)", ctx.processes)
	}
	if len(col.dones) != 1 {
		t.Fatalf("onDone = %v, want the phrase to finalize on a pause to ambient noise", col.dones)
	}
	if col.dones[0] != "word1" {
		t.Errorf("onDone = %q, want %q", col.dones[0], "word1")
	}
}

// TestStream_AmbientNoiseOnly_NoFinalize: steady ambient at 0.02 is below the
// enter gate (≈1.6× floor) so it must never be read as speech — no onDone and
// no commits from a quiet room.
func TestStream_AmbientNoiseOnly_NoFinalize(t *testing.T) {
	ctx := &fakeContext{}
	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment { return wordSegments(window) }
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	col := &emissionCollector{}
	done := startStream(t, w, nil, col.onCommitted(), col.onDone())

	ambient := ampChunk(48000, 0.02)
	for i := 0; i < 3; i++ {
		time.Sleep(250 * time.Millisecond)
		if err := w.Feed(ambient); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	if len(col.dones) != 0 {
		t.Errorf("ambient-only audio produced onDone: %v", col.dones)
	}
	if len(col.commits) != 0 {
		t.Errorf("ambient-only audio produced onCommitted: %v", col.commits)
	}
}

func TestStream_BelowThresholdAudio_CatchAllFires(t *testing.T) {
	ctx := &fakeContext{}
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment { return wordSegments(window) }
	done := startStream(t, w, nil, nil, nil)

	// Amplitude 0.001 is below absFloor (0.005) — the tail never clears the
	// adaptive speech gate, so speaking stays false. But the window has
	// non-zero energy, so the catch-all (maxInterval) must still fire.
	low := float32ToBytes(func() []float32 {
		in := make([]float32, 48000)
		for i := range in {
			in[i] = 0.001
		}
		return in
	}())
	for i := 0; i < 6; i++ {
		time.Sleep(750 * time.Millisecond)
		if err := w.Feed(low); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	if ctx.processes == 0 {
		t.Error("catch-all never ran Process for below-threshold audio")
	}
}

// TestStream_RevisedTail_FinalizedAfterCommit drives the state machine directly
// (no wall-clock gating) to check the revision contract: a word committed via
// agreement is NOT re-emitted, and the phrase boundary uses the LATEST
// transcription of the tail (revisions included).
func TestStream_RevisedTail_FinalizedAfterCommit(t *testing.T) {
	ctx := &fakeContext{detected: "en"}
	ctx.processFn = func(window []float32, seq int) []whispercpp.Segment {
		switch seq {
		case 0:
			return manualSegments(window, "word1 word2 word3", nil)
		case 1:
			// same span, continued: stable prefix word1..word3 → commit word1 word2
			return manualSegments(window, "word1 word2 word3 word4 word5 word6", nil)
		default:
			// trailing silence: whisper re-transcribes the tail REVISED
			// (word2 → word2b); the revision must win — stale text isn't re-emitted.
			return manualSegments(window, "word2b word3 word4 word5 word6", []float32{0.8, 0.9, 0.9, 0.9, 0.7})
		}
	}
	col := &emissionCollector{}
	st := newStreamState(ctx, modelSampleRate, nil, col.onCommitted(), col.onDone(), nil)
	st.params = streamDefaults
	st.window = audioFor("word1 word2 word3")

	if err := st.process(time.Now(), trigCommit); err != nil {
		t.Fatal(err)
	}
	if err := st.process(time.Now(), trigCommit); err != nil {
		t.Fatal(err)
	}
	if err := st.process(time.Now(), trigFinalize); err != nil {
		t.Fatal(err)
	}

	if len(col.commits) != 1 || col.commits[0] != "word1 word2" {
		t.Fatalf("onCommitted = %v, want [word1 word2]", col.commits)
	}
	if len(col.dones) != 1 || col.dones[0] != "word2b word3 word4 word5 word6" {
		t.Fatalf("onDone = %v, want [word2b word3 word4 word5 word6] (revised tail)", col.dones)
	}
	if strings.Contains(col.dones[0], "word1") {
		t.Errorf("done re-emitted committed words: %q", col.dones[0])
	}
	if ctx.processes != 3 {
		t.Errorf("Process called %d times, want 3 (commit pass, finalize)", ctx.processes)
	}
}

// TestStream_CommitsWordEndBeyondWindow: whisper's VAD path maps timestamps
// back to original coordinates and can report the last token's end slightly
// past the audio it was fed (orig_end snaps to the VAD chunk grid). commitRun
// slices the window at that end, so an overshoot must clamp to the window
// length instead of panicking (live crash: "slice bounds out of range
// [160960:160800]").
func TestStream_CommitsWordEndBeyondWindow(t *testing.T) {
	ctx := &fakeContext{}
	overshootSamples := 160 // the real crash overshot a 10.05s window by 160 samples
	st := newStreamState(ctx, modelSampleRate, nil, nil, nil, nil)
	st.params = streamDefaults
	st.params.maxWindow = 2 * time.Second
	st.window = audioFor("word1 word2 word3") // 3 model-seconds of speech

	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment {
		// Last word's end overshoots len(window) by overshootSamples, as the
		// VAD time-mapping table does (orig_end > actual audio length).
		segs := manualSegments(window, "word1 word2 word3", nil)
		toks := segs[0].Tokens
		last := &toks[len(toks)-1]
		last.End = time.Duration(len(window)+overshootSamples) * time.Second / modelSampleRate
		segs[0].End = last.End
		segs[0].Tokens = toks
		return segs
	}

	if err := st.process(time.Now(), trigCap); err != nil {
		t.Fatal(err)
	}
	if len(st.window) != 0 {
		t.Errorf("window not fully committed after cap with overshot word-end: len=%d", len(st.window))
	}
}

// degenerateSegments mirrors manualSegments but snaps every token's start/end
// to the window origin — modeling whisper's VAD time-mapping under-report, the
// mirror image of the overshoot clamp (TestStream_CommitsWordEndBeyondWindow):
// org_end collapses toward the window front, so the commit trim degenerates.
func degenerateSegments(window []float32, text string) []whispercpp.Segment {
	segs := manualSegments(window, text, nil)
	for si := range segs {
		for ti := range segs[si].Tokens {
			segs[si].Tokens[ti].Start = 0
			segs[si].Tokens[ti].End = 0
		}
	}
	return segs
}

// TestStream_DegenerateTimestamps_ExactlyOnce is the regression for the
// reported duplicate-sentence bug: whisper's VAD path can UNDERREPORT token
// end timestamps, collapsing the commit trim to a no-op. The already-committed
// audio stays at the window front, and the finalize (or a later agreeing run)
// re-emits it verbatim — the same sentence appearing two or three times inside
// one interviewer message. The stream must emit every word of the turn exactly
// once across commits and done, regardless of how degenerate the timestamps are.
func TestStream_DegenerateTimestamps_ExactlyOnce(t *testing.T) {
	ctx := &fakeContext{detected: "en"}
	text := "word1 word2 word3 word4 word5"
	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment {
		return degenerateSegments(window, text)
	}
	col := &emissionCollector{}
	st := newStreamState(ctx, modelSampleRate, nil, col.onCommitted(), col.onDone(), nil)
	st.params = streamDefaults
	st.window = audioFor(text)

	for i := 0; i < 2; i++ {
		if err := st.process(time.Now(), trigCommit); err != nil {
			t.Fatal(err)
		}
	}
	// maxWindow cap force-commit: the whole (degraded) transcription is emitted.
	if err := st.process(time.Now(), trigCap); err != nil {
		t.Fatal(err)
	}
	if err := st.process(time.Now(), trigFinalize); err != nil {
		t.Fatal(err)
	}

	reassembled := strings.Fields(strings.Join(append(append([]string{}, col.commits...), col.dones...), " "))
	if len(reassembled) != 5 {
		t.Fatalf("turn reassembled to %d words, want exactly 5: %v (commits=%v dones=%v)", len(reassembled), reassembled, col.commits, col.dones)
	}
	for i, wd := range reassembled {
		if wd != fmt.Sprintf("word%d", i+1) {
			t.Fatalf("turn reassembly broken at %d: %q (commits=%v dones=%v)", i, wd, col.commits, col.dones)
		}
	}
}

// TestStream_BoundaryCollapse_TailStripped pins the real-audio repro (Russian
// interview clip): timestamps are mostly healthy and the phrase commits via
// agreement, but the last committed word's end underreports to far below a
// realistic per-word extent (collapse, not full degeneration). The finalize
// then re-transcribes the boundary word — capitalized, "И" after committed
// "и" — and the phrase reads the same word twice. The floor must flag laggy so
// the replay-strip runs, and the strip must match case-insensitively.
func TestStream_BoundaryCollapse_TailStripped(t *testing.T) {
	ctx := &fakeContext{detected: "ru"}
	ctx.processFn = func(window []float32, seq int) []whispercpp.Segment {
		switch seq {
		case 0:
			// First pass: no agreement yet, nothing commits.
			return manualSegments(window, "word1 word2 word3 word4", nil)
		case 1:
			// Agreeing pass: commit word1..word3, but word3.end collapses to
			// ~500ms — well inside the region an 80ms-per-word floor would
			// accept, yet far below the true ~1s extent of three words.
			segs := manualSegments(window, "word1 word2 word3 word4", nil)
			toks := segs[0].Tokens
			toks[2].End = time.Duration(500) * time.Millisecond
			segs[0].Tokens = toks
			return segs
		default:
			// Finalize: the trimmed window re-transcribes the boundary word,
			// capitalized and parked at the window front (collapsed start).
			segs := manualSegments(window, "Word3 word4 word5", nil)
			toks := segs[0].Tokens
			toks[0].Start = 0
			toks[0].End = time.Duration(80) * time.Millisecond
			segs[0].Tokens = toks
			return segs
		}
	}
	col := &emissionCollector{}
	st := newStreamState(ctx, modelSampleRate, nil, col.onCommitted(), col.onDone(), nil)
	st.params = streamDefaults
	st.window = audioFor("word1 word2 word3 word4 word5")

	if err := st.process(time.Now(), trigCommit); err != nil {
		t.Fatal(err)
	}
	if err := st.process(time.Now(), trigCommit); err != nil {
		t.Fatal(err)
	}
	if err := st.process(time.Now(), trigFinalize); err != nil {
		t.Fatal(err)
	}

	reassembled := strings.Fields(strings.ToLower(strings.Join(append(append([]string{}, col.commits...), col.dones...), " ")))
	want := []string{"word1", "word2", "word3", "word4", "word5"}
	if len(reassembled) != len(want) {
		t.Fatalf("turn reassembled to %d words, want exactly %d: %v (commits=%v dones=%v)",
			len(reassembled), len(want), reassembled, col.commits, col.dones)
	}
	for i, wd := range reassembled {
		if wd != want[i] {
			t.Fatalf("reassembly broken at %d: got %q want %q (commits=%v dones=%v)",
				i, wd, want[i], col.commits, col.dones)
		}
	}
}

// TestStream_GenuineRepeat_Preserved guards the replay-strip behind
// degenerate-timestamp detection: when timestamps are healthy, an emphatic
// genuine repetition ("word3 word3") must survive commits and done untouched —
// the strip must never run unless the phrase is flagged as lagged.
func TestStream_GenuineRepeat_Preserved(t *testing.T) {
	ctx := &fakeContext{detected: "en"}
	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment { return wordSegments(window) }
	col := &emissionCollector{}
	st := newStreamState(ctx, modelSampleRate, nil, col.onCommitted(), col.onDone(), nil)
	st.params = streamDefaults
	st.window = audioFor("word1 word2 word3 word3 word4 word5 word6")

	if err := st.process(time.Now(), trigCommit); err != nil {
		t.Fatal(err)
	}
	if err := st.process(time.Now(), trigCommit); err != nil {
		t.Fatal(err)
	}
	if err := st.process(time.Now(), trigFinalize); err != nil {
		t.Fatal(err)
	}

	reassembled := strings.Fields(strings.Join(append(append([]string{}, col.commits...), col.dones...), " "))
	want := []string{"word1", "word2", "word3", "word3", "word4", "word5", "word6"}
	if len(reassembled) != len(want) {
		t.Fatalf("reassembled turn = %v, want %v (commits=%v dones=%v)", reassembled, want, col.commits, col.dones)
	}
	for i, wd := range reassembled {
		if wd != want[i] {
			t.Fatalf("turn reassembly broken at %d: got %q want %q (commits=%v dones=%v)", i, wd, want[i], col.commits, col.dones)
		}
	}
}

func TestStream_EmptyPhrase_NoDone(t *testing.T) {
	ctx := &fakeContext{}
	ctx.processFn = func(window []float32, _ int) []whispercpp.Segment { return wordSegments(window) }
	col := &emissionCollector{}
	st := newStreamState(ctx, modelSampleRate, nil, col.onCommitted(), col.onDone(), nil)
	st.params = streamDefaults
	st.window = make([]float32, modelSampleRate) // a second of silence → no words

	if err := st.process(time.Now(), trigFinalize); err != nil {
		t.Fatal(err)
	}
	if err := st.process(time.Now(), trigCommit); err != nil {
		t.Fatal(err)
	}
	if len(col.dones) != 0 {
		t.Errorf("onDone fired for empty phrase: %v", col.dones)
	}
	if len(col.commits) != 0 {
		t.Errorf("onCommitted fired for empty phrase: %v", col.commits)
	}
}

// TestWordsFromSegments_SkipsControlTokens: whisper.cpp control tokens
// ([_BEG_], [_TT_NNN], [_T0]) must never surface as words — the binding's
// IsText filter decides textness by token id, not by text shape.
func TestWordsFromSegments_SkipsControlTokens(t *testing.T) {
	segs := []whispercpp.Segment{{
		Num:   0,
		Text:  "[_BEG_] hello world [_TT_190] again [_T0] [BLANK_AUDIO]",
		Start: 0,
		End:   1000 * time.Millisecond,
		Tokens: []whispercpp.Token{
			{Id: 501, Text: "[_BEG_]", Start: 0, End: 100 * time.Millisecond},
			{Id: 1, Text: " hello", P: 0.9, Start: 100 * time.Millisecond, End: 300 * time.Millisecond},
			{Id: 2, Text: " world", P: 0.9, Start: 300 * time.Millisecond, End: 500 * time.Millisecond},
			{Id: 512, Text: "[_TT_190]", Start: 500 * time.Millisecond, End: 600 * time.Millisecond},
			{Id: 3, Text: " again", P: 0.9, Start: 600 * time.Millisecond, End: 800 * time.Millisecond},
			{Id: 502, Text: "[_T0]", Start: 800 * time.Millisecond, End: 900 * time.Millisecond},
			{Id: 480, Text: "[BLANK_AUDIO]", Start: 900 * time.Millisecond, End: 1000 * time.Millisecond},
		},
	}}
	// The fake marks control tokens by id, but isBracketToken must catch
	// [BLANK_AUDIO] even when its id looks "texty" (n-token vocab range).
	isText := func(tok whispercpp.Token) bool { return tok.Id < 500 }

	got := joinWords(wordsFromSegments(segs, isText))
	if got != "hello world again" {
		t.Fatalf("wordsFromSegments = %q, want %q (control tokens must be skipped)", got, "hello world again")
	}
}

func TestFeed_AfterClose_ReturnsError(t *testing.T) {
	w := newFakeWhisper(t, &fakeModel{ctx: &fakeContext{}})
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Feed(nil); err != io.ErrClosedPipe {
		t.Errorf("Feed after Close = %v, want io.ErrClosedPipe", err)
	}
}

// TestClose_FreesModel: the loaded whisper model is a native (C) allocation
// holding the weights and the Vulkan buffers; Go's GC cannot reclaim it. Close
// must call model.Close() (Whisper_free) or the model stays resident for the
// whole process lifetime — and leaks on every rebuildSTT (each StartListening).
func TestClose_FreesModel(t *testing.T) {
	model := &fakeModel{ctx: &fakeContext{}}
	w := newFakeWhisper(t, model)
	if err := w.Preload(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if !model.closed {
		t.Fatal("Close() did not free the native model — Whisper_free was never called")
	}
}

// TestClose_FreesModelWhileStreaming: Close can race a running Stream goroutine
// that may be mid-Process. Close must stop the stream and wait for it before
// freeing the model, and must still free exactly once.
func TestClose_FreesModelWhileStreaming(t *testing.T) {
	model := &fakeModel{ctx: &fakeContext{}}
	w := newFakeWhisper(t, model)
	if err := w.Preload(); err != nil {
		t.Fatal(err)
	}
	done := startStream(t, w, nil, nil, nil)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil && err != io.ErrClosedPipe {
		t.Fatalf("Stream() error: %v", err)
	}
	if !model.closed {
		t.Fatal("Close() while streaming did not free the native model")
	}
}

// TestPreload_AfterClose_Errors: a closed adapter must not load a new model —
// otherwise the model loaded after Close could never be freed.
func TestPreload_AfterClose_Errors(t *testing.T) {
	w := newFakeWhisper(t, &fakeModel{ctx: &fakeContext{}})
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Preload(); err == nil {
		t.Fatal("Preload() after Close must error, not load a new model")
	}
}

// TestFeed_CopiesChunkBeforeEnqueue guards against buffer aliasing: the pulse
// pump reads every chunk into the SAME backing array (capture_linux.go), so
// Feed must copy before handing the slice to the stream. Otherwise all queued
// chunks alias one array that the pump overwrites during the next read, and
// the stream transcribes a blend of every later frame as noise — exactly the
// "*Mumbling*" symptom seen on live system audio (offline replays of the same
// bytes are perfect because each chunk is a distinct slice).
func TestFeed_CopiesChunkBeforeEnqueue(t *testing.T) {
	w := newFakeWhisper(t, &fakeModel{ctx: &fakeContext{}})

	src := loudChunk(48000) // 0.1 amplitude → all bytes non-zero
	want := append([]byte(nil), src...)
	if err := w.Feed(src); err != nil {
		t.Fatal(err)
	}
	// Producer reuses the buffer: overwrite every byte before the stream reads it.
	for i := range src {
		src[i] = 0
	}

	select {
	case got := <-w.feed:
		if !bytes.Equal(got, want) {
			t.Fatalf("queued chunk was aliased by the shared buffer — Feed must copy\n got %d bytes with prefix %v\nwant %d bytes with prefix %v", len(got), got[:8], len(want), want[:8])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Feed never enqueued the chunk")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// manualSegments turns a space-separated word string into segments with
// timestamps spread evenly across the window (used by the hand-driven tests).
func manualSegments(window []float32, text string, probs []float32) []whispercpp.Segment {
	words := strings.Fields(text)
	if len(words) == 0 || len(window) == 0 {
		return nil
	}
	span := len(window) / len(words)
	toks := make([]whispercpp.Token, 0, len(words))
	start := 0
	for i, wd := range words {
		end := start + span
		if i == len(words)-1 {
			end = len(window)
		}
		p := float32(0.9)
		if probs != nil && i < len(probs) {
			p = probs[i]
		}
		toks = append(toks, whispercpp.Token{
			Id:    i,
			Text:  " " + wd,
			P:     p,
			Start: time.Duration(start) * time.Second / modelSampleRate,
			End:   time.Duration(end) * time.Second / modelSampleRate,
		})
		start = end
	}
	return []whispercpp.Segment{{
		Num:    0,
		Text:   text,
		Start:  0,
		End:    time.Duration(len(window)) * time.Second / modelSampleRate,
		Tokens: toks,
	}}
}

// audioFor builds 1s-per-word DC-level audio for a space-separated word list.
func audioFor(text string) []float32 {
	words := strings.Fields(text)
	var out []float32
	for _, wd := range words {
		id, err := strconv.Atoi(strings.TrimPrefix(wd, "word"))
		if err != nil {
			panic("audioFor: not a wordN token")
		}
		amp := float32(0.05) + float32(id)*wordAmpStep
		s := make([]float32, modelSampleRate)
		for i := range s {
			s[i] = amp
		}
		out = append(out, s...)
	}
	return out
}
