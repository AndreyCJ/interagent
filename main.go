package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/src/app/dist
var assets embed.FS

func newOverlayMenu(app *App) *menu.Menu {
	m := menu.NewMenuFromItems(
		menu.AppMenu(),
		menu.EditMenu(),
		menu.WindowMenu(),
	)
	overlay := m.AddSubmenu("Overlay")
	overlay.AddText("Show Overlay", nil, func(_ *menu.CallbackData) {
		_ = app.overlay.Show()
	})
	overlay.AddText("Hide Overlay", nil, func(_ *menu.CallbackData) {
		_ = app.overlay.Hide()
	})
	overlay.AddText("Toggle Click-through", nil, func(_ *menu.CallbackData) {
		_ = app.overlay.ToggleMode()
	})
	return m
}

func main() {
	app := NewApp()

	sessionBind := NewSessionBind(app.session)
	audioBind := NewAudioBind(app)
	modelsBind := NewModelsBind(app)
	screenshotBind := NewScreenshotBind(&app.screenshot)
	llmBind := NewLLMBind(app)
	agentBind := NewAgentBind(&app.agent)
	settingsBind := NewSettingsBind(&app.settings)
	overlayBind := NewOverlayBind(&app.overlay)
	hotkeysBind := NewHotkeysBind(&app.hotkeys)
	permissionsBind := NewPermissionsBind(&app.permissions)

	err := wails.Run(&options.App{
		Title:     "Interagent",
		Width:     1024,
		Height:    768,
		Frameless: true,
		Windows: &windows.Options{
			WebviewIsTransparent: true,
		},
		Mac: &mac.Options{
			WebviewIsTransparent: true,
		},
		Linux: &linux.Options{
			WindowIsTranslucent: true,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup:        app.startup,
		Menu:             newOverlayMenu(app),
		Bind: []interface{}{
			app,
			sessionBind,
			audioBind,
			modelsBind,
			screenshotBind,
			llmBind,
			agentBind,
			settingsBind,
			overlayBind,
			hotkeysBind,
			permissionsBind,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
