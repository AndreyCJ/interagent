package usecase

import (
	"errors"

	"interagent/internal/port"
)

type sessionWriter interface {
	AppendMessage(role, text string) error
}

type LLM struct {
	engine  port.LLM
	events  port.Events
	session sessionWriter
}

func NewLLM(engine port.LLM, events port.Events, session sessionWriter) *LLM {
	return &LLM{engine: engine, events: events, session: session}
}

func (l *LLM) Generate(text string) (string, error) {
	if text == "" {
		return "", errors.New("empty text")
	}
	if l.session != nil {
		if err := l.session.AppendMessage("user", text); err != nil {
			return "", err
		}
	}
	if err := l.events.Emit("llm:started", struct{}{}); err != nil {
		return "", err
	}
	answer, err := l.engine.Complete(text, nil)
	if err != nil {
		_ = l.events.Emit("llm:error", map[string]string{"error": err.Error()})
		return "", err
	}
	if answer == "" {
		_ = l.events.Emit("llm:error", map[string]string{"error": "empty response from LLM"})
		return "", errors.New("empty response from LLM")
	}
	if l.session != nil {
		if err := l.session.AppendMessage("assistant", answer); err != nil {
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
	return l.engine.Cancel()
}
