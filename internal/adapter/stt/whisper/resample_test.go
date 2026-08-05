package whisper

import (
	"math"
	"testing"
)

func TestResample_Downsample_48kTo16k(t *testing.T) {
	in := make([]float32, 48)
	for i := range in {
		in[i] = float32(i)
	}
	out := resample(in, 48000, 16000)
	if len(out) != 16 {
		t.Fatalf("resample length = %d, want 16", len(out))
	}
	if out[0] != 0 || out[15] != 45 {
		t.Errorf("resample endpoints = (%v, %v), want (0, 45)", out[0], out[15])
	}
}

func TestResample_SameRate_NoCopyNeeded(t *testing.T) {
	in := []float32{1, 2, 3}
	out := resample(in, 16000, 16000)
	if len(out) != 3 || out[2] != 3 {
		t.Errorf("same-rate resample = %v", out)
	}
}

func TestBytesFloat32_RoundTrip(t *testing.T) {
	in := []float32{0.0, -1.0, 0.5, math.MaxFloat32}
	back := bytesToFloat32(float32ToBytes(in))
	for i := range in {
		if back[i] != in[i] {
			t.Errorf("round trip %d: %v != %v", i, back[i], in[i])
		}
	}
}
