package whisper

import (
	"encoding/binary"
	"errors"
)

// decodeWavFloat32 decodes a mono, 16-bit PCM WAV file into float32 samples in
// [-1, 1]. Only used by the gated integration test.
func decodeWavFloat32(data []byte) ([]float32, error) {
	if len(data) < 44 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, errors.New("not a RIFF/WAVE file")
	}
	channels := int(binary.LittleEndian.Uint16(data[22:24]))
	sampleRate := int(binary.LittleEndian.Uint32(data[24:28]))
	bits := int(binary.LittleEndian.Uint16(data[34:36]))
	if channels != 1 {
		return nil, errors.New("only mono wav supported")
	}
	if bits != 16 {
		return nil, errors.New("only 16-bit wav supported")
	}
	if sampleRate != 16000 {
		return nil, errors.New("expected 16000 Hz wav")
	}
	// Find the data chunk.
	off := 12
	for off+8 <= len(data) {
		id := string(data[off : off+4])
		size := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if id == "data" {
			payload := data[off+8 : off+8+size]
			samples := make([]float32, size/2)
			for i := range samples {
				v := int16(binary.LittleEndian.Uint16(payload[i*2:]))
				samples[i] = float32(v) / 32768.0
			}
			return samples, nil
		}
		off += 8 + size + size%2
	}
	return nil, errors.New("no data chunk")
}
