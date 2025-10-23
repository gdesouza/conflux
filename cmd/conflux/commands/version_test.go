package commands

import (
	"bytes"
	"os"
	"testing"

	"conflux/pkg/version"
)

func TestRunVersion(t *testing.T) {
	old := shortVersion
	defer func() { shortVersion = old }()

	// Capture stdout
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	// full
	shortVersion = false
	version.Version = "1.2.3"
	version.GitCommit = "abc"
	version.BuildDate = "2025-10-23"
	runVersion(nil, nil)
	w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !bytes.Contains(buf.Bytes(), []byte("conflux version")) {
		t.Fatalf("expected full version output, got %s", buf.String())
	}

	// short
	r, w, _ = os.Pipe()
	os.Stdout = w
	shortVersion = true
	version.Version = "9.9.9"
	runVersion(nil, nil)
	w.Close()
	buf.Reset()
	_, _ = buf.ReadFrom(r)
	if buf.String() != "9.9.9\n" {
		t.Fatalf("expected short version, got %s", buf.String())
	}
}
