package usecase

import (
	"testing"
)

type mockScreenCapture struct {
	fullData   []byte
	fullErr    error
	regionData []byte
	regionErr  error
}

func (m *mockScreenCapture) CaptureFull() ([]byte, error) {
	return m.fullData, m.fullErr
}

func (m *mockScreenCapture) CaptureRegion() ([]byte, error) {
	return m.regionData, m.regionErr
}

type mockOCR struct {
	text string
	err  error
}

func (m *mockOCR) ExtractText(image []byte) (string, error) {
	return m.text, m.err
}

func TestScreenshot_CaptureFull(t *testing.T) {
	capture := &mockScreenCapture{fullData: []byte("image-data")}
	ocr := &mockOCR{}
	s := NewScreenshot(capture, ocr)

	err := s.CaptureFull()
	if err != nil {
		t.Fatalf("CaptureFull() returned error: %v", err)
	}
}

func TestScreenshot_CaptureRegion(t *testing.T) {
	capture := &mockScreenCapture{regionData: []byte("region-data")}
	ocr := &mockOCR{}
	s := NewScreenshot(capture, ocr)

	err := s.CaptureRegion()
	if err != nil {
		t.Fatalf("CaptureRegion() returned error: %v", err)
	}
}

func TestScreenshot_CaptureAndOCR_ReturnsText(t *testing.T) {
	capture := &mockScreenCapture{regionData: []byte("screenshot")}
	ocr := &mockOCR{text: "recognized text from screenshot"}
	s := NewScreenshot(capture, ocr)

	text, err := s.CaptureAndOCR()
	if err != nil {
		t.Fatalf("CaptureAndOCR() returned error: %v", err)
	}
	if text == "" {
		t.Error("CaptureAndOCR() returned empty text")
	}
}
