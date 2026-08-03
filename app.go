package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	eventsimpl "interagent/internal/adapter/events"
	stubllm "interagent/internal/adapter/llm/stub"
	"interagent/internal/adapter/storage"
	adaptersub "interagent/internal/adapter/stub"
	"interagent/internal/adapter/window"
	"interagent/internal/usecase"
)

type App struct {
	ctx         context.Context
	session     usecase.Session
	audio       usecase.Audio
	screenshot  usecase.Screenshot
	llm         usecase.LLM
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
	store, err := storage.New(dbPath())
	if err != nil {
		panic("storage: " + err.Error())
	}

	ev := eventsimpl.New(context.TODO())
	llmStub := stubllm.New()
	overlayAdapter := window.New()
	hotkeysAdapter := adaptersub.NewHotkeys()
	permissionsAdapter := adaptersub.NewPermissions()

	overlay := usecase.NewOverlay(overlayAdapter, ev)
	sessionUC := usecase.NewSession(store)
	agentUC := usecase.NewAgent(store)
	settingsUC := usecase.NewSettings(store)

	return &App{
		session:     *sessionUC,
		audio:       *usecase.NewAudio(nil, nil),
		screenshot:  *usecase.NewScreenshot(nil, nil),
		llm:         *usecase.NewLLM(llmStub, ev, sessionUC),
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
		_, _ = a.llm.Generate(text)
		a.mu.Lock()
		a.generating = false
		a.mu.Unlock()
	}()
	return nil
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
