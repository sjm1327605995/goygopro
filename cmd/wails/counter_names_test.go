package main

import (
	"os"
	"testing"
)

func TestCounterNamesUseStringConfig(t *testing.T) {
	withConfigDir(t, "")
	if err := os.WriteFile("strings.conf", []byte("!system 1 unrelated\n!counter 0x1 魔力指示物\n!counter\t0x1002\tCustom Counter\n"), 0644); err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	if got := a.GetCounterName(1); got != "魔力指示物" {
		t.Fatalf("name=%q", got)
	}
	if got := a.GetCounterName(0x1002); got != "Custom Counter" {
		t.Fatalf("name=%q", got)
	}
	if got := a.GetCounterName(999); got != "？？？" {
		t.Fatalf("unknown=%q", got)
	}
}
