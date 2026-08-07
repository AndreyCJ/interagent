package usecase

import (
	"sync"
	"testing"
	"time"

	"interagent/internal/port"
)

type mockPipelineInput struct {
	mu       sync.Mutex
	onChunk  func([]byte)
	startErr error
	stopErr  error
	started  int
	stopped  int
}

func (m *mockPipelineInput) Start(onChunk func([]byte)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return m.startErr
	}
	m.onChunk = onChunk
	m.started++
	return nil
}

func (m *mockPipelineInput) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped++
	return m.stopErr
}

func (m *mockPipelineInput) startedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

type mockPipelineSTT struct {
	mu         sync.Mutex
	onPartial  func(string)
	onDone     func(string, float64, string)
	sampleRate int
	feedCalls  int
	streamErr  error
	closed     bool
	stopCh     chan struct{}
}

func (m *mockPipelineSTT) Feed(chunk []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.feedCalls++
	return nil
}

func (m *mockPipelineSTT) Stream(sampleRate int, onPartial func(string), onDone func(string, float64, string)) error {
	m.mu.Lock()
	m.sampleRate = sampleRate
	m.onPartial = onPartial
	m.onDone = onDone
	m.stopCh = make(chan struct{})
	m.mu.Unlock()
	if m.streamErr != nil {
		return m.streamErr
	}
	<-m.stopCh
	return nil
}

func (m *mockPipelineSTT) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	if m.stopCh != nil {
		close(m.stopCh)
	}
	return nil
}

func (m *mockPipelineSTT) partial(text string) {
	m.mu.Lock()
	fn := m.onPartial
	m.mu.Unlock()
	if fn != nil {
		fn(text)
	}
}

func (m *mockPipelineSTT) done(text string, confidence float64, language string) {
	m.mu.Lock()
	fn := m.onDone
	m.mu.Unlock()
	if fn != nil {
		fn(text, confidence, language)
	}
}

func (m *mockPipelineSTT) sampleRateValue() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sampleRate
}

type mockPipelineLLM struct {
	mu          sync.Mutex
	inputs      []port.LLMInput
	roles       []string
	cancelCalls int
	block       chan struct{}
	gates       []chan struct{}
	returns     int
}

func (m *mockPipelineLLM) Generate(input port.LLMInput, role string) (string, error) {
	m.mu.Lock()
	m.inputs = append(m.inputs, input)
	m.roles = append(m.roles, role)
	var gate chan struct{}
	if m.block != nil {
		gate = make(chan struct{})
		m.gates = append(m.gates, gate)
	}
	m.mu.Unlock()
	if gate != nil {
		<-gate
	}
	m.mu.Lock()
	m.returns++
	m.mu.Unlock()
	return "answer", nil
}

func (m *mockPipelineLLM) Cancel() error {
	m.mu.Lock()
	m.cancelCalls++
	if n := len(m.gates); n > 0 {
		close(m.gates[n-1])
		m.gates = m.gates[:n-1]
	}
	m.mu.Unlock()
	return nil
}

func (m *mockPipelineLLM) calls() ([]port.LLMInput, []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]port.LLMInput{}, m.inputs...), append([]string{}, m.roles...)
}

func (m *mockPipelineLLM) cancels() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cancelCalls
}

func (m *mockPipelineLLM) returned() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.returns
}

type mockHistory struct {
	mu    sync.Mutex
	roles []string
	texts []string
}

func (m *mockHistory) AppendMessage(role, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roles = append(m.roles, role)
	m.texts = append(m.texts, text)
	return nil
}

func (m *mockHistory) last() (string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.roles) == 0 {
		return "", ""
	}
	return m.roles[len(m.roles)-1], m.texts[len(m.texts)-1]
}

func TestAudioPipeline_Start_StartsInputAndStream(t *testing.T) {
	events := newEventRecorder()
	input := &mockPipelineInput{}
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, input, stt, llm, &mockHistory{})

	if err := p.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if input.startedCount() != 1 {
		t.Errorf("input.Start not called exactly once, got %d", input.startedCount())
	}
	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	if stt.sampleRateValue() != 48000 {
		t.Errorf("Stream sampleRate = %d, want 48000", stt.sampleRateValue())
	}
	if !p.IsRunning() {
		t.Error("IsRunning() = false after Start")
	}
}

func TestAudioPipeline_Start_InputError_StopsStream(t *testing.T) {
	events := newEventRecorder()
	input := &mockPipelineInput{startErr: errTest}
	stt := &mockPipelineSTT{}
	p := NewAudioPipeline(port.AudioSourceMic, events, input, stt, &mockPipelineLLM{}, &mockHistory{})

	if err := p.Start(); err == nil {
		t.Fatal("Start() should return input error")
	}
	waitFor(t, func() bool {
		stt.mu.Lock()
		defer stt.mu.Unlock()
		return stt.closed
	})
	if p.IsRunning() {
		t.Error("IsRunning() = true after failed Start")
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "boom" }

func TestAudioPipeline_NoPartialEvents(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, &mockPipelineLLM{}, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	// The pipeline registers no partial callback; the mock's partial
	// invocations are nil-safe and must not surface as events.
	stt.partial("Hel")
	stt.partial("Hello")
	if events.count("transcription:partial") != 0 {
		t.Errorf("transcription:partial count = %d, want 0", events.count("transcription:partial"))
	}
}

func TestAudioPipeline_SystemDone_AutoAnswers(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.done("What is your approach?", 0.92, "en")

	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 1 })
	inputs, roles := llm.calls()
	if roles[0] != "interviewer" {
		t.Errorf("Generate role = %q, want interviewer", roles[0])
	}
	if inputs[0].Text != "What is your approach?" || inputs[0].Language != "en" {
		t.Errorf("Generate input = %+v", inputs[0])
	}
	if events.count("transcription:done") != 1 {
		t.Fatalf("transcription:done count = %d", events.count("transcription:done"))
	}
	done := events.payload("transcription:done", 0).(map[string]any)
	if done["text"] != "What is your approach?" || done["confidence"] != 0.92 || done["language"] != "en" {
		t.Errorf("transcription:done payload = %v", done)
	}
}

func TestAudioPipeline_SystemDone_EmptyText_NoLLM(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.done("", 0, "")
	time.Sleep(50 * time.Millisecond)
	if inputs, _ := llm.calls(); len(inputs) != 0 {
		t.Error("empty transcription must not trigger the LLM")
	}
	if events.count("transcription:done") != 0 {
		t.Error("empty transcription must not emit transcription:done")
	}
}

func TestAudioPipeline_MicDone_HistoryOnly(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	history := &mockHistory{}
	p := NewAudioPipeline(port.AudioSourceMic, events, &mockPipelineInput{}, stt, llm, history)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.done("my answer", 0.8, "ru")

	waitFor(t, func() bool { role, _ := history.last(); return role == "user" })
	if events.count("transcription:done") != 1 {
		t.Fatalf("transcription:done count = %d", events.count("transcription:done"))
	}
	if inputs, _ := llm.calls(); len(inputs) != 0 {
		t.Error("mic transcription must never trigger the LLM")
	}
	if role, text := history.last(); role != "user" || text != "my answer" {
		t.Errorf("history append = (%q, %q), want (user, my answer)", role, text)
	}
}

func TestAudioPipeline_NewInputDuringGeneration_CancelsThenAnswers(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{block: make(chan struct{})}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.done("first question", 0.9, "en")
	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 1 })

	// The second question arrives while the first Generate is still blocked,
	// so cancel-on-new-input is exercised deterministically: Cancel() releases
	// the cancelled generation's gate, while the second stays blocked.
	stt.done("second question", 0.95, "en")
	waitFor(t, func() bool { return llm.cancels() >= 1 })
	close(llm.block)
	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 2 })
	inputs, _ := llm.calls()
	if inputs[1].Text != "second question" {
		t.Errorf("second Generate input = %+v", inputs[1])
	}
	if llm.cancels() < 1 {
		t.Error("Cancel() must be called when a new question arrives during generation")
	}
}

func TestAudioPipeline_NewInputDuringGeneration_CancelsTwice(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{block: make(chan struct{})}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })

	stt.done("first question", 0.9, "en")
	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 1 })

	// Second done cancels G1 (Cancel releases its gate, so G1 returns) and
	// starts G2, which stays blocked on its own gate.
	stt.done("second question", 0.95, "en")
	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 2 })
	// Wait for G1's cancelled Generate to return so its goroutine has finished;
	// a stale gen clear (G1 finishing after G2 started) would then already have
	// happened before the third done fires.
	waitFor(t, func() bool { return llm.returned() >= 1 })

	// G2 is still generating, so the third done must cancel it too (not spawn
	// G3 concurrently with G2) — the regressed behavior skipped this Cancel.
	stt.done("third question", 0.97, "en")
	waitFor(t, func() bool { return llm.cancels() >= 2 })
	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 3 })
	inputs, _ := llm.calls()
	if inputs[2].Text != "third question" {
		t.Errorf("third Generate input = %+v", inputs[2])
	}
	if llm.cancels() < 2 {
		t.Error("Cancel() must be called for every superseded generation")
	}
}

func TestAudioPipeline_Stop_StopsInputAndClosesSTT(t *testing.T) {
	events := newEventRecorder()
	input := &mockPipelineInput{}
	stt := &mockPipelineSTT{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, input, stt, &mockPipelineLLM{}, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
	waitFor(t, func() bool {
		stt.mu.Lock()
		defer stt.mu.Unlock()
		return stt.closed
	})
	if input.stopped != 1 {
		t.Errorf("input.Stop not called once, got %d", input.stopped)
	}
	if p.IsRunning() {
		t.Error("IsRunning() = true after Stop")
	}
}

func TestAudioPipeline_StartTwice_Idempotent(t *testing.T) {
	events := newEventRecorder()
	input := &mockPipelineInput{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, input, &mockPipelineSTT{}, &mockPipelineLLM{}, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	if err := p.Start(); err != nil {
		t.Fatalf("second Start() error: %v", err)
	}
	if input.startedCount() != 1 {
		t.Errorf("input.Start called %d times, want 1", input.startedCount())
	}
}
