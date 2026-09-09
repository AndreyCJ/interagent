package whisper

import (
	"errors"
	"io"
	"testing"
	"time"

	whispercpp "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type fakeContext struct {
	detected  string
	setLang   []string
	threads   uint
	vad       bool
	vadModel  string
	vadThresh float32
	processFn func(window []float32, enc whispercpp.EncoderBeginCallback, seg whispercpp.SegmentCallback, prog whispercpp.ProgressCallback) error
	processes int
}

func (f *fakeContext) SetLanguage(l string) error { f.setLang = append(f.setLang, l); return nil }
func (f *fakeContext) SetThreads(v uint)          { f.threads = v }
func (f *fakeContext) SetVAD(v bool)              { f.vad = v }
func (f *fakeContext) SetVADModelPath(p string)   { f.vadModel = p }
func (f *fakeContext) SetVADThreshold(t float32)  { f.vadThresh = t }

func (f *fakeContext) Process(window []float32, enc whispercpp.EncoderBeginCallback, seg whispercpp.SegmentCallback, prog whispercpp.ProgressCallback) error {
	f.processes++
	if f.processFn != nil {
		return f.processFn(window, enc, seg, prog)
	}
	return nil
}

func (f *fakeContext) NextSegment() (whispercpp.Segment, error) { return whispercpp.Segment{}, io.EOF }
func (f *fakeContext) DetectedLanguage() string                 { return f.detected }

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

func TestStream_ConfiguresContext(t *testing.T) {
	ctx := &fakeContext{}
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	done := make(chan error, 1)
	go func() { done <- w.Stream(48000, nil, nil) }()

	time.Sleep(20 * time.Millisecond) // let Stream reach the loop
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
}

func TestStream_ConfiguredLanguage(t *testing.T) {
	ctx := &fakeContext{}
	w := newWithOpener("model.bin", "vad.onnx", "ru", func(string) (sttModel, error) {
		return &fakeModel{ctx: ctx}, nil
	})
	done := make(chan error, 1)
	go func() { done <- w.Stream(48000, nil, nil) }()
	time.Sleep(20 * time.Millisecond)
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
	if err := w.Stream(48000, nil, nil); err == nil {
		t.Fatal("Stream() should return model load error")
	}
}

func loudChunk(samples int) []byte {
	in := make([]float32, samples)
	for i := range in {
		in[i] = 0.1
	}
	return float32ToBytes(in)
}

func quietChunk(samples int) []byte {
	return float32ToBytes(make([]float32, samples))
}

func TestStream_SilenceAfterSpeech_FinalizesOnce(t *testing.T) {
	ctx := &fakeContext{detected: "en"}
	call := 0
	ctx.processFn = func(window []float32, enc whispercpp.EncoderBeginCallback, seg whispercpp.SegmentCallback, prog whispercpp.ProgressCallback) error {
		call++
		enc()
		if call == 1 {
			seg(whispercpp.Segment{Text: "Hello ", End: time.Second, Tokens: []whispercpp.Token{{P: 0.9}, {P: 0.7}}})
		}
		return nil
	}
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	var dones []string
	done := make(chan error, 1)
	go func() {
		done <- w.Stream(48000, nil, func(text string, _ float64, _ string) { dones = append(dones, text) })
	}()

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
	if call != 1 {
		t.Errorf("Process called %d times, want exactly 1 (silence-gated)", call)
	}
	if len(dones) != 1 || dones[0] != "Hello " {
		t.Fatalf("onDone = %v, want one phrase 'Hello '", dones)
	}
}

func TestStream_LongSpeech_BackstopProcessesNoDone(t *testing.T) {
	ctx := &fakeContext{}
	call := 0
	ctx.processFn = func(window []float32, enc whispercpp.EncoderBeginCallback, seg whispercpp.SegmentCallback, prog whispercpp.ProgressCallback) error {
		call++
		enc()
		// Segment always ends at the window tail so the phrase never finalizes
		// mid-speech; only the backstop cadence drives these Process calls.
		seg(whispercpp.Segment{Text: "Hello world", End: time.Duration(len(window)) * time.Second / modelSampleRate, Tokens: []whispercpp.Token{{P: 0.8}}})
		return nil
	}
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	var partials []string
	done := make(chan error, 1)
	go func() { done <- w.Stream(48000, func(p string) { partials = append(partials, p) }, nil) }()

	for i := 0; i < 4; i++ {
		time.Sleep(1 * time.Second)
		if err := w.Feed(loudChunk(48000)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	if call == 0 {
		t.Error("backstop never ran Process during continuous speech")
	}
	if len(partials) == 0 {
		t.Error("backstop process must produce partial text")
	}
}

func TestStream_IdleSilence_NoProcessing(t *testing.T) {
	ctx := &fakeContext{}
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	done := make(chan error, 1)
	go func() { done <- w.Stream(48000, nil, nil) }()

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

func TestStream_BelowThresholdAudio_CatchAllFires(t *testing.T) {
	ctx := &fakeContext{}
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	call := 0
	ctx.processFn = func(window []float32, enc whispercpp.EncoderBeginCallback, seg whispercpp.SegmentCallback, prog whispercpp.ProgressCallback) error {
		call++
		enc()
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- w.Stream(48000, nil, nil) }()

	// Amplitude 0.001 is below gateThreshold (0.005) — the tail stays "quiet",
	// but the window has non-zero energy, so the catch-all must still fire.
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
	if call == 0 {
		t.Error("catch-all never ran Process for below-threshold audio")
	}
}

func TestStream_WindowTrimmedToCap(t *testing.T) {
	ctx := &fakeContext{}
	gotLen := 0
	ctx.processFn = func(window []float32, enc whispercpp.EncoderBeginCallback, seg whispercpp.SegmentCallback, prog whispercpp.ProgressCallback) error {
		gotLen = len(window)
		enc()
		seg(whispercpp.Segment{Text: "x", End: time.Duration(len(window)) * time.Second / modelSampleRate})
		return nil
	}
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	done := make(chan error, 1)
	go func() { done <- w.Stream(48000, nil, nil) }()

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
	if max := modelSampleRate * windowSeconds; gotLen > max {
		t.Errorf("Process window = %d samples, cap is %d", gotLen, max)
	}
	if gotLen == 0 {
		t.Error("Process never ran")
	}
}

func TestStream_EmptyPhrase_NoDone(t *testing.T) {
	ctx := &fakeContext{}
	ctx.processFn = func(window []float32, enc whispercpp.EncoderBeginCallback, seg whispercpp.SegmentCallback, prog whispercpp.ProgressCallback) error {
		enc()
		enc() // boundary with no segments → no phrase
		return nil
	}
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	var dones []string
	done := make(chan error, 1)
	go func() { done <- w.Stream(48000, nil, func(t string, _ float64, _ string) { dones = append(dones, t) }) }()

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
	if len(dones) != 0 {
		t.Errorf("onDone fired for empty phrase: %v", dones)
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

func TestWhisper_ConformsToPortSTT(t *testing.T) {
	var _ sttModel = (*fakeModel)(nil)
}
