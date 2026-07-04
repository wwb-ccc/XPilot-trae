package main

import (
	"context"
	"embed"
	"log"
	"os"
	"time"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

var appInstance *App

func main() {
	appInstance = NewApp()
	appInstance.ctx = context.Background() // placeholder, will be replaced in OnStartup

	// Create Wails application
	err := wails.Run(&options.App{
		Title:             "XPilot - 应用时间追踪",
		Width:             1200,
		Height:            800,
		MinWidth:          900,
		MinHeight:         600,
		HideWindowOnClose: true,
		BackgroundColour:  options.NewRGB(248, 250, 252),
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		Bind: []interface{}{
			appInstance,
		},
		OnStartup:  appInstance.startup,
		OnShutdown: appInstance.shutdown,
	})

	if err != nil {
		log.Fatalf("Wails app error: %v", err)
	}
}

// onSystrayReady is called when systray is ready
func onSystrayReady() {
	systray.SetTitle("XPilot")
	systray.SetTooltip("XPilot - 应用时间追踪")

	
mShow := systray.AddMenuItem("打开 XPilot", "显示窗口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "完全退出程序")

	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				if appInstance != nil {
					appInstance.ShowWindow()
				}
			case <-mQuit.ClickedCh:
				systray.Quit()
				os.Exit(0)
			}
		}
	}()

	// Auto-show window on first launch
	go func() {
		time.Sleep(800 * time.Millisecond)
		if appInstance != nil {
			appInstance.ShowWindow()
		}
	}()
}

// onSystrayExit is called when systray exits
func onSystrayExit() {
	if appInstance != nil {
		appInstance.shutdown(appInstance.ctx)
	}
}