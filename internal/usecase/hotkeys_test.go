package usecase

import (
	"testing"

	"interagent/internal/port"
)

type mockHotkeysAdapter struct {
	registered   map[string][]string
	unregistered []string
}

func newMockHotkeysAdapter() *mockHotkeysAdapter {
	return &mockHotkeysAdapter{registered: make(map[string][]string)}
}

func (m *mockHotkeysAdapter) Register(id string, keys []string) error {
	m.registered[id] = keys
	return nil
}

func (m *mockHotkeysAdapter) Unregister(id string) error {
	m.unregistered = append(m.unregistered, id)
	return nil
}

func newTestHotkeys() (*Hotkeys, *mockHotkeysAdapter, *mockOverlay, *mockEvents) {
	ov := &mockOverlay{mode: port.OverlayModeClickThrough}
	events := newMockEvents()
	overlayUseCase := NewOverlay(ov, events)
	adapter := newMockHotkeysAdapter()
	u := NewHotkeys(adapter, overlayUseCase)
	return u, adapter, ov, events
}

func TestHotkeys_RegisterShortcuts_RegistersOnlyEnabled(t *testing.T) {
	u, adapter, _, _ := newTestHotkeys()
	shortcuts := []port.Shortcut{
		{ID: "overlay_toggle", Keys: []string{"cmd", "shift", "i"}, Enabled: true},
		{ID: "overlay_mode", Keys: []string{"cmd", "shift", "space"}, Enabled: false},
	}

	if err := u.RegisterShortcuts(shortcuts); err != nil {
		t.Fatalf("RegisterShortcuts() returned error: %v", err)
	}
	if _, ok := adapter.registered["overlay_toggle"]; !ok {
		t.Error("enabled shortcut overlay_toggle was not registered")
	}
	if _, ok := adapter.registered["overlay_mode"]; ok {
		t.Error("disabled shortcut overlay_mode should not be registered")
	}
}

func TestHotkeys_RegisterShortcuts_UnknownID_ReturnsError(t *testing.T) {
	u, _, _, _ := newTestHotkeys()
	shortcuts := []port.Shortcut{
		{ID: "nope", Keys: []string{"cmd"}, Enabled: true},
	}

	if err := u.RegisterShortcuts(shortcuts); err == nil {
		t.Error("RegisterShortcuts() with unknown id should return error")
	}
}

func TestHotkeys_RegisterShortcuts_EmptyKeys_ReturnsError(t *testing.T) {
	u, _, _, _ := newTestHotkeys()
	shortcuts := []port.Shortcut{
		{ID: "overlay_toggle", Keys: []string{}, Enabled: true},
	}

	if err := u.RegisterShortcuts(shortcuts); err == nil {
		t.Error("RegisterShortcuts() with empty keys should return error")
	}
}

func TestHotkeys_RegisterShortcut_PassesKeysToAdapter(t *testing.T) {
	u, adapter, _, _ := newTestHotkeys()
	keys := []string{"cmd", "shift", "space"}

	if err := u.RegisterShortcut(port.Shortcut{ID: "overlay_mode", Keys: keys, Enabled: true}); err != nil {
		t.Fatalf("RegisterShortcut() returned error: %v", err)
	}
	got := adapter.registered["overlay_mode"]
	if len(got) != 3 || got[0] != "cmd" || got[2] != "space" {
		t.Errorf("registered keys = %v, want %v", got, keys)
	}
}

func TestHotkeys_Unregister_Delegates(t *testing.T) {
	u, adapter, _, _ := newTestHotkeys()

	if err := u.Unregister("overlay_toggle"); err != nil {
		t.Fatalf("Unregister() returned error: %v", err)
	}
	if len(adapter.unregistered) != 1 || adapter.unregistered[0] != "overlay_toggle" {
		t.Errorf("unregistered = %v, want [overlay_toggle]", adapter.unregistered)
	}
}

func TestHotkeys_Handle_OverlayToggle_TogglesVisibility(t *testing.T) {
	u, _, ov, _ := newTestHotkeys()

	if err := u.Handle("overlay_toggle"); err != nil {
		t.Fatalf("Handle() returned error: %v", err)
	}
	if !ov.toggled {
		t.Error("Handle(overlay_toggle) did not toggle overlay")
	}
}

func TestHotkeys_Handle_OverlayMode_TogglesModeAndEmits(t *testing.T) {
	u, _, ov, events := newTestHotkeys()

	if err := u.Handle("overlay_mode"); err != nil {
		t.Fatalf("Handle() returned error: %v", err)
	}
	if ov.mode != port.OverlayModeInteractive {
		t.Errorf("after Handle(overlay_mode) mode = %q, want %q", ov.mode, port.OverlayModeInteractive)
	}
	if events.count("overlay:mode") != 1 {
		t.Fatalf("expected 1 overlay:mode event, got %d", events.count("overlay:mode"))
	}
	if got := modePayload(events.payload("overlay:mode", 0)); got != "interactive" {
		t.Errorf("overlay:mode payload mode = %q, want %q", got, "interactive")
	}
}

func TestHotkeys_Handle_UnknownID_ReturnsError(t *testing.T) {
	u, _, _, _ := newTestHotkeys()

	if err := u.Handle("bogus"); err == nil {
		t.Error("Handle() with unknown id should return error")
	}
}
