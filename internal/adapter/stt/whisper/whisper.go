// Package whisper implements port.STT on the whisper.cpp Go binding (the
// frozen cgo exception, ADR-005). Streaming uses Silero VAD (ADR-007 option D);
// phrases finalize on trailing silence between single-segment re-transcriptions.
package whisper

import (
	"io"
	"sync"
)

// whisper.cpp native sample rate (samples/sec, float32 mono).
const modelSampleRate = 16000

// Tunable stream constants (validated by the CI integration test).
const (
	vadThreshold  = 0.6
	windowSeconds = 5
)

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
	if w.loaded {
		return w.model, w.loadErr
	}
	w.model, w.loadErr = w.opener(w.path)
	w.loaded = true
	return w.model, w.loadErr
}

func (w *Whisper) Feed(chunk []byte) error {
	select {
	case <-w.close:
		return io.ErrClosedPipe
	default:
	}
	select {
	case w.feed <- chunk:
		return nil
	case <-w.close:
		return io.ErrClosedPipe
	}
}

func (w *Whisper) Stream(sampleRate int, onPartial func(string), onDone func(text string, confidence float64, language string)) error {
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
	ctx.SetThreads(streamThreads)
	lang := w.language
	if lang == "" {
		lang = "auto"
	}
	if err := ctx.SetLanguage(lang); err != nil {
		return err
	}

	st := newStreamState(ctx, sampleRate, onPartial, onDone)
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
	w.once.Do(func() { close(w.close) })
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
