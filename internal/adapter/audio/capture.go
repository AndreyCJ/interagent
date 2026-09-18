// Package audio implements the AudioInput port: microphone (AVCaptureDevice)
// and system sound (ScreenCaptureKit) capture on macOS, and a stub elsewhere.
// Chunks are little-endian float32 mono PCM at CaptureSampleRate.
package audio

import (
	"encoding/binary"
	"math"
	"strings"
)

const CaptureSampleRate = 48000

// systemStartErrorMsg returns the raw SCStream start error if one was
// reported, otherwise a stable fallback. The ObjC adapter writes the
// ScreenCaptureKit code + description into the error buffer on failure, but an
// empty buffer must still produce a diagnostic (ADR-008: no silent failures).
func systemStartErrorMsg(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "screencapturekit: stream start failed"
	}
	return raw
}

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
