package main

import (
	"strings"
	"testing"
)

func TestLintValidLine(t *testing.T) {
	findings, err := Lint(strings.NewReader("*/15 0 1,15 * * /usr/bin/backup\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestLintOutOfRangeMinute(t *testing.T) {
	findings, err := Lint(strings.NewReader("60 0 * * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if findings[0].Line != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("unexpected finding: %+v", findings[0])
	}
}

func TestLintDayOfMonthAndWeekBothRestricted(t *testing.T) {
	findings, err := Lint(strings.NewReader("0 0 1 * 1 echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityWarning {
		t.Fatalf("expected a single warning, got %v", findings)
	}
}

func TestLintUnknownMacro(t *testing.T) {
	findings, err := Lint(strings.NewReader("@fortnightly echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("expected a single error, got %v", findings)
	}
}

func TestLintSkipsCommentsBlankLinesAndAssignments(t *testing.T) {
	input := "# nightly backup\n\nMAILTO=admin@example.com\n* * * * * echo hi\n"
	findings, err := Lint(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestLintTooFewFields(t *testing.T) {
	findings, err := Lint(strings.NewReader("* * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("expected a single error, got %v", findings)
	}
}
