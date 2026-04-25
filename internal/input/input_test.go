package input

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPickEditor_EnvPrecedence(t *testing.T) {
	t.Setenv("EDITOR", "")
	t.Setenv("VISUAL", "")

	t.Setenv("EDITOR", "my-editor")
	t.Setenv("VISUAL", "my-visual")
	if got := pickEditor(); got != "my-editor" {
		t.Errorf("EDITOR should win, got %q", got)
	}

	t.Setenv("EDITOR", "")
	if got := pickEditor(); got != "my-visual" {
		t.Errorf("VISUAL should be used when EDITOR is empty, got %q", got)
	}
}

func TestReadFromEditor_WritesAndReadsTempfile(t *testing.T) {
	// Build a tiny shell editor that writes a known string to its first arg.
	dir := t.TempDir()
	editorScript := filepath.Join(dir, "fake-editor.sh")
	expected := "ってかこれ見てほしいんだけど\n複数行ある"
	if err := os.WriteFile(editorScript, []byte("#!/bin/sh\nprintf '%s' \""+expected+"\" > \"$1\"\n"), 0o755); err != nil { // #nosec G306 — intentional 0755 for test exec
		t.Fatal(err)
	}

	t.Setenv("EDITOR", editorScript)

	got, err := readFromEditor()
	if err != nil {
		t.Fatalf("readFromEditor: %v", err)
	}
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestReadFromEditor_EmptyContentRejected(t *testing.T) {
	dir := t.TempDir()
	editorScript := filepath.Join(dir, "no-op-editor.sh")
	// Editor that does nothing — leaves the file empty.
	if err := os.WriteFile(editorScript, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil { // #nosec G306
		t.Fatal(err)
	}

	t.Setenv("EDITOR", editorScript)

	_, err := readFromEditor()
	if err == nil || !strings.Contains(err.Error(), "empty content") {
		t.Errorf("expected empty-content error, got %v", err)
	}
}

func TestReadFromEditor_EditorExitErrorPropagates(t *testing.T) {
	dir := t.TempDir()
	editorScript := filepath.Join(dir, "failing-editor.sh")
	if err := os.WriteFile(editorScript, []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil { // #nosec G306
		t.Fatal(err)
	}

	t.Setenv("EDITOR", editorScript)

	_, err := readFromEditor()
	if err == nil || !strings.Contains(err.Error(), "exited with error") {
		t.Errorf("expected editor-exit error, got %v", err)
	}
}

func TestReadInteractiveStdin_PipedFallthrough(t *testing.T) {
	// When stdin is piped (which is what os.Pipe gives us in a test),
	// readInteractiveStdin must fall back to io.ReadAll without printing
	// the "Enter Japanese text..." prompt.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	expected := "改行\n含む\nテキスト"
	if _, err := w.Write([]byte(expected)); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	got, err := readInteractiveStdin()
	if err != nil {
		t.Fatalf("readInteractiveStdin: %v", err)
	}
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestResolve_EditorWinsOverArgs(t *testing.T) {
	dir := t.TempDir()
	editorScript := filepath.Join(dir, "fake-editor.sh")
	expected := "from editor"
	if err := os.WriteFile(editorScript, []byte("#!/bin/sh\nprintf '%s' \""+expected+"\" > \"$1\"\n"), 0o755); err != nil { // #nosec G306
		t.Fatal(err)
	}
	t.Setenv("EDITOR", editorScript)

	got, err := Resolve(Source{
		Args:      []string{"this", "should", "be", "ignored"},
		UseEditor: true,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestResolve_InteractiveWithPipedStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	expected := "改行ありの\n対話モード入力"
	if _, err := w.Write([]byte(expected + "\n")); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	got, err := Resolve(Source{UseInteractive: true})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}
