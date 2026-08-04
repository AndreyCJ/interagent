package storage

import (
	"testing"

	"interagent/internal/port"
)

const testDSN = "file::memory:?cache=shared"

type testCrypto struct{}

func (testCrypto) Encrypt(plaintext string) (string, error) { return "enc:" + plaintext, nil }
func (testCrypto) Decrypt(ciphertext string) (string, error) {
	return "dec:" + ciphertext, nil
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(testDSN, testCrypto{})
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStorage_CreateAndGetSession(t *testing.T) {
	s := newTestStore(t)
	sess := port.Session{
		ID:          "s1",
		ChatHistory: []port.Message{{Role: "user", Text: "hello", Timestamp: 100}},
		StartedAt:   200,
	}

	if err := s.CreateSession(sess); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	got, err := s.GetSession("s1")
	if err != nil {
		t.Fatalf("GetSession() returned error: %v", err)
	}
	if got.ID != "s1" {
		t.Errorf("session ID = %q, want s1", got.ID)
	}
	if got.StartedAt != 200 {
		t.Errorf("StartedAt = %d, want 200", got.StartedAt)
	}
	if len(got.ChatHistory) != 1 {
		t.Fatalf("ChatHistory length = %d, want 1", len(got.ChatHistory))
	}
	if got.ChatHistory[0].Role != "user" || got.ChatHistory[0].Text != "hello" {
		t.Errorf("message = %+v, want user/hello", got.ChatHistory[0])
	}
}

func TestStorage_GetSession_NotFound_ReturnsError(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.GetSession("missing"); err == nil {
		t.Error("GetSession() for missing id should return error")
	}
}

func TestStorage_UpdateSession_ReplacesHistory(t *testing.T) {
	s := newTestStore(t)
	initial := port.Session{
		ID:          "s1",
		ChatHistory: []port.Message{{Role: "user", Text: "first", Timestamp: 1}},
		StartedAt:   200,
	}
	if err := s.CreateSession(initial); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	updated := port.Session{
		ID: "s1",
		ChatHistory: []port.Message{
			{Role: "user", Text: "first", Timestamp: 1},
			{Role: "assistant", Text: "second", Timestamp: 2},
		},
		StartedAt: 200,
	}
	if err := s.UpdateSession(updated); err != nil {
		t.Fatalf("UpdateSession() returned error: %v", err)
	}

	got, err := s.GetSession("s1")
	if err != nil {
		t.Fatalf("GetSession() returned error: %v", err)
	}
	if len(got.ChatHistory) != 2 {
		t.Fatalf("ChatHistory length = %d, want 2", len(got.ChatHistory))
	}
	if got.ChatHistory[1].Role != "assistant" || got.ChatHistory[1].Text != "second" {
		t.Errorf("second message = %+v, want assistant/second", got.ChatHistory[1])
	}
}

func TestStorage_DeleteSession_RemovesSessionAndMessages(t *testing.T) {
	s := newTestStore(t)
	sess := port.Session{
		ID:          "s1",
		ChatHistory: []port.Message{{Role: "user", Text: "hello", Timestamp: 100}},
		StartedAt:   200,
	}
	if err := s.CreateSession(sess); err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}

	if err := s.DeleteSession("s1"); err != nil {
		t.Fatalf("DeleteSession() returned error: %v", err)
	}
	if _, err := s.GetSession("s1"); err == nil {
		t.Error("session should be gone after DeleteSession")
	}
}

func TestStorage_GetSettings_ReturnsSeededDefaults(t *testing.T) {
	s := newTestStore(t)

	settings, err := s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() returned error: %v", err)
	}
	if settings.Theme != "transparent" {
		t.Errorf("default Theme = %q, want transparent", settings.Theme)
	}
	if len(settings.Shortcuts) < 2 {
		t.Fatalf("expected at least 2 default shortcuts, got %d", len(settings.Shortcuts))
	}
	ids := map[string]port.Shortcut{}
	for _, sc := range settings.Shortcuts {
		ids[sc.ID] = sc
	}
	if sc, ok := ids["overlay_toggle"]; !ok || !sc.Enabled {
		t.Errorf("default shortcut overlay_toggle should exist and be enabled: %+v", sc)
	}
	if sc, ok := ids["overlay_mode"]; !ok || !sc.Enabled {
		t.Errorf("default shortcut overlay_mode should exist and be enabled: %+v", sc)
	}
}

func TestStorage_SaveAndGetSettings(t *testing.T) {
	s := newTestStore(t)

	custom := port.AppSettings{
		Theme:              "dark",
		Language:           "ru",
		AutoStartListening: true,
		Shortcuts: []port.Shortcut{
			{ID: "overlay_toggle", Label: "Show/Hide", Keys: []string{"cmd", "shift", "h"}, Enabled: true},
		},
	}
	if err := s.SaveSettings(custom); err != nil {
		t.Fatalf("SaveSettings() returned error: %v", err)
	}

	got, err := s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings() returned error: %v", err)
	}
	if got.Theme != "dark" || got.Language != "ru" || !got.AutoStartListening {
		t.Errorf("saved settings mismatch: %+v", got)
	}
	if len(got.Shortcuts) != 1 || got.Shortcuts[0].ID != "overlay_toggle" {
		t.Errorf("shortcuts mismatch: %+v", got.Shortcuts)
	}
}

func TestStorage_Agents_CRUD(t *testing.T) {
	s := newTestStore(t)

	agent := port.AgentConfig{
		ID:           "a1",
		Name:         "Local",
		Provider:     "local",
		Model:        "model-3b",
		BaseURL:      "",
		APIKey:       "sk-secret",
		SystemPrompt: "You help",
		Temperature:  0.3,
	}
	if err := s.SaveAgent(agent); err != nil {
		t.Fatalf("SaveAgent() returned error: %v", err)
	}

	var storedKey string
	err := s.db.QueryRow(`SELECT api_key FROM agents WHERE id = 'a1'`).Scan(&storedKey)
	if err != nil {
		t.Fatalf("read stored key: %v", err)
	}
	if storedKey != "enc:sk-secret" {
		t.Errorf("stored api_key = %q, want enc:sk-secret (encrypted)", storedKey)
	}

	agents, err := s.GetAgents()
	if err != nil {
		t.Fatalf("GetAgents() returned error: %v", err)
	}
	var found *port.AgentConfig
	for i := range agents {
		if agents[i].ID == "a1" {
			found = &agents[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("agent a1 not found in %d agents", len(agents))
	}
	if found.Model != "model-3b" {
		t.Errorf("agent mismatch: %+v", *found)
	}

	if err := s.DeleteAgent("a1"); err != nil {
		t.Fatalf("DeleteAgent() returned error: %v", err)
	}
	agents, err = s.GetAgents()
	if err != nil {
		t.Fatalf("GetAgents() returned error: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("agents count after delete = %d, want 1", len(agents))
	}
}

func TestStorage_Seed_CreatesDefaultLocalAgent(t *testing.T) {
	s := newTestStore(t)
	agents, err := s.GetAgents()
	if err != nil {
		t.Fatalf("GetAgents() returned error: %v", err)
	}
	if len(agents) == 0 {
		t.Fatal("expected a seeded default local agent")
	}
	if agents[0].Provider != "local" || agents[0].Model == "" {
		t.Errorf("default agent mismatch: %+v", agents[0])
	}
}
