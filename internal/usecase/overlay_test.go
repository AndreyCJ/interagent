package usecase

import (
	"errors"
	"testing"

	"interagent/internal/port"
)

type mockOverlay struct {
	mode       port.OverlayMode
	shown      bool
	hidden     bool
	toggled    bool
	setModeErr error
}

func (m *mockOverlay) Show() error {
	m.shown = true
	return nil
}

func (m *mockOverlay) Hide() error {
	m.hidden = true
	return nil
}

func (m *mockOverlay) Toggle() error {
	m.toggled = true
	return nil
}

func (m *mockOverlay) SetMode(mode port.OverlayMode) error {
	if m.setModeErr != nil {
		return m.setModeErr
	}
	m.mode = mode
	return nil
}

func (m *mockOverlay) GetMode() (port.OverlayMode, error) {
	return m.mode, nil
}

type mockEvents struct {
	emitted map[string][]any
}

func newMockEvents() *mockEvents {
	return &mockEvents{emitted: make(map[string][]any)}
}

func (m *mockEvents) Emit(name string, payload any) error {
	m.emitted[name] = append(m.emitted[name], payload)
	return nil
}

func (m *mockEvents) count(name string) int {
	return len(m.emitted[name])
}

func (m *mockEvents) payload(name string, i int) any {
	return m.emitted[name][i]
}

func modePayload(payload any) string {
	return payload.(map[string]string)["mode"]
}

func TestOverlay_Show_DelegatesToAdapter(t *testing.T) {
	ov := &mockOverlay{}
	u := NewOverlay(ov, newMockEvents())

	if err := u.Show(); err != nil {
		t.Fatalf("Show() returned error: %v", err)
	}
	if !ov.shown {
		t.Error("Show() did not call adapter Show")
	}
}

func TestOverlay_Hide_DelegatesToAdapter(t *testing.T) {
	ov := &mockOverlay{}
	u := NewOverlay(ov, newMockEvents())

	if err := u.Hide(); err != nil {
		t.Fatalf("Hide() returned error: %v", err)
	}
	if !ov.hidden {
		t.Error("Hide() did not call adapter Hide")
	}
}

func TestOverlay_Toggle_DelegatesToAdapter(t *testing.T) {
	ov := &mockOverlay{}
	u := NewOverlay(ov, newMockEvents())

	if err := u.Toggle(); err != nil {
		t.Fatalf("Toggle() returned error: %v", err)
	}
	if !ov.toggled {
		t.Error("Toggle() did not call adapter Toggle")
	}
}

func TestOverlay_SetMode_Interactive_SetsModeAndEmits(t *testing.T) {
	ov := &mockOverlay{}
	events := newMockEvents()
	u := NewOverlay(ov, events)

	err := u.SetMode(port.OverlayModeInteractive)
	if err != nil {
		t.Fatalf("SetMode() returned error: %v", err)
	}
	if ov.mode != port.OverlayModeInteractive {
		t.Errorf("adapter mode = %q, want %q", ov.mode, port.OverlayModeInteractive)
	}
	if events.count("overlay:mode") != 1 {
		t.Fatalf("expected 1 overlay:mode event, got %d", events.count("overlay:mode"))
	}
	if got := modePayload(events.payload("overlay:mode", 0)); got != "interactive" {
		t.Errorf("overlay:mode payload mode = %q, want %q", got, "interactive")
	}
}

func TestOverlay_SetMode_ClickThrough_SetsModeAndEmits(t *testing.T) {
	ov := &mockOverlay{}
	events := newMockEvents()
	u := NewOverlay(ov, events)

	err := u.SetMode(port.OverlayModeClickThrough)
	if err != nil {
		t.Fatalf("SetMode() returned error: %v", err)
	}
	if ov.mode != port.OverlayModeClickThrough {
		t.Errorf("adapter mode = %q, want %q", ov.mode, port.OverlayModeClickThrough)
	}
	if got := modePayload(events.payload("overlay:mode", 0)); got != "click-through" {
		t.Errorf("overlay:mode payload mode = %q, want %q", got, "click-through")
	}
}

func TestOverlay_SetMode_InvalidMode_ReturnsError(t *testing.T) {
	ov := &mockOverlay{}
	events := newMockEvents()
	u := NewOverlay(ov, events)

	err := u.SetMode(port.OverlayMode("bogus"))
	if err == nil {
		t.Fatal("SetMode() with invalid mode should return error")
	}
	if events.count("overlay:mode") != 0 {
		t.Error("SetMode() with invalid mode should not emit overlay:mode")
	}
}

func TestOverlay_SetMode_AdapterError_DoesNotEmit(t *testing.T) {
	ov := &mockOverlay{setModeErr: errors.New("native failed")}
	events := newMockEvents()
	u := NewOverlay(ov, events)

	err := u.SetMode(port.OverlayModeInteractive)
	if err == nil {
		t.Fatal("SetMode() should propagate adapter error")
	}
	if events.count("overlay:mode") != 0 {
		t.Error("SetMode() on adapter error should not emit overlay:mode")
	}
}

func TestOverlay_ToggleMode_FromClickThrough_FlipsToInteractive(t *testing.T) {
	ov := &mockOverlay{mode: port.OverlayModeClickThrough}
	events := newMockEvents()
	u := NewOverlay(ov, events)

	err := u.ToggleMode()
	if err != nil {
		t.Fatalf("ToggleMode() returned error: %v", err)
	}
	if ov.mode != port.OverlayModeInteractive {
		t.Errorf("after ToggleMode() mode = %q, want %q", ov.mode, port.OverlayModeInteractive)
	}
	if got := modePayload(events.payload("overlay:mode", 0)); got != "interactive" {
		t.Errorf("overlay:mode payload mode = %q, want %q", got, "interactive")
	}
}

func TestOverlay_ToggleMode_FromInteractive_FlipsToClickThrough(t *testing.T) {
	ov := &mockOverlay{mode: port.OverlayModeInteractive}
	events := newMockEvents()
	u := NewOverlay(ov, events)

	err := u.ToggleMode()
	if err != nil {
		t.Fatalf("ToggleMode() returned error: %v", err)
	}
	if ov.mode != port.OverlayModeClickThrough {
		t.Errorf("after ToggleMode() mode = %q, want %q", ov.mode, port.OverlayModeClickThrough)
	}
}

func TestOverlay_GetMode_ReturnsAdapterMode(t *testing.T) {
	ov := &mockOverlay{mode: port.OverlayModeClickThrough}
	u := NewOverlay(ov, newMockEvents())

	mode, err := u.GetMode()
	if err != nil {
		t.Fatalf("GetMode() returned error: %v", err)
	}
	if mode != port.OverlayModeClickThrough {
		t.Errorf("GetMode() = %q, want %q", mode, port.OverlayModeClickThrough)
	}
}
