package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"interagent/internal/port"
)

type Ollama struct {
	baseURL      string
	model        string
	systemPrompt string
	client       *http.Client
	mu           sync.Mutex
	lastModel    string
}

func New(baseURL, model, systemPrompt string) *Ollama {
	return &Ollama{
		baseURL:      strings.TrimRight(baseURL, "/"),
		model:        model,
		systemPrompt: systemPrompt,
		client:       &http.Client{Timeout: 120 * time.Second},
	}
}

type ollamaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string      `json:"model"`
	Messages []ollamaMsg `json:"messages"`
	Stream   bool        `json:"stream"`
}

type chatResponse struct {
	Message ollamaMsg `json:"message"`
	Done    bool      `json:"done"`
	Error   string    `json:"error,omitempty"`
}

func roleFor(role string) string {
	if role == "assistant" {
		return "assistant"
	}
	return "user"
}

func (o *Ollama) buildMessages(input port.LLMInput, history []port.Message) []ollamaMsg {
	system := o.systemPrompt
	if input.Language != "" {
		system += fmt.Sprintf("\nThe interviewer is speaking %s. Answer in %s.", input.Language, input.Language)
	}
	msgs := []ollamaMsg{}
	if system != "" {
		msgs = append(msgs, ollamaMsg{Role: "system", Content: system})
	}
	for _, m := range history {
		msgs = append(msgs, ollamaMsg{Role: roleFor(m.Role), Content: m.Text})
	}
	msgs = append(msgs, ollamaMsg{Role: "user", Content: input.Text})
	return msgs
}

func (o *Ollama) Complete(input port.LLMInput, history []port.Message, onToken func(string)) (string, error) {
	if input.Text == "" {
		return "", errors.New("empty input")
	}
	if o.model == "" {
		return "", errors.New("empty model")
	}
	body, err := json.Marshal(chatRequest{Model: o.model, Messages: o.buildMessages(input, history), Stream: true})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	o.mu.Lock()
	o.lastModel = o.model
	o.mu.Unlock()

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama status %d", resp.StatusCode)
	}

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var chunk chatResponse
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			return "", err
		}
		if chunk.Error != "" {
			return "", errors.New(chunk.Error)
		}
		if chunk.Message.Content != "" {
			full.WriteString(chunk.Message.Content)
			if onToken != nil {
				onToken(full.String())
			}
		}
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return full.String(), nil
}

func (o *Ollama) Cancel() error {
	o.mu.Lock()
	model := o.lastModel
	o.mu.Unlock()
	if model == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]string{"model": model})
	req, err := http.NewRequest(http.MethodPost, o.baseURL+"/api/cancel", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (o *Ollama) Tags() ([]string, error) {
	resp, err := o.client.Get(o.baseURL + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama tags status %d", resp.StatusCode)
	}
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Models))
	for _, m := range out.Models {
		names = append(names, m.Name)
	}
	return names, nil
}
