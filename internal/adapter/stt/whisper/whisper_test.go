package whisper

import (
	"errors"
	"io"
	"math"
	"testing"
	"time"

	whispercpp "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type fakeContext struct {
	detected    string
	setLang     []string
	vad         bool
	vadModel    string
	vadThresh   float32
	processFn   func(window []float32, enc whispercpp.EncoderBeginCallback, seg whispercpp.SegmentCallback, prog whispercpp.ProgressCallback) error
	processes   int
}

func (f *fakeContext) SetLanguage(l string) error { f.setLang = append(f.setLang, l); return nil }
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
	w := newWithOpener("model.bin", "vad.onnx", func(string) (sttModel, error) { return model, nil })
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
	if !ctx.vad || ctx.vadModel != "vad.onnx" || ctx.vadThresh != 0.6 {
		t.Errorf("VAD config = (%v, %q, %v)", ctx.vad, ctx.vadModel, ctx.vadThresh)
	}
}

func TestStream_LoadError(t *testing.T) {
	w := newWithOpener("model.bin", "vad.onnx", func(string) (sttModel, error) {
		return nil, errors.New("cannot open model")
	})
	if err := w.Stream(48000, nil, nil); err == nil {
		t.Fatal("Stream() should return model load error")
	}
}

func TestStream_PartialReplacesAcrossCalls(t *testing.T) {
	ctx := &fakeContext{}
	texts := []string{"Hel", "Hello", "Hello world"}
	call := 0
	ctx.processFn = func(window []float32, enc whispercpp.EncoderBeginCallback, seg whispercpp.SegmentCallback, prog whispercpp.ProgressCallback) error {
		enc()
		if call < len(texts) {
			// single_segment: one segment per call = the full window re-transcription
			seg(whispercpp.Segment{Text: texts[call]})
		}
		call++
		return nil
	}
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	var partials []string
	done := make(chan error, 1)
	go func() { done <- w.Stream(48000, func(p string) { partials = append(partials, p) }, nil) }()

	for range texts {
		if err := w.Feed(float32ToBytes(make([]float32, 48000))); err != nil {
			t.Fatalf("Feed() error: %v", err)
		}
	}
	time.Sleep(20 * time.Millisecond)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	want := []string{"Hel", "Hello", "Hello world"}
	if len(partials) != len(want) || partials[0] != want[0] || partials[1] != want[1] || partials[2] != want[2] {
		t.Errorf("partials = %v, want %v (replaced, not accumulated)", partials, want)
	}
}

func TestStream_EndpointBoundary_FinalizesPhrase(t *testing.T) {
	ctx := &fakeContext{detected: "en"}
	call := 0
	ctx.processFn = func(window []float32, enc whispercpp.EncoderBeginCallback, seg whispercpp.SegmentCallback, prog whispercpp.ProgressCallback) error {
		call++
		enc()
		if call <= 2 {
			// Re-transcribe the same 1s speech as the window grows; by the
			// third call the segment leaves >= finalizeSilence of trailing
			// silence, so the second call's encoder-begin finalizes it.
			seg(whispercpp.Segment{Text: "Hello ", End: time.Second, Tokens: []whispercpp.Token{{P: 0.9}, {P: 0.7}}})
		}
		return nil
	}
	w := newFakeWhisper(t, &fakeModel{ctx: ctx})
	var dones []string
	var confs []float64
	var langs []string
	done := make(chan error, 1)
	go func() {
		done <- w.Stream(48000, nil, func(text string, confidence float64, language string) {
			dones = append(dones, text)
			confs = append(confs, confidence)
			langs = append(langs, language)
		})
	}()

	for i := 0; i < 3; i++ {
		if err := w.Feed(float32ToBytes(make([]float32, 48000))); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(20 * time.Millisecond)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
	if len(dones) != 1 || dones[0] != "Hello " {
		t.Fatalf("onDone = %v, want one phrase 'Hello '", dones)
	}
	if math.Abs(confs[0]-0.8) > 1e-4 {
		t.Errorf("confidence = %v, want ~0.8", confs[0])
	}
	if langs[0] != "en" {
		t.Errorf("language = %q, want en", langs[0])
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

	if err := w.Feed(float32ToBytes(make([]float32, 48000))); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
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
