package main

import (
	"context"

	"interagent/internal/usecase"
)

type App struct {
	ctx        context.Context
	session    usecase.Session
	audio      usecase.Audio
	screenshot usecase.Screenshot
	llm        usecase.LLM
	agent      usecase.Agent
	settings   usecase.Settings
}

func NewApp() *App {
	return &App{
		session:    *usecase.NewSession(nil),
		audio:      *usecase.NewAudio(nil, nil),
		screenshot: *usecase.NewScreenshot(nil, nil),
		llm:        *usecase.NewLLM(nil),
		agent:      *usecase.NewAgent(nil),
		settings:   *usecase.NewSettings(nil),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetVersion() string {
	return "0.1.0"
}

func (a *App) Quit() {

}
