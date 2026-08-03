package usecase

import (
	"errors"

	"interagent/internal/port"
)

type Hotkeys struct {
	adapter port.Hotkeys
	overlay *Overlay
}

func NewHotkeys(adapter port.Hotkeys, overlay *Overlay) *Hotkeys {
	return &Hotkeys{adapter: adapter, overlay: overlay}
}

func knownID(id string) bool {
	return id == "overlay_toggle" || id == "overlay_mode"
}

func (h *Hotkeys) RegisterShortcuts(shortcuts []port.Shortcut) error {
	for _, sc := range shortcuts {
		if err := h.RegisterShortcut(sc); err != nil {
			return err
		}
	}
	return nil
}

func (h *Hotkeys) RegisterShortcut(sc port.Shortcut) error {
	if !knownID(sc.ID) {
		return errors.New("unknown hotkey id: " + sc.ID)
	}
	if len(sc.Keys) == 0 {
		return errors.New("empty keys")
	}
	if !sc.Enabled {
		return nil
	}
	return h.adapter.Register(sc.ID, sc.Keys)
}

func (h *Hotkeys) Unregister(id string) error { return h.adapter.Unregister(id) }

func (h *Hotkeys) Handle(id string) error {
	switch id {
	case "overlay_toggle":
		return h.overlay.Toggle()
	case "overlay_mode":
		return h.overlay.ToggleMode()
	default:
		return errors.New("unknown hotkey id: " + id)
	}
}
