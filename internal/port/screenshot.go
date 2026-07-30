package port

type ScreenCapture interface {
	CaptureFull() ([]byte, error)
	CaptureRegion() ([]byte, error)
}

type OCR interface {
	ExtractText(image []byte) (string, error)
}
