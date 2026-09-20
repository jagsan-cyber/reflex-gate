package main

import (
	"context"
	"embed"
	"os"
	"os/signal"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	// Handle OS interrupt signals for graceful child process termination
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		_ = app.StopServer()
		os.Exit(0)
	}()

	err := wails.Run(&options.App{
		Title:             "ReflexGate (Qwen2.5-Coder-1.5B Fast AI Gateway)",
		Width:             1080,
		Height:            840,
		MinWidth:          960,
		MinHeight:         700,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 12, B: 16, A: 255},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			initTray(app)
		},
		OnShutdown: func(ctx context.Context) {
			app.shutdown(ctx)
		},
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			Theme:                windows.Dark,
			BackdropType:         windows.Mica,
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
