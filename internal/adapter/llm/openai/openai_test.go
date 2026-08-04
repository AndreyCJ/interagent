package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"interagent/internal/port"
)

func TestComplete_StreamsSSE_ReturnsAnswer(t *testing.T) {
	var (
		mu      sync.Mutex
		gotAuth string
		gotBody chatRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		mu.Unlock()
		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		for _, part := range []string{"Hi", "!"} {
			chunk, _ := json.Marshal(streamChunk{ID: "run-1", Choices: []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			}{{Delta: struct {
				Content string `json:"content"`
			}{Content: part}}}})
			fmt.Fprintf(w, "data: %s\n\n", string(chunk))
			flusher.Flush()
		}
		fmt.Fprintln(w, "data: [DONE]")
		flusher.Flush()
	}))
	defer srv.Close()

	o := New(srv.URL, "sk-test", "gpt-4o", "You help.")
	var tokens []string
	answer, err := o.Complete(port.LLMInput{Text: "hello"}, nil, func(t string) { tokens = append(tokens, t) })
	if err != nil {
		t.Fatalf("Complete() returned error: %v", err)
	}
	if answer != "Hi!" {
		t.Errorf("answer = %q, want Hi!", answer)
	}
	mu.Lock()
	if gotAuth != "Bearer sk-test" {
		t.Errorf("Authorization = %q, want Bearer sk-test", gotAuth)
	}
	mu.Unlock()
	if len(tokens) != 2 || tokens[len(tokens)-1] != "Hi!" {
		t.Errorf("tokens = %v, want cumulative [Hi Hi!]", tokens)
	}
}

func TestComplete_Language_AppendsSystemInstruction(t *testing.T) {
	var got chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		chunk, _ := json.Marshal(streamChunk{ID: "run-1", Choices: []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		}{}})
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n", chunk)
	}))
	defer srv.Close()

	o := New(srv.URL, "sk", "m", "Base.")
	if _, err := o.Complete(port.LLMInput{Text: "q", Language: "en"}, nil, nil); err != nil {
		t.Fatalf("Complete() returned error: %v", err)
	}
	found := false
	for _, m := range got.Messages {
		if m.Role == "system" && strings.Contains(m.Content, "Answer in en") {
			found = true
		}
	}
	if !found {
		t.Errorf("system message should contain language instruction: %+v", got.Messages)
	}
}

func TestComplete_HTTPError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	o := New(srv.URL, "sk", "m", "")
	if _, err := o.Complete(port.LLMInput{Text: "q"}, nil, nil); err == nil {
		t.Error("Complete() should fail on 429")
	}
}
