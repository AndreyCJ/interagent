package main

import "interagent/internal/port"

type OverlayBind struct {
	usecase interface {
		Show() error
		Hide() error
		SetMode(mode port.OverlayMode) error
		GetMode() (port.OverlayMode, error)
	}
}

func NewOverlayBind(u interface {
	Show() error
	Hide() error
	SetMode(mode port.OverlayMode) error
	GetMode() (port.OverlayMode, error)
}) *OverlayBind {
	return &OverlayBind{usecase: u}
}

func (b *OverlayBind) ShowOverlay() error { return b.usecase.Show() }
func (b *OverlayBind) HideOverlay() error { return b.usecase.Hide() }
func (b *OverlayBind) SetOverlayMode(mode port.OverlayMode) error {
	return b.usecase.SetMode(mode)
}
func (b *OverlayBind) GetOverlayMode() (port.OverlayMode, error) {
	return b.usecase.GetMode()
}
