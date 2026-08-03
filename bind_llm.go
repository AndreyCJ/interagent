package main

type LLMBind struct {
	app interface {
		SendText(text string) error
		Cancel() error
	}
}

func NewLLMBind(app interface {
	SendText(text string) error
	Cancel() error
}) *LLMBind {
	return &LLMBind{app: app}
}

func (b *LLMBind) SendText(text string) error {
	return b.app.SendText(text)
}

func (b *LLMBind) CancelResponse() error {
	return b.app.Cancel()
}
