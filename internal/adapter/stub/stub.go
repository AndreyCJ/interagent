// Package stub provides no-op native adapters for the overlay, hotkey and
// permission ports. They exist so the app wiring and usecases work end-to-end;
// real macOS implementations are deferred to later stages (ADR-006, ADR-008).
package stub

import "interagent/internal/port"

// --- Overlay (no-op) ---

type Overlay struct{ mode port.OverlayMode }

func NewOverlay() *Overlay { return &Overlay{mode: port.OverlayModeClickThrough} }

func (o *Overlay) Show() error   { return nil }
func (o *Overlay) Hide() error   { return nil }
func (o *Overlay) Toggle() error { return nil }

func (o *Overlay) SetMode(mode port.OverlayMode) error {
	o.mode = mode
	return nil
}

func (o *Overlay) GetMode() (port.OverlayMode, error) { return o.mode, nil }

// --- Hotkeys (no-op) ---

type Hotkeys struct{}

func NewHotkeys() *Hotkeys { return &Hotkeys{} }

func (h *Hotkeys) Register(id string, keys []string) error { return nil }
func (h *Hotkeys) Unregister(id string) error              { return nil }

// --- Permissions (no-op: assume granted) ---

type Permissions struct{}

func NewPermissions() *Permissions { return &Permissions{} }

func (p *Permissions) Status(perm port.Permission) (bool, error) { return true, nil }
func (p *Permissions) Request(perm port.Permission) error        { return nil }
func (p *Permissions) OpenSettings(perm port.Permission) error   { return nil }
