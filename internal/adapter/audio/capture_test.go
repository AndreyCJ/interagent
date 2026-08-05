package audio

import (
	"math"
	"testing"
)

func TestToMonoFloat32_StereoAverages(t *testing.T) {
	// interleaved L R L R
	in := []float32{1, 3, 1, 3, 1, 3}
	mono := toMonoFloat32(in, 2)
	if len(mono) != 3 {
		t.Fatalf("mono length = %d, want 3", len(mono))
	}
	if mono[0] != 2 || mono[1] != 2 || mono[2] != 2 {
		t.Errorf("mono = %v, want [2 2 2]", mono)
	}
}

func TestBytesFloat32_RoundTrip(t *testing.T) {
	in := []float32{0, -1, 0.5, math.MaxFloat32, math.SmallestNonzeroFloat32}
	back := bytesToFloat32(float32ToBytes(in))
	for i := range in {
		if back[i] != in[i] {
			t.Errorf("round trip %d: %v != %v", i, back[i], in[i])
		}
	}
}
