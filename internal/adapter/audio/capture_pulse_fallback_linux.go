//go:build linux && !cgo

package audio

func init() {
	openPulseReader = func(string, int, int) (pulseReader, error) { return nil, pulseReaderErr() }
	listPulseSources = func() ([]pulseSourceInfo, error) { return nil, pulseReaderErr() }
}
