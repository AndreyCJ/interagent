package main

type LLMBind struct {
	usecase interface {
		SendText(text string) error
		Cancel() error
	}
}

func NewLLMBind(u interface {
	SendText(text string) error
	Cancel() error
}) *LLMBind {
	return &LLMBind{usecase: u}
}

func (b *LLMBind) SendText(text string) error {
	return b.usecase.SendText(text)
}

func (b *LLMBind) CancelResponse() error {
	return b.usecase.Cancel()
}
