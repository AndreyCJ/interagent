// Package whisper implements port.STT on the whisper.cpp Go binding (the
// frozen cgo exception, ADR-005). Streaming uses Silero VAD (ADR-007 option D);
// Process runs only at gate triggers and reads back full segment results via
// NextSegment (no SegmentCallback, so the binding's single_segment forcing is
// avoided). Words stable across consecutive runs are committed mid-speech and
// re-emitted only through onCommitted; the final phrase tail goes to onDone.
package whisper

import (
	"errors"
	"io"
	"sync"
)

// whisper.cpp native sample rate (samples/sec, float32 mono).
const modelSampleRate = 16000

// Tunable stream constants (validated by the CI integration test).
const (
	vadThreshold = 0.6
)

// errStreamStarted guards the single-use Stream contract: rebuildSTT creates a
// fresh adapter per listening cycle, so a second Stream on one adapter is a bug.
var errStreamStarted = errors.New("whisper: stream already started")

// whisper.cpp inference threads (ADR-005). Fixed at 4: the binding defaults to
// runtime.NumCPU(), which saturates the M1 Pro during every Process call.
const streamThreads = 4

type Whisper struct {
	mu       sync.Mutex
	path     string
	vadPath  string
	language string
	opener   engineOpener
	feed     chan []byte
	close    chan struct{}
	once     sync.Once
	model    sttModel
	loaded   bool
	loadErr  error

	// Stream lifecycle: streamDone lets Close wait for the stream goroutine to
	// stop before freeing the model it may still be using.
	streamStarted bool
	streamDone    chan struct{}
	closed        bool
}

func New(modelPath, vadModelPath, language string) *Whisper {
	return newWithOpener(modelPath, vadModelPath, language, realOpener)
}

func newWithOpener(modelPath, vadModelPath, language string, opener engineOpener) *Whisper {
	return &Whisper{
		path:     modelPath,
		vadPath:  vadModelPath,
		language: language,
		opener:   opener,
		feed:     make(chan []byte, 128),
		close:    make(chan struct{}),
	}
}

func (w *Whisper) load() (sttModel, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil, io.ErrClosedPipe
	}
	if w.loaded {
		return w.model, w.loadErr
	}
	w.model, w.loadErr = w.opener(w.path)
	w.loaded = true
	return w.model, w.loadErr
}

// beginStream registers the single stream goroutine and refuses if the adapter
// is already closed (so a model loaded after Close can never be orphaned).
func (w *Whisper) beginStream() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return io.ErrClosedPipe
	}
	if w.streamStarted {
		return errStreamStarted
	}
	w.streamStarted = true
	w.streamDone = make(chan struct{})
	return nil
}

// endStream signals the stream goroutine has returned.
func (w *Whisper) endStream() {
	w.mu.Lock()
	done := w.streamDone
	w.streamDone = nil
	w.mu.Unlock()
	if done != nil {
		close(done)
	}
}

// Preload loads the model synchronously so a missing or corrupt model fails
// fast — StartListening calls this before the audio capture starts, so the
// caller gets a real error instead of silent background STT failure.
func (w *Whisper) Preload() error {
	_, err := w.load()
	return err
}

func (w *Whisper) Feed(chunk []byte) error {
	select {
	case <-w.close:
		return io.ErrClosedPipe
	default:
	}
	// Copy before enqueue: the capture pump reads every chunk into the same
	// backing array (capture_linux.go), so handing that slice to the buffered
	// channel would alias all queued chunks to one buffer the pump overwrites
	// on the next read — the stream would transcribe a blend of later frames
	// as noise instead of the spoken words.
	buf := make([]byte, len(chunk))
	copy(buf, chunk)
	select {
	case w.feed <- buf:
		return nil
	case <-w.close:
		return io.ErrClosedPipe
	}
}

func (w *Whisper) Stream(sampleRate int, onPartial func(string), onCommitted func(string), onDone func(text string, confidence float64, language string), onStatus func(bool)) error {
	if err := w.beginStream(); err != nil {
		return err
	}
	defer w.endStream()

	model, err := w.load()
	if err != nil {
		return err
	}
	ctx, err := model.NewContext()
	if err != nil {
		return err
	}
	ctx.SetVAD(true)
	ctx.SetVADModelPath(w.vadPath)
	ctx.SetVADThreshold(vadThreshold)
	ctx.SetTokenTimestamps(true) // word-level commit points need token timestamps
	ctx.SetThreads(streamThreads)
	lang := w.language
	if lang == "" {
		lang = "auto"
	}
	if err := ctx.SetLanguage(lang); err != nil {
		return err
	}

	st := newStreamState(ctx, sampleRate, onPartial, onCommitted, onDone, onStatus)
	for {
		select {
		case <-w.close:
			// Graceful drain: after close both cases are ready (feed is
			// buffered), so a bare select would return after ~1 chunk and
			// drop queued audio. Process anything already queued first.
			for {
				select {
				case chunk, ok := <-w.feed:
					if !ok {
						return nil
					}
					if err := st.ingest(chunk); err != nil {
						return err
					}
				default:
					return nil
				}
			}
		case chunk, ok := <-w.feed:
			if !ok {
				return nil
			}
			if err := st.ingest(chunk); err != nil {
				return err
			}
		}
	}
}

func (w *Whisper) Close() error {
	w.once.Do(func() {
		w.mu.Lock()
		w.closed = true
		started := w.streamStarted
		done := w.streamDone
		w.mu.Unlock()

		close(w.close)
		// Stop the stream before freeing the model: a Process may still be
		// running against the native context. If the stream goroutine ended on
		// its own, streamDone was already cleared.
		if started && done != nil {
			<-done
		}

		w.mu.Lock()
		model := w.model
		loaded := w.loaded
		w.model = nil
		w.loaded = false
		w.mu.Unlock()

		// model.Close() is Whisper_free: it releases the weights and the Vulkan
		// buffers, which Go's GC cannot reclaim (raw C pointer). Without this
		// every rebuildSTT / StartListening leaks a whole model.
		if loaded && model != nil {
			_ = model.Close()
		}
	})
	return nil
}

// bytesToFloat32 decodes little-endian float32 PCM.
func bytesToFloat32(b []byte) []float32 {
	n := len(b) / 4
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		out[i] = float32frombitsLE(b[i*4:])
	}
	return out
}

// float32ToBytes encodes little-endian float32 PCM.
func float32ToBytes(samples []float32) []byte {
	b := make([]byte, len(samples)*4)
	for i, s := range samples {
		putFloat32LE(b[i*4:], s)
	}
	return b
}

// resample linearly resamples float32 samples from -> to samples/sec.
func resample(in []float32, from, to int) []float32 {
	if from == to || len(in) == 0 {
		return in
	}
	n := len(in) * to / from
	if n == 0 {
		return nil
	}
	out := make([]float32, 0, n)
	step := float64(from) / float64(to)
	for i := 0; i < n; i++ {
		pos := float64(i) * step
		i0 := int(pos)
		i1 := i0 + 1
		frac := pos - float64(i0)
		switch {
		case i0 >= len(in):
			out = append(out, in[len(in)-1])
		case i1 >= len(in):
			out = append(out, in[i0])
		default:
			out = append(out, float32(float64(in[i0])*(1-frac)+float64(in[i1])*frac))
		}
	}
	return out
}
