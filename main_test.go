package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	orig := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = orig
	w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	return string(out)
}

func TestLintSourceJSONOutput(t *testing.T) {
	out := captureStdout(t, func() {
		ok := lintSource("crontab", strings.NewReader("60 0 * * * echo hi\n"), Lint, true)
		if ok {
			t.Fatalf("expected lintSource to report a failure")
		}
	})

	line := strings.TrimSpace(out)
	var got jsonFinding
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("output %q is not valid JSON: %v", line, err)
	}

	want := jsonFinding{
		File:     "crontab",
		Line:     1,
		Severity: "error",
		Message:  `minute field "60": "60" is not a valid minute`,
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestLintSourceJSONOutputNoFindings(t *testing.T) {
	out := captureStdout(t, func() {
		ok := lintSource("crontab", strings.NewReader("0 0 * * * echo hi\n"), Lint, true)
		if !ok {
			t.Fatalf("expected lintSource to report success")
		}
	})

	if out != "" {
		t.Fatalf("expected no output, got %q", out)
	}
}

func TestLintSourcePlainTextOutput(t *testing.T) {
	out := captureStdout(t, func() {
		lintSource("crontab", strings.NewReader("60 0 * * * echo hi\n"), Lint, false)
	})

	want := `crontab:1: error: minute field "60": "60" is not a valid minute` + "\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}
