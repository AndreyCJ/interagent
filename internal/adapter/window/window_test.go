package window

import (
	"testing"

	"interagent/internal/port"
)

func TestNew_DefaultsToClickThrough(t *testing.T) {
	mode, err := New().GetMode()
	if err != nil {
		t.Fatalf("GetMode() error: %v", err)
	}
	if mode != port.OverlayModeClickThrough {
		t.Errorf("mode = %q, want %q", mode, port.OverlayModeClickThrough)
	}
}

func TestNew_DefaultsToVisible(t *testing.T) {
	if o := New(); !o.visible {
		t.Error("overlay should start visible")
	}
}

func TestShowHideToggle_NilCtx_NoPanicNoStateChange(t *testing.T) {
	o := New()
	if err := o.Show(); err != nil {
		t.Fatalf("Show() error: %v", err)
	}
	if err := o.Hide(); err != nil {
		t.Fatalf("Hide() error: %v", err)
	}
	if err := o.Toggle(); err != nil {
		t.Fatalf("Toggle() error: %v", err)
	}
	if !o.visible {
		t.Error("visible state must not change without a Wails context")
	}
	mode, _ := o.GetMode()
	if mode != port.OverlayModeClickThrough {
		t.Errorf("mode changed unexpectedly: %q", mode)
	}
}

func TestSetMode_StoresMode(t *testing.T) {
	o := New()
	if err := o.SetMode(port.OverlayModeInteractive); err != nil {
		t.Fatalf("SetMode(interactive) error: %v", err)
	}
	mode, _ := o.GetMode()
	if mode != port.OverlayModeInteractive {
		t.Errorf("mode = %q, want %q", mode, port.OverlayModeInteractive)
	}

	if err := o.SetMode(port.OverlayModeClickThrough); err != nil {
		t.Fatalf("SetMode(click-through) error: %v", err)
	}
	mode, _ = o.GetMode()
	if mode != port.OverlayModeClickThrough {
		t.Errorf("mode = %q, want %q", mode, port.OverlayModeClickThrough)
	}
}
