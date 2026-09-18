package usecase

import (
	"fmt"
	"strings"
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
	Stream(sampleRate int, onPartial func(string), onCommitted func(string), onDone func(text string, confidence float64, language string), onStatus func(bool)) error
	Close() error
}

type llmGenerator interface {
	Generate(port.LLMInput, string) (string, error)
	AnswerLast(port.LLMInput, string) (string, error)
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

	mu       sync.Mutex
	running  bool
	gen      bool
	seq      int
	phrase   string
	answered bool
	lang     string
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
		defer func() {
			// A panic in STT must not take the whole app down (live crash:
			// stream.go slice overreach on VAD time-mapped word ends). Surface it
			// as an error event and let the frontend decide how to recover.
			if r := recover(); r != nil {
				_ = a.events.Emit("app:error", map[string]string{"stage": "stt", "error": fmt.Sprintf("panic: %v", r)})
			}
		}()
		if err := a.stt.Stream(captureSampleRate, nil, a.onCommitted, a.onDone, a.onStatus); err != nil {
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

// onCommitted receives mid-speech words that whisper has committed beyond
// further revision (ADR-007). Surfaced to the chat live via
// transcription:committed with the CUMULATIVE phrase so far (ADR-007 amendment
// 2026-09-18), so the overlay grows one message per phrase instead of one
// bubble per span. A SYSTEM phrase auto-answers as soon as its completed
// sentence ends with "?" — the interviewer may keep talking, so we must not
// wait for phrase-end (ADR-007 amendment 2026-09-18); a phrase is answered at
// most once. Writes no history by itself and emits no transcription:done —
// only onDone finalizes the phrase.
func (a *AudioPipeline) onCommitted(text string) {
	if text == "" {
		return
	}
	a.mu.Lock()
	a.phrase = joinPhrase(a.phrase, text)
	full := a.phrase
	trigger := a.source == port.AudioSourceSystem && !a.answered && endsWithCompletedQuestion(full)
	if trigger {
		a.answered = true
	}
	a.mu.Unlock()
	_ = a.events.Emit("transcription:committed", map[string]string{
		"text":   full,
		"source": string(a.source),
	})
	if trigger {
		a.answerQuestion(full, a.lang)
	}
}

// onDone finalizes a phrase: the tail is appended to the accumulated committed
// spans and the COMPLETE phrase is delivered once — transcription:done carries
// the full text; the mic path persists one user message; the system path
// persists one interviewer message and auto-answers unless the phrase already
// answered at its "?" completion (ADR-007 amendment 2026-09-18). The phrase
// buffer (and the answered flag) reset here, so the next phrase starts fresh.
func (a *AudioPipeline) onDone(text string, confidence float64, language string) {
	a.mu.Lock()
	a.phrase = joinPhrase(a.phrase, trimOverlap(a.phrase, text))
	full := a.phrase
	a.phrase = ""
	answered := a.answered
	a.answered = false
	if language != "" {
		a.lang = language
	}
	a.mu.Unlock()
	if full == "" {
		return
	}
	_ = a.events.Emit("transcription:done", map[string]any{
		"text":       full,
		"confidence": confidence,
		"language":   language,
		"source":     string(a.source),
	})
	switch a.source {
	case port.AudioSourceSystem:
		if !answered {
			a.answerQuestion(full, language)
		}
	case port.AudioSourceMic:
		if a.writer != nil {
			_ = a.writer.AppendMessage("user", full)
		}
	}
}

// onStatus surfaces the whisper processing lifecycle: busy=true when a
// transcription run starts, busy=false when it returns. The frontend shows a
// loading indicator while the current transcription is being updated.
func (a *AudioPipeline) onStatus(busy bool) {
	name := "stt:idle"
	if busy {
		name = "stt:processing"
	}
	_ = a.events.Emit(name, map[string]string{"source": string(a.source)})
}

// answerQuestion persists the interviewer question once per phrase (ADR-007)
// and runs the answer. The question is already the last session message when
// LLM.AnswerLast fetches history, so the engine prompt gets it exactly once.
func (a *AudioPipeline) answerQuestion(question, language string) {
	if a.writer != nil {
		_ = a.writer.AppendMessage("interviewer", question)
	}
	a.answerLLM(question, language)
}

// answerLLM cancels an in-flight generation before starting a fresh one
// (ADR-007 "cancel on new input").
func (a *AudioPipeline) answerLLM(text, language string) {
	a.mu.Lock()
	if a.gen {
		_ = a.llm.Cancel()
	}
	a.gen = true
	a.seq++
	mine := a.seq
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			// only the newest generation may clear the in-flight flag; a
			// superseded (cancelled) one that returns after a newer generation
			// started must not clear it for the newer generation (ADR-007)
			if a.seq == mine {
				a.gen = false
			}
			a.mu.Unlock()
		}()
		_, _ = a.llm.AnswerLast(port.LLMInput{Text: text, Language: language}, "interviewer")
	}()
}

// endsWithCompletedQuestion reports whether the last completed sentence of the
// phrase — the one terminated by the final ".", "!" or "?" in the trimmed
// text — is a question. A trailing, still-unfinished fragment
// ("Is that it? Correct") does not nullify the completed question: the
// interviewer already asked it and must not be made to wait for phrase-end.
func endsWithCompletedQuestion(s string) bool {
	s = strings.TrimSpace(s)
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case '?':
			return true
		case '.', '!':
			return false
		}
	}
	return false
}

// joinPhrase appends a committed span or finalize tail to the phrase buffer,
// trimming surrounding whitespace so consecutive spans join into one sentence.
// If the new span already contains the ENTIRE accumulated buffer as a
// contiguous case-insensitive subsequence and is strictly longer, the span is a
// cleaner re-transcription of the same speech (whisper revising the whole
// remaining window after a boundary collapse) — the buffer is REPLACED instead
// of stacked, so the phrase never reads its own old fragments twice
// (ADR-007 amendment: superset-replace guard).
func joinPhrase(phrase, part string) string {
	phrase = strings.TrimSpace(phrase)
	part = strings.TrimSpace(part)
	if phrase == "" {
		return part
	}
	if part == "" {
		return phrase
	}
	if isSuperset(phrase, part) {
		return part
	}
	return phrase + " " + part
}

// isSuperset reports whether pat appears as a contiguous case-insensitive
// subsequence inside sup, which must hold strictly more word-tokens.
func isSuperset(pat, sup string) bool {
	p := strings.Fields(pat)
	s := strings.Fields(sup)
	if len(p) == 0 || len(s) <= len(p) {
		return false
	}
	for i := 0; i+len(p) <= len(s); i++ {
		ok := true
		for j, w := range p {
			if !strings.EqualFold(w, s[i+j]) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// trimOverlap strips a verbatim boundary repeat between the accumulated phrase
// and a finalize tail. whisper can re-transcribe the last committed word(s)
// into the done-tail after a boundary revision (stream.go finalize, ADR-007);
// without this guard the phrase would show the same words twice at a phrase
// end. Only the longest exact suffix-of-phrase / prefix-of-tail match is
// removed, so repeats that live wholly inside a single span are preserved.
func trimOverlap(phrase, tail string) string {
	p := strings.Fields(phrase)
	t := strings.Fields(tail)
	if len(p) == 0 || len(t) == 0 {
		return tail
	}
	max := len(t)
	if len(p) < max {
		max = len(p)
	}
	k := max
	for k > 0 && !equalWords(t[:k], p[len(p)-k:]) {
		k--
	}
	if k == 0 {
		return tail
	}
	return strings.Join(t[k:], " ")
}

func equalWords(a, b []string) bool {
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}
