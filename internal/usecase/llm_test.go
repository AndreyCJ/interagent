package usecase

import (
	"errors"
	"sync"
	"testing"

	"interagent/internal/port"
)

type mockLLM struct {
	response   string
	err        error
	cancelled  bool
	gotInput   port.LLMInput
	gotHist    []port.Message
	cancelHook func()
}

func (m *mockLLM) Complete(input port.LLMInput, history []port.Message, onToken func(string)) (string, error) {
	m.gotInput = input
	m.gotHist = history
	if m.cancelHook != nil {
		m.cancelHook()
	}
	if onToken != nil && m.response != "" {
		onToken(m.response)
	}
	return m.response, m.err
}

func (m *mockLLM) Cancel() error { m.cancelled = true; return nil }

type mockSessionWriter struct {
	roles []string
	texts []string
	err   error
}

func (m *mockSessionWriter) AppendMessage(role, text string) error {
	if m.err != nil {
		return m.err
	}
	m.roles = append(m.roles, role)
	m.texts = append(m.texts, text)
	return nil
}

type mockFactory struct {
	engine port.LLM
	err    error
}

func (m mockFactory) ForAgent(cfg port.AgentConfig) (port.LLM, error) {
	return m.engine, m.err
}

type mockAgentProvider struct {
	cfg port.AgentConfig
	err error
}

func (m mockAgentProvider) GetActive() (port.AgentConfig, error) { return m.cfg, m.err }

type cancelOnGetActiveAgent struct {
	cfg  port.AgentConfig
	hook func()
}

func (m cancelOnGetActiveAgent) GetActive() (port.AgentConfig, error) {
	m.hook()
	return m.cfg, nil
}

type mockSessionReader struct {
	session port.Session
	err     error
}

func (m mockSessionReader) GetCurrent() (port.Session, error) { return m.session, m.err }

type mockSessionLive struct {
	history []port.Message
	err     error
}

func (m *mockSessionLive) AppendMessage(role, text string) error {
	if m.err != nil {
		return m.err
	}
	m.history = append(m.history, port.Message{Role: role, Text: text})
	return nil
}

func (m *mockSessionLive) GetCurrent() (port.Session, error) {
	if m.err != nil {
		return port.Session{}, m.err
	}
	return port.Session{ID: "s1", ChatHistory: m.history}, nil
}

func llmPayload(payload any) string {
	return payload.(map[string]string)["text"]
}

func errorPayload(payload any) string {
	return payload.(map[string]string)["error"]
}

func newTestLLM(engine port.LLM, session *mockSessionWriter, reader mockSessionReader, agent mockAgentProvider) *LLM {
	if engine == nil {
		engine = &mockLLM{response: "answer"}
	}
	return NewLLM(newMockEvents(), session, reader, agent, mockFactory{engine: engine})
}

func TestLLM_Generate_EmptyText_ReturnsError(t *testing.T) {
	llm := newTestLLM(nil, &mockSessionWriter{}, mockSessionReader{}, mockAgentProvider{})
	if _, err := llm.Generate(port.LLMInput{}, "user"); err == nil {
		t.Error("Generate with empty text should return error")
	}
}

func TestLLM_Generate_InvalidRole_ReturnsError(t *testing.T) {
	llm := newTestLLM(nil, &mockSessionWriter{}, mockSessionReader{}, mockAgentProvider{})
	if _, err := llm.Generate(port.LLMInput{Text: "hi"}, "system"); err == nil {
		t.Error("Generate with invalid role should return error")
	}
}

func TestLLM_Generate_NoActiveAgent_ReturnsError(t *testing.T) {
	llm := newTestLLM(nil, &mockSessionWriter{}, mockSessionReader{}, mockAgentProvider{})
	if _, err := llm.Generate(port.LLMInput{Text: "hi"}, "user"); err == nil {
		t.Fatal("Generate without active agent should error")
	}
}

func TestLLM_Generate_StreamsPartials_ReturnsAnswer(t *testing.T) {
	engine := &mockLLM{response: "Hello world"}
	events := newMockEvents()
	session := &mockSessionWriter{}
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "local", Model: "qwen3"}}
	llm := NewLLM(events, session, mockSessionReader{}, agent, mockFactory{engine: engine})

	answer, err := llm.Generate(port.LLMInput{Text: "greet"}, "user")
	if err != nil {
		t.Fatalf("Generate() returned error: %v", err)
	}
	if answer != "Hello world" {
		t.Errorf("answer = %q, want %q", answer, "Hello world")
	}
	if events.count("llm:partial") != 1 || llmPayload(events.payload("llm:partial", 0)) != "Hello world" {
		t.Errorf("expected 1 llm:partial with full text, got %d events", events.count("llm:partial"))
	}
	if events.count("llm:started") != 1 || events.count("llm:response") != 1 {
		t.Errorf("expected started+response, got started=%d response=%d", events.count("llm:started"), events.count("llm:response"))
	}
	if len(session.roles) != 2 || session.roles[0] != "user" || session.roles[1] != "assistant" {
		t.Errorf("roles = %v, want [user assistant]", session.roles)
	}
}

func TestLLM_Generate_Language_AppendsInstruction(t *testing.T) {
	engine := &mockLLM{response: "réponse"}
	events := newMockEvents()
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "local"}}
	llm := NewLLM(events, &mockSessionWriter{}, mockSessionReader{}, agent, mockFactory{engine: engine})

	if _, err := llm.Generate(port.LLMInput{Text: "Question", Language: "fr"}, "user"); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if got := engine.gotInput.Text; got != "Question\nAnswer in the speaker's language (detected: fr)." {
		t.Errorf("engine text = %q", got)
	}
}

func TestLLM_Generate_NoLanguage_NoInstruction(t *testing.T) {
	engine := &mockLLM{response: "answer"}
	events := newMockEvents()
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "local"}}
	llm := NewLLM(events, &mockSessionWriter{}, mockSessionReader{}, agent, mockFactory{engine: engine})

	if _, err := llm.Generate(port.LLMInput{Text: "Question"}, "user"); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if engine.gotInput.Text != "Question" {
		t.Errorf("engine text = %q, want unchanged", engine.gotInput.Text)
	}
}

func TestLLM_Generate_PassesHistoryFromReader(t *testing.T) {
	engine := &mockLLM{response: "ok"}
	hist := []port.Message{{Role: "user", Text: "earlier", Timestamp: 1}}
	reader := mockSessionReader{session: port.Session{ID: "s1", ChatHistory: hist}}
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "openai-compatible"}}
	llm := NewLLM(newMockEvents(), &mockSessionWriter{}, reader, agent, mockFactory{engine: engine})

	if _, err := llm.Generate(port.LLMInput{Text: "now"}, "user"); err != nil {
		t.Fatalf("Generate() returned error: %v", err)
	}
	if len(engine.gotHist) != 1 || engine.gotHist[0].Text != "earlier" {
		t.Errorf("history passed to engine = %+v, want earlier message", engine.gotHist)
	}
	if engine.gotInput.Language != "" {
		t.Errorf("Language = %q, want empty", engine.gotInput.Language)
	}
}

func TestLLM_Generate_PassesLanguage(t *testing.T) {
	engine := &mockLLM{response: "ok"}
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "local"}}
	llm := NewLLM(newMockEvents(), &mockSessionWriter{}, mockSessionReader{}, agent, mockFactory{engine: engine})

	if _, err := llm.Generate(port.LLMInput{Text: "question", Language: "ru"}, "interviewer"); err != nil {
		t.Fatalf("Generate() returned error: %v", err)
	}
	if engine.gotInput.Language != "ru" {
		t.Errorf("Language = %q, want ru", engine.gotInput.Language)
	}
}

func TestLLM_Generate_WriterReaderShared_NoDuplicatePrompt(t *testing.T) {
	engine := &mockLLM{response: "answer"}
	store := &mockSessionLive{
		history: []port.Message{{Role: "user", Text: "earlier", Timestamp: 1}},
	}
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "local"}}
	llm := NewLLM(newMockEvents(), store, store, agent, mockFactory{engine: engine})

	if _, err := llm.Generate(port.LLMInput{Text: "current"}, "user"); err != nil {
		t.Fatalf("Generate() returned error: %v", err)
	}
	if len(engine.gotHist) != 1 || engine.gotHist[0].Text != "earlier" {
		t.Fatalf("history passed to engine = %+v, want only the earlier message", engine.gotHist)
	}
	if engine.gotHist[len(engine.gotHist)-1].Text == "current" {
		t.Error("history passed to engine ends with the current prompt; prompt duplicated")
	}
}

func TestLLM_Generate_CancelledOnSuccess_NoResponse(t *testing.T) {
	engine := &mockLLM{response: "partial-answer"}
	events := newMockEvents()
	session := &mockSessionWriter{}
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "local"}}
	var llm *LLM
	engine.cancelHook = func() { _ = llm.Cancel() }
	llm = NewLLM(events, session, mockSessionReader{}, agent, mockFactory{engine: engine})

	answer, err := llm.Generate(port.LLMInput{Text: "q"}, "user")
	if err != nil {
		t.Fatalf("Generate() after cancel should not return an error, got: %v", err)
	}
	if answer != "" {
		t.Errorf("answer = %q, want empty after cancel", answer)
	}
	if events.count("llm:response") != 0 {
		t.Errorf("llm:response must not fire after cancel, got %d", events.count("llm:response"))
	}
	if events.count("llm:cancelled") != 1 {
		t.Errorf("expected 1 llm:cancelled event, got %d", events.count("llm:cancelled"))
	}
	if len(session.roles) != 1 || session.roles[0] != "user" {
		t.Errorf("assistant must not be appended after cancel, roles = %v", session.roles)
	}
}

func TestLLM_Generate_NoAgent_EmitsErrorEvent(t *testing.T) {
	events := newMockEvents()
	llm := NewLLM(events, &mockSessionWriter{}, mockSessionReader{}, mockAgentProvider{}, mockFactory{})

	if _, err := llm.Generate(port.LLMInput{Text: "hi"}, "user"); err == nil {
		t.Fatal("Generate without active agent should return error")
	}
	if events.count("llm:error") != 1 || errorPayload(events.payload("llm:error", 0)) != "no active agent" {
		t.Errorf("expected llm:error with 'no active agent', got %d events", events.count("llm:error"))
	}
}

func TestLLM_Generate_FactoryError_EmitsErrorEvent(t *testing.T) {
	events := newMockEvents()
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "openai-compatible"}}
	factory := mockFactory{err: errors.New("decrypt api key: boom")}
	llm := NewLLM(events, &mockSessionWriter{}, mockSessionReader{}, agent, factory)

	if _, err := llm.Generate(port.LLMInput{Text: "hi"}, "user"); err == nil {
		t.Fatal("Generate should propagate factory error")
	}
	if events.count("llm:error") != 1 || errorPayload(events.payload("llm:error", 0)) != "decrypt api key: boom" {
		t.Errorf("expected llm:error with factory message, got %d events", events.count("llm:error"))
	}
}

func TestLLM_Generate_EngineError_EmitsError_NoAssistant(t *testing.T) {
	engine := &mockLLM{err: errors.New("network timeout")}
	events := newMockEvents()
	session := &mockSessionWriter{}
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "local"}}
	llm := NewLLM(events, session, mockSessionReader{}, agent, mockFactory{engine: engine})

	if _, err := llm.Generate(port.LLMInput{Text: "q"}, "user"); err == nil {
		t.Fatal("Generate() should return engine error")
	}
	if events.count("llm:error") != 1 || errorPayload(events.payload("llm:error", 0)) != "network timeout" {
		t.Errorf("expected llm:error with timeout message")
	}
	if len(session.roles) != 1 || session.roles[0] != "user" {
		t.Error("assistant must not be appended on engine error")
	}
}

func TestLLM_Generate_CancelBeforeEngineSet_EmitsCancelled(t *testing.T) {
	events := newMockEvents()
	session := &mockSessionWriter{}
	engine := &mockLLM{response: "answer"}
	agent := cancelOnGetActiveAgent{cfg: port.AgentConfig{ID: "a1", Provider: "local"}}
	var llm *LLM
	agent.hook = func() { _ = llm.Cancel() }
	llm = NewLLM(events, session, mockSessionReader{}, agent, mockFactory{engine: engine})

	answer, err := llm.Generate(port.LLMInput{Text: "q"}, "user")
	if err != nil {
		t.Fatalf("Generate() after cancel should not return an error, got: %v", err)
	}
	if answer != "" {
		t.Errorf("answer = %q, want empty after cancel", answer)
	}
	if events.count("llm:cancelled") != 1 {
		t.Errorf("expected 1 llm:cancelled event, got %d", events.count("llm:cancelled"))
	}
	if events.count("llm:response") != 0 {
		t.Errorf("llm:response must not fire after cancel, got %d", events.count("llm:response"))
	}
	if len(session.roles) != 1 || session.roles[0] != "user" {
		t.Errorf("assistant must not be appended after cancel, roles = %v", session.roles)
	}
}

func TestLLM_Generate_AppendMessageError_EmitsErrorEvent(t *testing.T) {
	events := newMockEvents()
	session := &mockSessionWriter{err: errors.New("write failed")}
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "local"}}
	llm := NewLLM(events, session, mockSessionReader{}, agent, mockFactory{engine: &mockLLM{response: "answer"}})

	answer, err := llm.Generate(port.LLMInput{Text: "q"}, "user")
	if err == nil || err.Error() != "write failed" {
		t.Fatalf("Generate() error = %v, want 'write failed'", err)
	}
	if answer != "" {
		t.Errorf("answer = %q, want empty on writer error", answer)
	}
	if events.count("llm:error") != 1 || errorPayload(events.payload("llm:error", 0)) != "write failed" {
		t.Errorf("expected 1 llm:error with 'write failed', got %d events", events.count("llm:error"))
	}
}

func TestLLM_Cancel_CancelsCurrentEngine(t *testing.T) {
	engine := &mockLLM{response: "answer"}
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "local"}}
	llm := NewLLM(newMockEvents(), &mockSessionWriter{}, mockSessionReader{}, agent, mockFactory{engine: engine})

	if _, err := llm.Generate(port.LLMInput{Text: "q"}, "user"); err != nil {
		t.Fatalf("Generate() returned error: %v", err)
	}
	if err := llm.Cancel(); err != nil {
		t.Fatalf("Cancel() returned error: %v", err)
	}
	if !engine.cancelled {
		t.Error("Cancel() did not cancel the built engine")
	}
}

func TestLLM_Cancel_EmitsCancelled(t *testing.T) {
	engine := &mockLLM{response: "answer"}
	events := newMockEvents()
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "local"}}
	llm := NewLLM(events, &mockSessionWriter{}, mockSessionReader{}, agent, mockFactory{engine: engine})

	if _, err := llm.Generate(port.LLMInput{Text: "q"}, "user"); err != nil {
		t.Fatalf("Generate() returned error: %v", err)
	}
	if err := llm.Cancel(); err != nil {
		t.Fatalf("Cancel() returned error: %v", err)
	}
	if events.count("llm:cancelled") != 1 {
		t.Errorf("expected 1 llm:cancelled event, got %d", events.count("llm:cancelled"))
	}
}

func TestLLM_Generate_ErrorAfterCancel_NoErrorEvent(t *testing.T) {
	engine := &mockLLM{err: errors.New("stream aborted")}
	events := newMockEvents()
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "local"}}
	var llm *LLM
	engine.cancelHook = func() { _ = llm.Cancel() }
	llm = NewLLM(events, &mockSessionWriter{}, mockSessionReader{}, agent, mockFactory{engine: engine})

	if _, err := llm.Generate(port.LLMInput{Text: "q"}, "user"); err != nil {
		t.Fatalf("Generate() after cancel should not return an error, got: %v", err)
	}
	if events.count("llm:error") != 0 {
		t.Errorf("llm:error must not fire after cancel, got %d", events.count("llm:error"))
	}
	if events.count("llm:cancelled") != 1 {
		t.Errorf("expected 1 llm:cancelled event, got %d", events.count("llm:cancelled"))
	}
}

func TestLLM_ListLocalModels_OnlyForLocalProvider(t *testing.T) {
	agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "openai-compatible"}}
	llm := NewLLM(newMockEvents(), &mockSessionWriter{}, mockSessionReader{}, agent, mockFactory{engine: &mockLLM{}})
	models, err := llm.ListLocalModels()
	if err != nil {
		t.Fatalf("ListLocalModels() returned error: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected no models for non-local provider, got %v", models)
	}
}

type concurrentEvents struct {
	mu  sync.Mutex
	set map[string][]any
}

func newConcurrentEvents() *concurrentEvents {
	return &concurrentEvents{set: make(map[string][]any)}
}

func (e *concurrentEvents) Emit(name string, payload any) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.set[name] = append(e.set[name], payload)
	return nil
}

func TestLLM_Cancel_ConcurrentWithGenerate_Race(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		engine := &mockLLM{response: "answer"}
		agent := mockAgentProvider{cfg: port.AgentConfig{ID: "a1", Provider: "local"}}
		llm := NewLLM(newConcurrentEvents(), &mockSessionWriter{}, mockSessionReader{}, agent, mockFactory{engine: engine})

		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = llm.Generate(port.LLMInput{Text: "q"}, "user")
		}()
		go func() {
			defer wg.Done()
			_ = llm.Cancel()
		}()
		wg.Wait()
	}
}
