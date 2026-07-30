package main

type ScreenshotBind struct {
	usecase interface {
		CaptureFull() error
		CaptureRegion() error
		CaptureAndOCR() (string, error)
	}
}

func NewScreenshotBind(u interface {
	CaptureFull() error
	CaptureRegion() error
	CaptureAndOCR() (string, error)
}) *ScreenshotBind {
	return &ScreenshotBind{usecase: u}
}

func (b *ScreenshotBind) CaptureFullScreen() error {
	return b.usecase.CaptureFull()
}

func (b *ScreenshotBind) CaptureRegion() error {
	return b.usecase.CaptureRegion()
}

func (b *ScreenshotBind) CaptureAndOCR(region string) (string, error) {
	return b.usecase.CaptureAndOCR()
}
