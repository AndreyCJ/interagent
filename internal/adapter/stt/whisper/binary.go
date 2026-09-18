package whisper

import (
	"encoding/binary"
	"math"
)

func float32frombitsLE(b []byte) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(b))
}

func putFloat32LE(b []byte, v float32) {
	binary.LittleEndian.PutUint32(b, math.Float32bits(v))
}
