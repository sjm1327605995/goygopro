package main

import (
	"os"
	"path/filepath"
)

// findResourceDir locates the runtime data shared by the client and server.
// Requiring both the card database and script directory avoids selecting an
// empty database SQLite may have created after a failed launch from bin/.
func findResourceDir(workingDir, executableDir string) string {
	candidates := []string{workingDir}
	for dir, i := executableDir, 0; i < 4; i++ {
		candidates = append(candidates, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	for _, dir := range candidates {
		db, err := os.Stat(filepath.Join(dir, "cards.cdb"))
		if err != nil || !db.Mode().IsRegular() || db.Size() == 0 {
			continue
		}
		scripts, err := os.Stat(filepath.Join(dir, "script"))
		if err == nil && scripts.IsDir() {
			return dir
		}
	}
	return ""
}

// useResourceDir keeps existing relative paths (DLL, scripts, config, decks,
// and replays) together when the executable is launched from a build directory.
func useResourceDir() error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if dir := findResourceDir(workingDir, filepath.Dir(executable)); dir != "" {
		return os.Chdir(dir)
	}
	return nil
}
