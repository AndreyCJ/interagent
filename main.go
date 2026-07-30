package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	sessionBind := NewSessionBind(&app.session)
	audioBind := NewAudioBind(&app.audio)
	screenshotBind := NewScreenshotBind(&app.screenshot)
	llmBind := NewLLMBind(&app.llm)
	agentBind := NewAgentBind(&app.agent)
	settingsBind := NewSettingsBind(&app.settings)

	err := wails.Run(&options.App{
		Title:  "interagent",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
			sessionBind,
			audioBind,
			screenshotBind,
			llmBind,
			agentBind,
			settingsBind,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
