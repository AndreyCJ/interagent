package port

type LLM interface {
	Complete(prompt string, history []Message) (string, error)
	Cancel() error
}
