package whisper

import (
	whispercpp "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

// sttModel / sttContext are narrow seams over the whisper.cpp binding so the
// streaming state machine is unit-testable without a real model file.
type sttModel interface {
	NewContext() (sttContext, error)
	Close() error
}

type sttContext interface {
	SetLanguage(string) error
	SetThreads(uint)
	SetVAD(bool)
	SetVADModelPath(string)
	SetVADThreshold(float32)
	Process([]float32, whispercpp.EncoderBeginCallback, whispercpp.SegmentCallback, whispercpp.ProgressCallback) error
	NextSegment() (whispercpp.Segment, error)
	DetectedLanguage() string
}

// Compile-time assertions that the real binding satisfies the seams.
var (
	_ sttContext = (whispercpp.Context)(nil)
	_ sttModel   = (*realModel)(nil)
)

type realModel struct{ m whispercpp.Model }

func (r realModel) NewContext() (sttContext, error) { return r.m.NewContext() }
func (r realModel) Close() error                    { return r.m.Close() }

type engineOpener func(path string) (sttModel, error)

func realOpener(path string) (sttModel, error) {
	m, err := whispercpp.New(path)
	if err != nil {
		return nil, err
	}
	return realModel{m}, nil
}
