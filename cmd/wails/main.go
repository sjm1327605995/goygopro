package main

import (
	"embed"
	"fmt"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	fmt.Println("Wails YGOPro 3D Client backend initialized.")
	_ = app
	// In standard Wails application:
	// wails.Run(&options.App{
	//    Title: "YGOPro 3D Client",
	//    Width: 1280,
	//    Height: 720,
	//    AssetServer: &assetserver.Options{
	//        Assets: assets,
	//    },
	//    OnStartup: app.Startup,
	//    OnShutdown: app.Shutdown,
	//    Bind: []interface{}{app},
	// })
}
