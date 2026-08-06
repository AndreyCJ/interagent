package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"interagent/internal/adapter/audio"
	cryptoadapter "interagent/internal/adapter/crypto"
	eventsimpl "interagent/internal/adapter/events"
	"interagent/internal/adapter/llm/ollama"
	"interagent/internal/adapter/llm/openai"
	modelsadapter "interagent/internal/adapter/models"
	"interagent/internal/adapter/storage"
	whisperadapter "interagent/internal/adapter/stt/whisper"
	"interagent/internal/adapter/stub"
	"interagent/internal/adapter/system"
	"interagent/internal/adapter/window"
	"interagent/internal/port"
	"interagent/internal/usecase"
)

type App struct {
	ctx         context.Context
	session     usecase.Session
	audioSystem *usecase.AudioPipeline
	audioMic    *usecase.AudioPipeline
	models      *usecase.Models
	captureMic  *audio.MicrophoneCapture
	screenshot  usecase.Screenshot
	llm         *usecase.LLM
	agent       usecase.Agent
	settings    usecase.Settings
	overlay     usecase.Overlay
	hotkeys     usecase.Hotkeys
	permissions usecase.Permissions
	events      *eventsimpl.Events
	overlayWin  *window.Overlay

	mu         sync.Mutex
	generating bool
}

func NewApp() *App {
	crypt, err := cryptoadapter.NewCrypto(cryptoadapter.NewKeychain())
	if err != nil {
		panic("crypto: " + err.Error())
	}
	store, err := storage.New(dbPath(), crypt)
	if err != nil {
		panic("storage: " + err.Error())
	}

	ev := eventsimpl.New(context.TODO())
	overlayAdapter := window.New()
	hotkeysAdapter := stub.NewHotkeys()
	permissionsAdapter := system.NewPermissions()

	factory := usecase.LLMFactory(func(cfg port.AgentConfig) (port.LLM, error) {
		switch cfg.Provider {
		case "local":
			return ollama.New(cfg.BaseURL, cfg.Model, cfg.SystemPrompt), nil
		case "openai-compatible":
			key, err := crypt.Decrypt(cfg.APIKey)
			if err != nil {
				return nil, fmt.Errorf("decrypt api key: %w", err)
			}
			if key == "" {
				return nil, errors.New("empty api key")
			}
			return openai.New(cfg.BaseURL, key, cfg.Model, cfg.SystemPrompt), nil
		default:
			return nil, errors.New("unknown provider: " + cfg.Provider)
		}
	})

	overlay := usecase.NewOverlay(overlayAdapter, ev)
	sessionUC := usecase.NewSession(store)
	agentUC := usecase.NewAgent(store)
	settingsUC := usecase.NewSettings(store)
	llm := usecase.NewLLM(ev, sessionUC, sessionUC, agentUC, factory)

	modelsDir := modelsDirPath()
	modelStore := modelsadapter.New(modelsDir)
	modelsUC := usecase.NewModels(ev, modelStore)

	modelPath := filepath.Join(modelsDir, "ggml-base.bin")
	vadPath := filepath.Join(modelsDir, "ggml-silero-v6.2.0.bin")
	sttSystem := whisperadapter.New(modelPath, vadPath)
	sttMic := whisperadapter.New(modelPath, vadPath)
	captureSystem := audio.NewSystemCapture()
	captureMic := audio.NewMicrophoneCapture()
	audioSystem := usecase.NewAudioPipeline(port.AudioSourceSystem, ev, captureSystem, sttSystem, llm, sessionUC)
	audioMic := usecase.NewAudioPipeline(port.AudioSourceMic, ev, captureMic, sttMic, llm, sessionUC)

	return &App{
		session:     *sessionUC,
		audioSystem: audioSystem,
		audioMic:    audioMic,
		models:      modelsUC,
		captureMic:  captureMic,
		screenshot:  *usecase.NewScreenshot(nil, nil),
		llm:         llm,
		agent:       *agentUC,
		settings:    *settingsUC,
		overlay:     *overlay,
		hotkeys:     *usecase.NewHotkeys(hotkeysAdapter, overlay),
		permissions: *usecase.NewPermissions(permissionsAdapter, ev),
		events:      ev,
		overlayWin:  overlayAdapter,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.events.SetContext(ctx)
	a.overlayWin.SetContext(ctx)
	runtime.WindowSetAlwaysOnTop(ctx, true)
	mode, _ := a.overlay.GetMode()
	_ = a.overlay.SetMode(mode)
	_ = a.permissions.CheckAll()
}

func (a *App) GetVersion() string {
	return "0.1.0"
}

func (a *App) Quit() {}

func (a *App) SendText(text string) error {
	a.mu.Lock()
	if a.generating {
		_ = a.llm.Cancel()
	}
	a.generating = true
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.generating = false
			a.mu.Unlock()
		}()
		_, _ = a.llm.Generate(port.LLMInput{Text: text}, "user")
	}()
	return nil
}

func (a *App) GetOllamaModels() ([]string, error) {
	return a.llm.ListLocalModels()
}

func (a *App) StartListening() error {
	if ok, err := a.permissions.Status(port.PermissionScreenCapture); err == nil && !ok {
		return errors.New("screen recording permission required for system sound")
	}
	if ok, err := a.permissions.Status(port.PermissionMicrophone); err == nil && !ok {
		return errors.New("microphone permission required")
	}
	if err := a.audioSystem.Start(); err != nil {
		return err
	}
	if err := a.audioMic.Start(); err != nil {
		_ = a.audioSystem.Stop()
		return err
	}
	return nil
}

func (a *App) StopListening() error {
	if err := a.audioSystem.Stop(); err != nil {
		return err
	}
	return a.audioMic.Stop()
}

func (a *App) IsListening() bool {
	return a.audioSystem.IsRunning() || a.audioMic.IsRunning()
}

func (a *App) GetAudioDevices() ([]port.AudioDevice, error) { return a.captureMic.Devices() }

func (a *App) SetAudioDevice(id string) error { return a.captureMic.SetDevice(id) }

func (a *App) DownloadSTTModel() error {
	if err := a.models.Download("ggml-base"); err != nil {
		return err
	}
	return a.models.Download("silero-vad")
}

func (a *App) GetSTTModelStatus() (port.STTModelStatus, error) {
	return a.models.Status("ggml-base")
}

func (a *App) Cancel() error {
	a.mu.Lock()
	a.generating = false
	a.mu.Unlock()
	return a.llm.Cancel()
}

func dbPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "interagent.db")
	}
	dir = filepath.Join(dir, "interagent")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "interagent.db")
}

func modelsDirPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "interagent", "models")
	}
	dir = filepath.Join(dir, "interagent", "models")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}
