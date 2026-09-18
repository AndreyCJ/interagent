package ollama

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

func TestComplete_StreamsAndAggregates(t *testing.T) {
	var (
		mu       sync.Mutex
		gotModel string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		var req chatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		gotModel = req.Model
		mu.Unlock()
		if !req.Stream {
			t.Error("expected stream:true")
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer must flush")
		}
		w.Header().Set("Content-Type", "application/json")
		for _, part := range []string{"Hel", "lo ", "world"} {
			chunk, _ := json.Marshal(chatResponse{Message: ollamaMsg{Role: "assistant", Content: part}})
			fmt.Fprintln(w, string(chunk))
			flusher.Flush()
		}
		done, _ := json.Marshal(chatResponse{Message: ollamaMsg{Role: "assistant", Content: ""}, Done: true})
		fmt.Fprintln(w, string(done))
		flusher.Flush()
	}))
	defer srv.Close()

	o := New(srv.URL, "qwen3:8b", "You are a hint assistant.")
	var tokens []string
	answer, err := o.Complete(port.LLMInput{Text: "hi"}, nil, func(tok string) { tokens = append(tokens, tok) })
	if err != nil {
		t.Fatalf("Complete() returned error: %v", err)
	}
	if answer != "Hello world" {
		t.Errorf("answer = %q, want %q", answer, "Hello world")
	}
	mu.Lock()
	if gotModel != "qwen3:8b" {
		t.Errorf("model = %q, want qwen3:8b", gotModel)
	}
	mu.Unlock()
	if len(tokens) != 3 {
		t.Fatalf("tokens = %d, want 3 cumulative partials", len(tokens))
	}
	if tokens[len(tokens)-1] != "Hello world" {
		t.Errorf("last token = %q, want cumulative full text", tokens[len(tokens)-1])
	}
}

func TestComplete_EmptyText_ReturnsError(t *testing.T) {
	o := New("http://localhost:1", "m", "")
	if _, err := o.Complete(port.LLMInput{}, nil, nil); err == nil {
		t.Error("Complete() with empty text should fail")
	}
}

func TestComplete_Language_AppendsSystemInstruction(t *testing.T) {
	var req chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&req)
		chunk, _ := json.Marshal(chatResponse{Message: ollamaMsg{Role: "assistant", Content: "ok"}, Done: true})
		fmt.Fprintln(w, string(chunk))
	}))
	defer srv.Close()

	o := New(srv.URL, "m", "Base prompt.")
	if _, err := o.Complete(port.LLMInput{Text: "q", Language: "ru"}, nil, nil); err != nil {
		t.Fatalf("Complete() returned error: %v", err)
	}
	found := false
	for _, m := range req.Messages {
		if m.Role == "system" && strings.Contains(m.Content, "Answer in ru") {
			found = true
		}
	}
	if !found {
		t.Errorf("system message should contain language instruction: %+v", req.Messages)
	}
}

func TestTags_ReturnsModelNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintln(w, `{"models":[{"name":"qwen3:8b"},{"name":"llama3.1"}]}`)
	}))
	defer srv.Close()

	o := New(srv.URL, "m", "")
	names, err := o.Tags()
	if err != nil {
		t.Fatalf("Tags() returned error: %v", err)
	}
	if len(names) != 2 || names[0] != "qwen3:8b" {
		t.Errorf("Tags() = %v, want [qwen3:8b llama3.1]", names)
	}
}
