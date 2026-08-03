package window

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"interagent/internal/port"
)

// Overlay implements port.Overlay against the real native window. Visibility is
// controlled via the Wails runtime; the mode is applied to the platform window
// (click-through on macOS, stubbed elsewhere).
type Overlay struct {
	mu      sync.RWMutex
	ctx     context.Context
	mode    port.OverlayMode
	visible bool
}

func New() *Overlay {
	return &Overlay{mode: port.OverlayModeClickThrough, visible: true}
}

// SetContext must be called once the Wails context is available (OnStartup).
func (o *Overlay) SetContext(ctx context.Context) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.ctx = ctx
}

func (o *Overlay) Show() error {
	o.mu.RLock()
	ctx := o.ctx
	o.mu.RUnlock()
	if ctx == nil {
		return nil
	}
	runtime.WindowShow(ctx)
	o.mu.Lock()
	o.visible = true
	o.mu.Unlock()
	return nil
}

func (o *Overlay) Hide() error {
	o.mu.RLock()
	ctx := o.ctx
	o.mu.RUnlock()
	if ctx == nil {
		return nil
	}
	runtime.WindowHide(ctx)
	o.mu.Lock()
	o.visible = false
	o.mu.Unlock()
	return nil
}

func (o *Overlay) Toggle() error {
	o.mu.RLock()
	ctx := o.ctx
	visible := o.visible
	o.mu.RUnlock()
	if ctx == nil {
		return nil
	}
	if visible {
		return o.Hide()
	}
	return o.Show()
}

func (o *Overlay) SetMode(mode port.OverlayMode) error {
	o.mode = mode
	return applyMode(mode)
}

func (o *Overlay) GetMode() (port.OverlayMode, error) { return o.mode, nil }
