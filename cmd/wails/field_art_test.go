package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFieldArtUsesDedicatedFiles(t *testing.T) {
	withConfigDir(t, "")
	a := NewApp()
	write := func(dir, file, content string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("pics", "1001.jpg", "card face")
	if got := a.GetFieldImage(1001); got != "" {
		t.Fatalf("card face used as field: %q", got)
	}
	write("pics/field", "1001.jpg", "local field")
	local := a.GetFieldImage(1001)
	if !strings.HasPrefix(local, "data:") {
		t.Fatal("local field not loaded")
	}
	write("expansions/pics/field", "1001.png", "expansion field")
	if got := a.GetFieldImage(1001); got == local || got == "" {
		t.Fatal("expansion field must take priority")
	}
	if got := a.GetFieldImage(0); got != "" {
		t.Fatal("code zero should not have field art")
	}
}
