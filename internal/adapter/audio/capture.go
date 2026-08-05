// Package audio implements the AudioInput port: microphone (AVCaptureDevice)
// and system sound (ScreenCaptureKit) capture on macOS, and a stub elsewhere.
// Chunks are little-endian float32 mono PCM at CaptureSampleRate.
package audio

import (
	"encoding/binary"
	"math"
)

const CaptureSampleRate = 48000

// toMonoFloat32 averages interleaved channel samples into mono.
func toMonoFloat32(interleaved []float32, channels int) []float32 {
	if channels <= 1 {
		return interleaved
	}
	n := len(interleaved) / channels
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		var sum float64
		for c := 0; c < channels; c++ {
			sum += float64(interleaved[i*channels+c])
		}
		out[i] = float32(sum / float64(channels))
	}
	return out
}

func bytesToFloat32(b []byte) []float32 {
	n := len(b) / 4
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}

func float32ToBytes(samples []float32) []byte {
	b := make([]byte, len(samples)*4)
	for i, s := range samples {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(s))
	}
	return b
}
