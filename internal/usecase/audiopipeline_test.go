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
	mu          sync.Mutex
	onPartial   func(string)
	onCommitted func(string)
	onDone      func(string, float64, string)
	onStatus    func(bool)
	sampleRate  int
	feedCalls   int
	streamErr   error
	closed      bool
	stopCh      chan struct{}
}

func (m *mockPipelineSTT) Feed(chunk []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.feedCalls++
	return nil
}

func (m *mockPipelineSTT) Stream(sampleRate int, onPartial func(string), onCommitted func(string), onDone func(string, float64, string), onStatus func(bool)) error {
	m.mu.Lock()
	m.sampleRate = sampleRate
	m.onPartial = onPartial
	m.onCommitted = onCommitted
	m.onDone = onDone
	m.onStatus = onStatus
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

func (m *mockPipelineSTT) committed(text string) {
	m.mu.Lock()
	fn := m.onCommitted
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

func (m *mockPipelineSTT) status(busy bool) {
	m.mu.Lock()
	fn := m.onStatus
	m.mu.Unlock()
	if fn != nil {
		fn(busy)
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
	return m.answer(input, role)
}

func (m *mockPipelineLLM) AnswerLast(input port.LLMInput, role string) (string, error) {
	return m.answer(input, role)
}

func (m *mockPipelineLLM) answer(input port.LLMInput, role string) (string, error) {
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

func (m *mockHistory) Roles() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.roles)
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

func TestAudioPipeline_SttProcessingEvents(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	source := port.AudioSourceMic
	p := NewAudioPipeline(source, events, &mockPipelineInput{}, stt, &mockPipelineLLM{}, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	if events.count("stt:processing") != 0 || events.count("stt:idle") != 0 {
		t.Fatalf("status events before any status toggles: processing=%d idle=%d",
			events.count("stt:processing"), events.count("stt:idle"))
	}
	stt.status(true)
	waitFor(t, func() bool { return events.count("stt:processing") == 1 })
	if pl := events.payload("stt:processing", 0).(map[string]string); pl["source"] != string(source) {
		t.Errorf("stt:processing source = %q, want %q", pl["source"], source)
	}
	stt.status(false)
	waitFor(t, func() bool { return events.count("stt:idle") == 1 })
	if pl := events.payload("stt:idle", 0).(map[string]string); pl["source"] != string(source) {
		t.Errorf("stt:idle source = %q, want %q", pl["source"], source)
	}
}

func TestAudioPipeline_SystemCommitted_AccumulatesCumulativeEvents(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	history := &mockHistory{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, history)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.committed("the key point")
	stt.committed("you made is")

	waitFor(t, func() bool { return events.count("transcription:committed") == 2 })
	if events.count("transcription:done") != 0 {
		t.Error("committed speech must not emit transcription:done")
	}
	if inputs, _ := llm.calls(); len(inputs) != 0 {
		t.Error("committed speech must never trigger the LLM")
	}
	if history.Roles() != 0 {
		t.Errorf("committed speech must not write history, got %d rows", history.Roles())
	}
	first := events.payload("transcription:committed", 0).(map[string]string)
	if first["text"] != "the key point" || first["source"] != "system" {
		t.Errorf("transcription:committed[0] payload = %v, want {text:the key point, source:system}", first)
	}
	second := events.payload("transcription:committed", 1).(map[string]string)
	if second["text"] != "the key point you made is" || second["source"] != "system" {
		t.Errorf("transcription:committed[1] payload = %v, want cumulative {text:the key point you made is, source:system}", second)
	}
}

func TestAudioPipeline_MicCommitted_AccumulatesCumulativeEvents(t *testing.T) {
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
	stt.committed("my answer")
	stt.committed("is correct")

	waitFor(t, func() bool { return events.count("transcription:committed") == 2 })
	if events.count("transcription:done") != 0 {
		t.Error("committed speech must not emit transcription:done")
	}
	if inputs, _ := llm.calls(); len(inputs) != 0 {
		t.Error("committed speech must never trigger the LLM")
	}
	if history.Roles() != 0 {
		t.Errorf("committed speech must not write history, got %d rows", history.Roles())
	}
	last := events.payload("transcription:committed", 1).(map[string]string)
	if last["text"] != "my answer is correct" || last["source"] != "mic" {
		t.Errorf("transcription:committed[1] payload = %v, want {text:my answer is correct, source:mic}", last)
	}
}

func TestAudioPipeline_MicDone_HistoryGetsFullPhraseOnce(t *testing.T) {
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
	stt.committed("my answer")
	stt.committed("is")
	stt.done("clear", 0.8, "en")

	waitFor(t, func() bool { role, _ := history.last(); return role == "user" })
	if history.Roles() != 1 {
		t.Fatalf("history rows = %d, want 1 per phrase", history.Roles())
	}
	if role, text := history.last(); role != "user" || text != "my answer is clear" {
		t.Errorf("history = (%q, %q), want (user, my answer is clear)", role, text)
	}
	if inputs, _ := llm.calls(); len(inputs) != 0 {
		t.Error("mic transcription must never trigger the LLM")
	}
	done := events.payload("transcription:done", 0).(map[string]any)
	if done["text"] != "my answer is clear" {
		t.Errorf("transcription:done text = %v, want my answer is clear", done["text"])
	}
}

func TestAudioPipeline_SystemDone_LLMGetsFullPhrase(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	history := &mockHistory{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, history)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.committed("What is")
	stt.committed("your approach")
	stt.done("today?", 0.92, "en")

	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 1 })
	inputs, roles := llm.calls()
	if roles[0] != "interviewer" {
		t.Errorf("Generate role = %q, want interviewer", roles[0])
	}
	if inputs[0].Text != "What is your approach today?" || inputs[0].Language != "en" {
		t.Errorf("Generate input = %+v, want the full phrase", inputs[0])
	}
	if role, text := history.last(); role != "interviewer" || text != "What is your approach today?" {
		t.Errorf("pipeline must persist the interviewer phrase once, history = (%q, %q)", role, text)
	}
	if history.Roles() != 1 {
		t.Errorf("pipeline history rows = %d, want 1 per phrase", history.Roles())
	}
	done := events.payload("transcription:done", 0).(map[string]any)
	if done["text"] != "What is your approach today?" {
		t.Errorf("transcription:done text = %v, want the full phrase", done["text"])
	}
}

func TestAudioPipeline_Done_TrimsRevisedBoundaryOverlap(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.committed("I think this approach is")
	// whisper re-transcribes the boundary word into the done-tail (revised tail).
	stt.done("is the way to go", 0.9, "en")

	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 1 })
	inputs, _ := llm.calls()
	if inputs[0].Text != "I think this approach is the way to go" {
		t.Errorf("Generate input = %q, want boundary duplicate trimmed", inputs[0].Text)
	}
	done := events.payload("transcription:done", 0).(map[string]any)
	if done["text"] != "I think this approach is the way to go" {
		t.Errorf("transcription:done text = %q, want boundary duplicate trimmed", done["text"])
	}
}

func TestAudioPipeline_Done_KeepsInteriorRepeats(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.committed("go go")
	stt.done("faster", 0.9, "en")

	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 1 })
	inputs, _ := llm.calls()
	if inputs[0].Text != "go go faster" {
		t.Errorf("Generate input = %q, want interior repeats preserved", inputs[0].Text)
	}
}

// TestAudioPipeline_Done_TrimsCapitalizedBoundaryOverlap pins the Russian
// repro: the stream committed a phrase ending in "и", and the done-tail
// re-transcribes that boundary word capitalized ("И …"). Boundary-overlap
// trimming must match case-insensitively so the phrase does not read
// "...фракций, и И у их речей…".
func TestAudioPipeline_Done_TrimsCapitalizedBoundaryOverlap(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.committed("На этом заседании выступили главы всех фракций, и")
	stt.done("И у их речей есть особенность.", 0.9, "ru")

	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 1 })
	inputs, _ := llm.calls()
	if inputs[0].Text != "На этом заседании выступили главы всех фракций, и у их речей есть особенность." {
		t.Errorf("Generate input = %q, want capitalized boundary overlap trimmed", inputs[0].Text)
	}
}

// TestAudioPipeline_SupersetDone_ReplacesPollutedPhrase pins the top-level
// overlap family: after partial fragments accumulated, whisper finally returns
// a cleaner full re-transcription that contains the whole polluted buffer as a
// contiguous case-insensitive subsequence. The newer span must REPLACE the
// buffer, not append on top of it.
func TestAudioPipeline_SupersetDone_ReplacesPollutedPhrase(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.committed("заседании выступили")
	stt.done("На этом заседании выступили главы всех фракций, и у их речей.", 0.9, "ru")

	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 1 })
	inputs, _ := llm.calls()
	if inputs[0].Text != "На этом заседании выступили главы всех фракций, и у их речей." {
		t.Errorf("Generate input = %q, want superset span to replace the polluted buffer", inputs[0].Text)
	}
}

func TestAudioPipeline_NextPhraseStartsFresh(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.committed("first part")
	stt.done("tail", 0.9, "en")
	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 1 })

	stt.committed("second")
	waitFor(t, func() bool { return events.count("transcription:committed") >= 2 })
	committed := events.payload("transcription:committed", 1).(map[string]string)
	if committed["text"] != "second" {
		t.Errorf("committed after done = %q, want a fresh phrase, not accumulated across phrases", committed["text"])
	}
}

func TestTrimOverlap(t *testing.T) {
	cases := []struct {
		name   string
		phrase string
		tail   string
		want   string
	}{
		{"no overlap", "the key point", "you made is", "you made is"},
		{"single-word boundary repeat", "I think this approach is", "is the way to go", "the way to go"},
		{"multi-word boundary repeat", "the answer is very good", "very good day", "day"},
		{"whole tail repeats buffer end", "run run run", "run run", ""},
		{"empty phrase", "", "fresh tail", "fresh tail"},
		{"empty tail", "some phrase", "", ""},
		{"interior repeat untouched", "go go", "faster", "faster"},
		{"whitespace tolerant", "a  b", "  b c", "c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := trimOverlap(tc.phrase, tc.tail); got != tc.want {
				t.Errorf("trimOverlap(%q, %q) = %q, want %q", tc.phrase, tc.tail, got, tc.want)
			}
		})
	}
}

func TestJoinPhrase(t *testing.T) {
	cases := []struct {
		name   string
		phrase string
		part   string
		want   string
	}{
		{"first span", "", "the key point", "the key point"},
		{"continues", "the key point", "you made is", "the key point you made is"},
		{"empty part", "the key point", "", "the key point"},
		{"empty both", "", "", ""},
		{"leading-space span", "the key point", " you made is", "the key point you made is"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := joinPhrase(tc.phrase, tc.part); got != tc.want {
				t.Errorf("joinPhrase(%q, %q) = %q, want %q", tc.phrase, tc.part, got, tc.want)
			}
		})
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
	if done["source"] != "system" {
		t.Errorf("transcription:done source = %v, want system", done["source"])
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
	done := events.payload("transcription:done", 0).(map[string]any)
	if done["source"] != "mic" {
		t.Errorf("transcription:done source = %v, want mic", done["source"])
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

func TestEndsWithCompletedQuestion(t *testing.T) {
	cases := []struct {
		name   string
		phrase string
		want   bool
	}{
		{"simple question", "What do you think?", true},
		{"trailing space", "Really? ", true},
		{"question with trailing fragment", "Is that it? Correct", true},
		{"statement", "That is all.", false},
		{"exclaim", "Wow!", false},
		{"question then statement", "Hmm? Okay.", false},
		{"question then trailing statement", "What do you think? Actually never mind.", false},
		{"no terminal", "What do you think", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := endsWithCompletedQuestion(tc.phrase); got != tc.want {
				t.Errorf("endsWithCompletedQuestion(%q) = %v, want %v", tc.phrase, got, tc.want)
			}
		})
	}
}

func TestAudioPipeline_SystemCommitted_QuestionTriggersAnswerLast(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	history := &mockHistory{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, history)
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.committed("What do you")
	stt.committed("think?")

	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 1 })
	inputs, roles := llm.calls()
	if roles[0] != "interviewer" {
		t.Errorf("AnswerLast role = %q, want interviewer", roles[0])
	}
	if inputs[0].Text != "What do you think?" {
		t.Errorf("AnswerLast input = %q, want the completed question", inputs[0].Text)
	}
	if role, text := history.last(); role != "interviewer" || text != "What do you think?" {
		t.Errorf("history append = (%q, %q), want (interviewer, What do you think?)", role, text)
	}
	if events.count("transcription:done") != 0 {
		t.Error("the ?-trigger must not emit transcription:done itself")
	}
}

func TestAudioPipeline_SystemCommitted_MidSentence_NoTrigger(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.committed("What do you")
	stt.committed("think")
	time.Sleep(50 * time.Millisecond)
	if inputs, _ := llm.calls(); len(inputs) != 0 {
		t.Errorf("incomplete sentence must not trigger the LLM, got %d calls", len(inputs))
	}
	if events.count("transcription:done") != 0 {
		t.Error("incomplete sentence must not emit transcription:done")
	}
}

func TestAudioPipeline_SystemDone_Statement_StillAnswers(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.done("That is all.", 0.9, "fr")

	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 1 })
	inputs, _ := llm.calls()
	if inputs[0].Text != "That is all." || inputs[0].Language != "fr" {
		t.Errorf("AnswerLast input = %+v, want the final phrase with language", inputs[0])
	}
}

func TestAudioPipeline_SystemDone_QuestionAlreadyAnswered_NoDoubleAnswer(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.committed("What do you think?")
	waitFor(t, func() bool { inputs, _ := llm.calls(); return len(inputs) == 1 })

	stt.done("What do you think?", 0.92, "en")
	waitFor(t, func() bool { return events.count("transcription:done") == 1 })
	time.Sleep(50 * time.Millisecond)
	if inputs, _ := llm.calls(); len(inputs) != 1 {
		t.Errorf("LLM calls = %d, want 1 — the answered question must not be answered twice", len(inputs))
	}
}

func TestAudioPipeline_SystemCommitted_StatementMidPhrase_NoTrigger(t *testing.T) {
	events := newEventRecorder()
	stt := &mockPipelineSTT{}
	llm := &mockPipelineLLM{}
	p := NewAudioPipeline(port.AudioSourceSystem, events, &mockPipelineInput{}, stt, llm, &mockHistory{})
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	waitFor(t, func() bool { return stt.sampleRateValue() == 48000 })
	stt.committed("Let me explain.")
	time.Sleep(50 * time.Millisecond)
	if inputs, _ := llm.calls(); len(inputs) != 0 {
		t.Errorf("statement committed must not trigger, got %d calls", len(inputs))
	}
}

func TestAudioPipeline_MicCommitted_Question_NoTrigger(t *testing.T) {
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
	stt.committed("What do you think?")
	time.Sleep(50 * time.Millisecond)
	if inputs, _ := llm.calls(); len(inputs) != 0 {
		t.Errorf("mic ?-phrase must never trigger the LLM, got %d calls", len(inputs))
	}
	if history.Roles() != 0 {
		t.Errorf("mic ?-phrase must not write history yet, got %d rows", history.Roles())
	}
}
