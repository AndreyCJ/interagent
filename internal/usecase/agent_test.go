package usecase

import (
	"testing"

	"interagent/internal/port"
)

type mockAgentStore struct {
	agents   map[string]port.AgentConfig
	activeID string
}

func newMockAgentStore() *mockAgentStore {
	return &mockAgentStore{
		agents:   make(map[string]port.AgentConfig),
		activeID: "",
	}
}

func (m *mockAgentStore) CreateSession(s port.Session) error { return nil }
func (m *mockAgentStore) GetSession(id string) (port.Session, error) {
	return port.Session{}, nil
}
func (m *mockAgentStore) UpdateSession(s port.Session) error { return nil }
func (m *mockAgentStore) DeleteSession(id string) error      { return nil }

func (m *mockAgentStore) GetAgents() ([]port.AgentConfig, error) {
	var result []port.AgentConfig
	for _, a := range m.agents {
		result = append(result, a)
	}
	return result, nil
}

func (m *mockAgentStore) SaveAgent(cfg port.AgentConfig) error {
	m.agents[cfg.ID] = cfg
	return nil
}

func (m *mockAgentStore) DeleteAgent(id string) error {
	delete(m.agents, id)
	return nil
}

func (m *mockAgentStore) GetSettings() (port.AppSettings, error) { return port.AppSettings{}, nil }
func (m *mockAgentStore) SaveSettings(s port.AppSettings) error  { return nil }

func TestAgent_List_ReturnsAllAgents(t *testing.T) {
	store := newMockAgentStore()
	a := NewAgent(store)

	agents, err := a.List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if agents == nil {
		t.Fatal("List() returned nil, expected empty slice")
	}
}

func TestAgent_SaveAndList(t *testing.T) {
	store := newMockAgentStore()
	a := NewAgent(store)

	cfg := port.AgentConfig{
		ID:           "agent-1",
		Name:         "Default",
		Model:        "llama3-8b",
		SystemPrompt: "You are a helpful assistant",
		Temperature:  0.7,
	}

	saved, err := a.Save(cfg)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if saved.ID != cfg.ID {
		t.Errorf("Save() returned agent with different ID: got %q, want %q", saved.ID, cfg.ID)
	}

	agents, err := a.List()
	if err != nil {
		t.Fatalf("List() returned error: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}
}

func TestAgent_Delete_RemovesAgent(t *testing.T) {
	store := newMockAgentStore()
	a := NewAgent(store)

	cfg := port.AgentConfig{ID: "agent-1", Name: "Test"}
	a.Save(cfg)

	err := a.Delete("agent-1")
	if err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}

	agents, _ := a.List()
	if len(agents) != 0 {
		t.Errorf("after Delete(), expected 0 agents, got %d", len(agents))
	}
}

func TestAgent_SetActive_SwitchesAgent(t *testing.T) {
	store := newMockAgentStore()
	a := NewAgent(store)

	cfg := port.AgentConfig{ID: "agent-2", Name: "Code Assistant"}
	a.Save(cfg)

	err := a.SetActive("agent-2")
	if err != nil {
		t.Fatalf("SetActive() returned error: %v", err)
	}

	active, err := a.GetActive()
	if err != nil {
		t.Fatalf("GetActive() returned error: %v", err)
	}
	if active.ID != "agent-2" {
		t.Errorf("expected active agent ID 'agent-2', got %q", active.ID)
	}
}
