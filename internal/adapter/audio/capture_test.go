package audio

import (
	"math"
	"strings"
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

func TestSystemStartErrorMsg_EmptyRawUsesFallback(t *testing.T) {
	if msg := systemStartErrorMsg(""); !strings.Contains(msg, "stream start failed") {
		t.Errorf("empty raw -> %q, want fallback containing %q", msg, "stream start failed")
	}
}

func TestSystemStartErrorMsg_PassesThroughRealError(t *testing.T) {
	raw := "SCError -3803 missingEntitlements: capture requires entitlements"
	if msg := systemStartErrorMsg(raw); msg != raw {
		t.Errorf("raw %q -> %q, want passthrough", raw, msg)
	}
}
