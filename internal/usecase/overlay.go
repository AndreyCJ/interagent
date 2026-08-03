package usecase

import (
	"errors"

	"interagent/internal/port"
)

type Overlay struct {
	ov     port.Overlay
	events port.Events
}

func NewOverlay(ov port.Overlay, events port.Events) *Overlay {
	return &Overlay{ov: ov, events: events}
}

func (o *Overlay) Show() error   { return o.ov.Show() }
func (o *Overlay) Hide() error   { return o.ov.Hide() }
func (o *Overlay) Toggle() error { return o.ov.Toggle() }

func (o *Overlay) SetMode(mode port.OverlayMode) error {
	if mode != port.OverlayModeClickThrough && mode != port.OverlayModeInteractive {
		return errors.New("invalid mode")
	}
	if err := o.ov.SetMode(mode); err != nil {
		return err
	}
	return o.events.Emit("overlay:mode", map[string]string{"mode": string(mode)})
}

func (o *Overlay) GetMode() (port.OverlayMode, error) {
	return o.ov.GetMode()
}

func (o *Overlay) ToggleMode() error {
	cur, err := o.ov.GetMode()
	if err != nil {
		return err
	}
	next := port.OverlayModeInteractive
	if cur == port.OverlayModeInteractive {
		next = port.OverlayModeClickThrough
	}
	return o.SetMode(next)
}
