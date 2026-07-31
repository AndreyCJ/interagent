package usecase

import (
	"testing"

	"interagent/internal/port"
)

type mockSettingsStore struct {
	settings port.AppSettings
}

func newMockSettingsStore() *mockSettingsStore {
	return &mockSettingsStore{
		settings: port.AppSettings{
			Theme:    "dark",
			Language: "en",
			Shortcuts: []port.Shortcut{
				{ID: "mic_toggle", Label: "Toggle Mic", Keys: []string{"Control", "Shift", "M"}, Enabled: true},
			},
		},
	}
}

func (m *mockSettingsStore) CreateSession(s port.Session) error { return nil }
func (m *mockSettingsStore) GetSession(id string) (port.Session, error) {
	return port.Session{}, nil
}
func (m *mockSettingsStore) UpdateSession(s port.Session) error     { return nil }
func (m *mockSettingsStore) DeleteSession(id string) error          { return nil }
func (m *mockSettingsStore) GetAgents() ([]port.AgentConfig, error) { return nil, nil }
func (m *mockSettingsStore) SaveAgent(cfg port.AgentConfig) error   { return nil }
func (m *mockSettingsStore) DeleteAgent(id string) error            { return nil }
func (m *mockSettingsStore) GetSettings() (port.AppSettings, error) { return m.settings, nil }
func (m *mockSettingsStore) SaveSettings(s port.AppSettings) error {
	m.settings = s
	return nil
}

func TestSettings_Get_ReturnsDefaults(t *testing.T) {
	store := newMockSettingsStore()
	s := NewSettings(store)

	settings, err := s.Get()
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if settings.Theme != "dark" && settings.Theme != "light" && settings.Theme != "transparent" {
		t.Errorf("unexpected theme value: %q", settings.Theme)
	}
}

func TestSettings_Save_PersistsChanges(t *testing.T) {
	store := newMockSettingsStore()
	s := NewSettings(store)

	updated := port.AppSettings{
		Theme:    "light",
		Language: "ru",
	}

	err := s.Save(updated)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	got, _ := s.Get()
	if got.Theme != "light" {
		t.Errorf("expected theme 'light', got %q", got.Theme)
	}
	if got.Language != "ru" {
		t.Errorf("expected language 'ru', got %q", got.Language)
	}
}

func TestSettings_UpdateShortcut_ChangesKeys(t *testing.T) {
	store := newMockSettingsStore()
	settings := NewSettings(store)

	newKeys := []string{"Alt", "M"}
	err := settings.UpdateShortcut("mic_toggle", newKeys)
	if err != nil {
		t.Fatalf("UpdateShortcut() returned error: %v", err)
	}

	got, _ := settings.Get()
	for _, sh := range got.Shortcuts {
		if sh.ID == "mic_toggle" {
			if len(sh.Keys) != 2 || sh.Keys[0] != "Alt" {
				t.Errorf("shortcut keys not updated: got %v", sh.Keys)
			}
			return
		}
	}
	t.Error("UpdateShortcut() did not update the shortcut")
}
