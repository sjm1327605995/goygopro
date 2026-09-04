package main

import "testing"

// TestExtract runs the tarball extraction so it can be triggered via
// `go test ./cmd/extractscripts` without executing the built binary.
func TestExtract(t *testing.T) {
	if err := run(); err != nil {
		t.Fatal(err)
	}
}
