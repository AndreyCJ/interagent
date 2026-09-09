package usecase

import (
	"errors"
	"sync"
	"testing"
	"time"
)

type mockModelStore struct {
	mu            sync.Mutex
	installed     map[string]bool
	statusErr     error
	downloadErr   error
	progress      []int64
	total         int64
	downloadCalls []string
}

func (m *mockModelStore) Status(model string) (bool, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.statusErr != nil {
		return false, "", m.statusErr
	}
	return m.installed[model], "/models/" + model, nil
}

func (m *mockModelStore) Download(model string, onProgress func(received, total int64)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloadCalls = append(m.downloadCalls, model)
	if m.downloadErr != nil {
		return m.downloadErr
	}
	for _, p := range m.progress {
		onProgress(p, m.total)
	}
	return nil
}

type eventRecorder struct {
	mu      sync.Mutex
	emitted map[string][]any
}

func newEventRecorder() *eventRecorder {
	return &eventRecorder{emitted: make(map[string][]any)}
}

func (r *eventRecorder) Emit(name string, payload any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.emitted[name] = append(r.emitted[name], payload)
	return nil
}

func (r *eventRecorder) count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.emitted[name])
}

func (r *eventRecorder) payload(name string, i int) any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.emitted[name][i]
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met within 2s")
}

func TestModels_Status_NotInstalled(t *testing.T) {
	store := &mockModelStore{installed: map[string]bool{}}
	m := NewModels(newEventRecorder(), store)

	status, err := m.Status("ggml-base")
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if status.Installed || status.Path == "" {
		t.Errorf("Status() = %+v", status)
	}
}

func TestModels_Status_Installed(t *testing.T) {
	store := &mockModelStore{installed: map[string]bool{"ggml-base": true}}
	m := NewModels(newEventRecorder(), store)

	status, err := m.Status("ggml-base")
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if !status.Installed {
		t.Error("Status() = not installed, want installed")
	}
}

func TestModels_Status_PropagatesError(t *testing.T) {
	store := &mockModelStore{statusErr: errors.New("storage boom")}
	m := NewModels(newEventRecorder(), store)

	if _, err := m.Status("ggml-base"); err == nil {
		t.Fatal("Status() should propagate store error")
	}
}

func TestModels_Download_EmitsProgressAndDone(t *testing.T) {
	events := newEventRecorder()
	store := &mockModelStore{
		installed: map[string]bool{},
		progress:  []int64{1, 10, 20, 40, 60, 80, 100},
		total:     100,
	}
	m := NewModels(events, store)

	if err := m.Download("ggml-base"); err != nil {
		t.Fatalf("Download() error: %v", err)
	}

	waitFor(t, func() bool { return events.count("model:downloaded") == 1 })

	if got := events.count("model:download-progress"); got == 0 {
		t.Fatal("no model:download-progress events")
	}
	last := events.payload("model:download-progress", events.count("model:download-progress")-1).(map[string]any)
	if last["received"] != int64(100) || last["total"] != int64(100) {
		t.Errorf("final progress = %v, want received==total==100", last)
	}
	// Throttle: 7 progress callbacks must produce strictly fewer events.
	if events.count("model:download-progress") >= 7 {
		t.Errorf("progress not throttled: %d events for 7 callbacks", events.count("model:download-progress"))
	}
	done := events.payload("model:downloaded", 0).(map[string]string)
	if done["model"] != "ggml-base" {
		t.Errorf("model:downloaded model = %q", done["model"])
	}
}

func TestModels_Download_AlreadyInstalled_EmitsDone(t *testing.T) {
	events := newEventRecorder()
	store := &mockModelStore{installed: map[string]bool{"ggml-base": true}}
	m := NewModels(events, store)

	if err := m.Download("ggml-base"); err != nil {
		t.Fatalf("Download() error: %v", err)
	}
	waitFor(t, func() bool { return events.count("model:downloaded") == 1 })
	if events.count("model:download-progress") != 0 {
		t.Error("already-installed download must not emit progress")
	}
	if len(store.downloadCalls) != 0 {
		t.Error("already-installed download must not call store.Download")
	}
}

func TestModels_Download_Failure_EmitsAppError(t *testing.T) {
	events := newEventRecorder()
	store := &mockModelStore{
		installed:   map[string]bool{},
		downloadErr: errors.New("network down"),
	}
	m := NewModels(events, store)

	if err := m.Download("ggml-base"); err != nil {
		t.Fatalf("Download() should launch async and return nil, got: %v", err)
	}
	waitFor(t, func() bool { return events.count("app:error") == 1 })
	payload := events.payload("app:error", 0).(map[string]string)
	if payload["stage"] != "models" || payload["error"] != "network down" {
		t.Errorf("app:error = %v", payload)
	}
	if events.count("model:downloaded") != 0 {
		t.Error("failed download must not emit model:downloaded")
	}
}
