package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeResourceFixture(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "script"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cards.cdb"), []byte("card database fixture"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestFindResourceDirFromBin(t *testing.T) {
	root := t.TempDir()
	writeResourceFixture(t, root)
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0755); err != nil {
		t.Fatal(err)
	}
	// SQLite creates this empty file when the previous launch uses the wrong cwd.
	if err := os.WriteFile(filepath.Join(bin, "cards.cdb"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if got := findResourceDir(bin, bin); got != root {
		t.Fatalf("resource directory = %q, want %q", got, root)
	}
	if got := findResourceDir(t.TempDir(), bin); got != root {
		t.Fatalf("launch from unrelated directory: got %q, want %q", got, root)
	}
}

func TestFindResourceDirPrefersWorkingDirectory(t *testing.T) {
	workingDir, installedDir := t.TempDir(), t.TempDir()
	writeResourceFixture(t, workingDir)
	writeResourceFixture(t, installedDir)
	if got := findResourceDir(workingDir, installedDir); got != workingDir {
		t.Fatalf("resource directory = %q, want %q", got, workingDir)
	}
}

func TestFindResourceDirMissingData(t *testing.T) {
	dir := t.TempDir()
	if got := findResourceDir(dir, dir); got != "" {
		t.Fatalf("missing resources: got %q, want empty path", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "cards.cdb")); !os.IsNotExist(err) {
		t.Fatalf("resource lookup must not create a database: %v", err)
	}
}
