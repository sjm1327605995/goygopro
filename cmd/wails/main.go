package main

import (
	"fmt"
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

	wailsApp := application.New(application.Options{
		Name:        "goygopro",
		Description: "YGOPro 3D Client",
		Services: []application.Service{
			application.NewService(app),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(os.DirFS(frontendDir)),
		},
	})

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
