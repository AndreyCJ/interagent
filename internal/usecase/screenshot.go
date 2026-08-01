package usecase

import (
	"interagent/internal/port"
)

type Screenshot struct {
	capture port.ScreenCapture
	ocr     port.OCR
}

func NewScreenshot(capture port.ScreenCapture, ocr port.OCR) *Screenshot {
	return &Screenshot{capture: capture, ocr: ocr}
}

func (s *Screenshot) CaptureFull() error {
	_, err := s.capture.CaptureFull()
	return err
}

func (s *Screenshot) CaptureRegion() error {
	_, err := s.capture.CaptureRegion()
	return err
}

func (s *Screenshot) CaptureAndOCR() (string, error) {
	image, err := s.capture.CaptureRegion()
	if err != nil {
		return "", err
	}
	return s.ocr.ExtractText(image)
}
