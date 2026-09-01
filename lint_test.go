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

// Repeated values within a field aren't flagged yet - that's a separate
// roadmap item (warn on duplicate/overlapping values). This test exists so
// that when that check lands, it fails here rather than going unnoticed.
func TestLintDuplicateValuesNotYetFlagged(t *testing.T) {
	findings, err := Lint(strings.NewReader("0,0,0 0 * * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestLintTrailingCommaInField(t *testing.T) {
	findings, err := Lint(strings.NewReader("0, 0 * * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("expected a single error, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, "empty value in list") {
		t.Fatalf("expected an empty value complaint, got %q", findings[0].Message)
	}
}

func TestLintDoubleCommaInField(t *testing.T) {
	findings, err := Lint(strings.NewReader("1,,3 0 * * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("expected a single error, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, "empty value in list") {
		t.Fatalf("expected an empty value complaint, got %q", findings[0].Message)
	}
}

// A bare negative number is parsed as a range with an empty start (the
// leading "-" is read as a range separator, not a sign), so it's rejected
// via the range path rather than the plain out-of-bounds path. Either way
// it's still an error, which is what matters to a caller.
func TestLintNegativeNumberField(t *testing.T) {
	findings, err := Lint(strings.NewReader("-5 0 * * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("expected a single error, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, "minute field") {
		t.Fatalf("expected a minute field complaint, got %q", findings[0].Message)
	}
}

func TestLintNegativeStepValue(t *testing.T) {
	findings, err := Lint(strings.NewReader("*/-5 0 * * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("expected a single error, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, "must be a positive integer") {
		t.Fatalf("expected a step value complaint, got %q", findings[0].Message)
	}
}
