package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// findFrontendDir locates the frontend/ directory, searching the current
// working directory and the directory of the running executable (walking up
// to a few levels so it works both from the repo root and from a packaged
// binary's install location).
func findFrontendDir() (string, error) {
	// The React frontend is built with Vite into frontend/dist; in dev mode
	// Wails proxies to the Vite dev server, so only the built assets need to be
	// served here. Prefer the built directory and fall back to the source
	// directory so the app still starts against an un-built checkout.
	var candidates []string
	candidates = append(candidates, "frontend/dist", "frontend")

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for i := 0; i < 4; i++ {
			candidates = append(candidates, filepath.Join(dir, "frontend", "dist"), filepath.Join(dir, "frontend"))
			dir = filepath.Dir(dir)
		}
	}

	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "index.html")); err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("frontend directory not found (searched: %v)", candidates)
}

func main() {
	frontendDir, err := findFrontendDir()
	if err != nil {
		panic(err)
	}

	app := NewApp()

	opts := application.Options{
		Name:        "goygopro",
		Description: "YGOPro 3D Client",
		Services: []application.Service{
			application.NewService(app),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(os.DirFS(frontendDir)),
		},
	}
	// 调试钩子：YGO_CDP_PORT=9222 时给 WebView2 加 CDP 远程调试端口
	// （自动化测试/无头诊断用，正常启动不受影响）。
	if port := os.Getenv("YGO_CDP_PORT"); port != "" {
		opts.Windows.AdditionalBrowserArgs = append(opts.Windows.AdditionalBrowserArgs, "--remote-debugging-port="+port)
		opts.LogLevel = slog.LevelDebug // 诊断：打印 "Registering bound method: fqn=..." 等
	}
	// 调试钩子：YGO_USER_DATA 指定 WebView2 用户数据目录（默认 %APPDATA%\goygopro）。
	// 同机多开测试时必须各用各的目录，否则 Chromium profile 单例锁会让后启动的实例失效。
	if dir := os.Getenv("YGO_USER_DATA"); dir != "" {
		opts.Windows.WebviewUserDataPath = dir
	}

	wailsApp := application.New(opts)

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "YGOPro 3D Client",
		Width:     1280,
		Height:    720,
		MinWidth:  1024,
		MinHeight: 640,
	})

	if err := wailsApp.Run(); err != nil {
		panic(err)
	}
}
