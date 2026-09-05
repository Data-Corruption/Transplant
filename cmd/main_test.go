package main

import (
	"io"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/Data-Corruption/Transplant/internal/transplant"
)

func TestWindowsWizardExplainsWSLBeforeAppInitialization(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows entrypoint")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	args, stderr := os.Args, os.Stderr
	defer func() { os.Args = args; os.Stderr = stderr }()
	os.Args = []string{"transplant"}
	os.Stderr = w
	code := runMain()
	w.Close()
	os.Stderr = stderr
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 || !strings.Contains(string(output), transplant.WindowsMessage) {
		t.Fatalf("exit=%d stderr=%s", code, output)
	}
	if strings.Contains(string(output), "layout") || strings.Contains(string(output), "migration") {
		t.Fatalf("initialized the app before rejecting Windows: %s", output)
	}
}
