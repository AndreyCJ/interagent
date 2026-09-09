package openai

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

var _ port.LLM = (*OpenAI)(nil)

type OpenAI struct {
	baseURL      string
	apiKey       string
	model        string
	systemPrompt string
	client       *http.Client
	mu           sync.Mutex
	lastRunID    string
}

func New(baseURL, apiKey, model, systemPrompt string) *OpenAI {
	return &OpenAI{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		model:        model,
		systemPrompt: systemPrompt,
		client:       &http.Client{Timeout: 120 * time.Second},
	}
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []chatMsg `json:"messages"`
	Stream   bool      `json:"stream"`
}

type streamChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (o *OpenAI) roleFor(role string) string {
	if role == "assistant" {
		return "assistant"
	}
	return "user"
}

func (o *OpenAI) buildMessages(input port.LLMInput, history []port.Message) []chatMsg {
	system := o.systemPrompt
	if input.Language != "" {
		system += fmt.Sprintf("\nThe interviewer is speaking %s. Answer in %s.", input.Language, input.Language)
	}
	msgs := []chatMsg{}
	if system != "" {
		msgs = append(msgs, chatMsg{Role: "system", Content: system})
	}
	for _, m := range history {
		msgs = append(msgs, chatMsg{Role: o.roleFor(m.Role), Content: m.Text})
	}
	msgs = append(msgs, chatMsg{Role: "user", Content: input.Text})
	return msgs
}

func (o *OpenAI) Complete(input port.LLMInput, history []port.Message, onToken func(string)) (string, error) {
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
	req, err := http.NewRequest(http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai status %d", resp.StatusCode)
	}

	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", err
		}
		o.mu.Lock()
		if chunk.ID != "" {
			o.lastRunID = chunk.ID
		}
		o.mu.Unlock()
		for _, c := range chunk.Choices {
			if c.Delta.Content != "" {
				full.WriteString(c.Delta.Content)
				if onToken != nil {
					onToken(full.String())
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return full.String(), nil
}

func (o *OpenAI) Cancel() error {
	o.mu.Lock()
	runID := o.lastRunID
	o.mu.Unlock()
	if runID == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodDelete, o.baseURL+"/chat/completions/"+runID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
