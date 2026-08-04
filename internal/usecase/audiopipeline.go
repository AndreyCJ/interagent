package usecase

import (
	"sync"

	"interagent/internal/port"
)

// captureSampleRate is the native sample rate of the audio capture adapters
// (AVCaptureDevice / ScreenCaptureKit deliver 48 kHz mono float32).
const captureSampleRate = 48000

type audioInput interface {
	Start(onChunk func([]byte)) error
	Stop() error
}

type sttStreamer interface {
	Feed(chunk []byte) error
	Stream(sampleRate int, onPartial func(string), onDone func(text string, confidence float64, language string)) error
	Close() error
}

type llmGenerator interface {
	Generate(port.LLMInput, string) (string, error)
	Cancel() error
}

// AudioPipeline captures one audio source and routes finalized phrases
// (ADR-007): system → interviewer (auto-answer), mic → user (history only).
type AudioPipeline struct {
	events port.Events
	source port.AudioSource
	input  audioInput
	stt    sttStreamer
	llm    llmGenerator
	writer sessionWriter

	mu      sync.Mutex
	running bool
	gen     bool
}

func NewAudioPipeline(source port.AudioSource, events port.Events, input audioInput, stt sttStreamer, llm llmGenerator, writer sessionWriter) *AudioPipeline {
	return &AudioPipeline{source: source, events: events, input: input, stt: stt, llm: llm, writer: writer}
}

func (a *AudioPipeline) Start() error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return nil
	}
	a.running = true
	a.mu.Unlock()

	go func() {
		if err := a.stt.Stream(captureSampleRate, a.onPartial, a.onDone); err != nil {
			_ = a.events.Emit("app:error", map[string]string{"stage": "stt", "error": err.Error()})
		}
	}()
	if err := a.input.Start(func(chunk []byte) { _ = a.stt.Feed(chunk) }); err != nil {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
		_ = a.stt.Close()
		return err
	}
	return nil
}

func (a *AudioPipeline) Stop() error {
	a.mu.Lock()
	a.running = false
	a.mu.Unlock()
	if err := a.input.Stop(); err != nil {
		return err
	}
	return a.stt.Close()
}

func (a *AudioPipeline) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

func (a *AudioPipeline) onPartial(text string) {
	if text == "" {
		return
	}
	_ = a.events.Emit("transcription:partial", map[string]string{"text": text})
}

func (a *AudioPipeline) onDone(text string, confidence float64, language string) {
	if text == "" {
		return
	}
	_ = a.events.Emit("transcription:done", map[string]any{
		"text":       text,
		"confidence": confidence,
		"language":   language,
	})
	switch a.source {
	case port.AudioSourceSystem:
		a.autoAnswer(text, language)
	case port.AudioSourceMic:
		if a.writer != nil {
			_ = a.writer.AppendMessage("user", text)
		}
	}
}

// autoAnswer cancels an in-flight generation before starting a fresh one
// (ADR-007 "cancel on new input").
func (a *AudioPipeline) autoAnswer(text, language string) {
	a.mu.Lock()
	if a.gen {
		_ = a.llm.Cancel()
	}
	a.gen = true
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.gen = false
			a.mu.Unlock()
		}()
		_, _ = a.llm.Generate(port.LLMInput{Text: text, Language: language}, "interviewer")
	}()
}
