package port

type OverlayMode string

const (
	OverlayModeClickThrough OverlayMode = "click-through"
	OverlayModeInteractive  OverlayMode = "interactive"
)

type Overlay interface {
	Show() error
	Hide() error
	Toggle() error
	SetMode(mode OverlayMode) error
	GetMode() (OverlayMode, error)
}
