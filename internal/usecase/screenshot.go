package usecase

import (
	"errors"

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
	return errors.New("not implemented")
}

func (s *Screenshot) CaptureRegion() error {
	return errors.New("not implemented")
}

func (s *Screenshot) CaptureAndOCR() (string, error) {
	return "", errors.New("not implemented")
}
