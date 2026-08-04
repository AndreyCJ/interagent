package usecase

import (
	"errors"
	"sync"

	"interagent/internal/port"
)

type sessionWriter interface {
	AppendMessage(role, text string) error
}

type sessionReader interface {
	GetCurrent() (port.Session, error)
}

type agentProvider interface {
	GetActive() (port.AgentConfig, error)
}

type llmFactory interface {
	ForAgent(cfg port.AgentConfig) (port.LLM, error)
}

// LLMFactory lets app.go provide the provider→adapter mapping as a func literal.
type LLMFactory func(cfg port.AgentConfig) (port.LLM, error)

func (f LLMFactory) ForAgent(cfg port.AgentConfig) (port.LLM, error) { return f(cfg) }

type LLM struct {
	events  port.Events
	writer  sessionWriter
	reader  sessionReader
	agents  agentProvider
	factory llmFactory
	current port.LLM

	mu        sync.Mutex
	cancelled bool
}

func NewLLM(events port.Events, writer sessionWriter, reader sessionReader, agents agentProvider, factory llmFactory) *LLM {
	return &LLM{events: events, writer: writer, reader: reader, agents: agents, factory: factory}
}

func (l *LLM) Generate(input port.LLMInput, role string) (string, error) {
	if input.Text == "" {
		return "", errors.New("empty text")
	}
	if role != "user" && role != "interviewer" {
		return "", errors.New("invalid role: " + role)
	}
	l.mu.Lock()
	l.cancelled = false
	l.mu.Unlock()
	agent, err := l.agents.GetActive()
	if err != nil {
		return "", err
	}
	if agent.ID == "" {
		_ = l.events.Emit("llm:error", map[string]string{"error": "no active agent"})
		return "", errors.New("no active agent")
	}
	engine, err := l.factory.ForAgent(agent)
	if err != nil {
		_ = l.events.Emit("llm:error", map[string]string{"error": err.Error()})
		return "", err
	}
	l.mu.Lock()
	l.current = engine
	l.mu.Unlock()

	history := []port.Message{}
	if l.reader != nil {
		if cur, err := l.reader.GetCurrent(); err == nil {
			history = cur.ChatHistory
		}
	}

	if l.writer != nil {
		if err := l.writer.AppendMessage(role, input.Text); err != nil {
			_ = l.events.Emit("llm:error", map[string]string{"error": err.Error()})
			return "", err
		}
	}
	if err := l.events.Emit("llm:started", struct{}{}); err != nil {
		return "", err
	}

	onToken := func(token string) {
		_ = l.events.Emit("llm:partial", map[string]string{"text": token})
	}
	answer, err := engine.Complete(input, history, onToken)
	if err != nil {
		l.mu.Lock()
		cancelled := l.cancelled
		l.mu.Unlock()
		if cancelled {
			// user cancelled mid-stream; do not surface llm:error (llm:cancelled already fired)
			return "", nil
		}
		_ = l.events.Emit("llm:error", map[string]string{"error": err.Error()})
		return "", err
	}
	l.mu.Lock()
	cancelled := l.cancelled
	l.mu.Unlock()
	if cancelled {
		// user cancelled mid-stream; do not persist a truncated answer or emit llm:response
		return "", nil
	}
	if answer == "" {
		_ = l.events.Emit("llm:error", map[string]string{"error": "empty response from LLM"})
		return "", errors.New("empty response from LLM")
	}
	if l.writer != nil {
		if err := l.writer.AppendMessage("assistant", answer); err != nil {
			_ = l.events.Emit("llm:error", map[string]string{"error": err.Error()})
			return "", err
		}
	}
	if err := l.events.Emit("llm:response", map[string]string{"text": answer}); err != nil {
		return "", err
	}
	return answer, nil
}

func (l *LLM) Cancel() error {
	l.mu.Lock()
	if l.cancelled {
		l.mu.Unlock()
		return nil
	}
	l.cancelled = true
	cur := l.current
	l.mu.Unlock()
	_ = l.events.Emit("llm:cancelled", struct{}{})
	if cur == nil {
		return nil
	}
	return cur.Cancel()
}

func (l *LLM) ListLocalModels() ([]string, error) {
	agent, err := l.agents.GetActive()
	if err != nil {
		return nil, err
	}
	if agent.Provider != "local" {
		return []string{}, nil
	}
	engine, err := l.factory.ForAgent(agent)
	if err != nil {
		return nil, err
	}
	tags, ok := engine.(interface{ Tags() ([]string, error) })
	if !ok {
		return []string{}, nil
	}
	return tags.Tags()
}
