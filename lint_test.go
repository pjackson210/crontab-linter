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

func TestLintDayOfMonthLastDay(t *testing.T) {
	findings, err := Lint(strings.NewReader("0 0 L * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestLintDayOfMonthNearestWeekday(t *testing.T) {
	findings, err := Lint(strings.NewReader("0 0 15W * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestLintDayOfMonthLastWeekday(t *testing.T) {
	findings, err := Lint(strings.NewReader("0 0 LW * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestLintDayOfMonthInvalidNearestWeekday(t *testing.T) {
	findings, err := Lint(strings.NewReader("0 0 35W * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("expected a single error, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, "W expression") {
		t.Fatalf("expected a W expression complaint, got %q", findings[0].Message)
	}
}

func TestLintDayOfWeekLastOccurrence(t *testing.T) {
	findings, err := Lint(strings.NewReader("0 0 * * 5L echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestLintDayOfWeekNthOccurrence(t *testing.T) {
	findings, err := Lint(strings.NewReader("0 0 * * MON#2 echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestLintDayOfWeekInvalidNthOccurrence(t *testing.T) {
	findings, err := Lint(strings.NewReader("0 0 * * MON#6 echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("expected a single error, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, "# expression") {
		t.Fatalf("expected a # expression complaint, got %q", findings[0].Message)
	}
}

func TestLintDayOfWeekInvalidLastOccurrence(t *testing.T) {
	findings, err := Lint(strings.NewReader("0 0 * * 9L echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("expected a single error, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, "L expression") {
		t.Fatalf("expected an L expression complaint, got %q", findings[0].Message)
	}
}

func TestLintDuplicateValueInField(t *testing.T) {
	findings, err := Lint(strings.NewReader("0,0,0 0 * * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Severity != SeverityWarning {
			t.Fatalf("expected a warning, got %+v", f)
		}
		if !strings.Contains(f.Message, "already covered by") {
			t.Fatalf("expected an overlap complaint, got %q", f.Message)
		}
	}
}

func TestLintOverlappingRanges(t *testing.T) {
	findings, err := Lint(strings.NewReader("0 0 1-5,3-8 * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityWarning {
		t.Fatalf("expected a single warning, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, `value 3 in "3-8" is already covered by "1-5"`) {
		t.Fatalf("unexpected message: %q", findings[0].Message)
	}
}

func TestLintOverlapWithStar(t *testing.T) {
	findings, err := Lint(strings.NewReader("*,5 0 * * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityWarning {
		t.Fatalf("expected a single warning, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, `value 5 in "5" is already covered by "*"`) {
		t.Fatalf("unexpected message: %q", findings[0].Message)
	}
}

func TestLintOverlappingSteps(t *testing.T) {
	findings, err := Lint(strings.NewReader("*/15,*/30 0 * * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityWarning {
		t.Fatalf("expected a single warning, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, `value 0 in "*/30" is already covered by "*/15"`) {
		t.Fatalf("unexpected message: %q", findings[0].Message)
	}
}

// L/W/# extensions don't reduce to a plain integer set, so they're exempt
// from overlap checking even when repeated.
func TestLintNoOverlapForExtensions(t *testing.T) {
	findings, err := Lint(strings.NewReader("0 0 * * MON#2,MON#2 echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestLintNoOverlapForDistinctValues(t *testing.T) {
	findings, err := Lint(strings.NewReader("*/15 0 1,15 * * /usr/bin/backup\n"))
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

func TestLintSecondsValidLine(t *testing.T) {
	findings, err := LintSeconds(strings.NewReader("*/15 0 0 1,15 * * /usr/bin/backup\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestLintSecondsOutOfRangeSecond(t *testing.T) {
	findings, err := LintSeconds(strings.NewReader("60 0 0 * * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("expected a single error, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, "second field") {
		t.Fatalf("expected a second field complaint, got %q", findings[0].Message)
	}
}

func TestLintSecondsTooFewFields(t *testing.T) {
	findings, err := LintSeconds(strings.NewReader("0 0 * * * echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("expected a single error, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, "expected 6 schedule fields") {
		t.Fatalf("expected a field count complaint, got %q", findings[0].Message)
	}
}

func TestLintSecondsDayOfMonthAndWeekBothRestricted(t *testing.T) {
	findings, err := LintSeconds(strings.NewReader("0 0 0 1 * 1 echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityWarning {
		t.Fatalf("expected a single warning, got %v", findings)
	}
}

func TestLintSecondsUnknownMacro(t *testing.T) {
	findings, err := LintSeconds(strings.NewReader("@fortnightly echo hi\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 || findings[0].Severity != SeverityError {
		t.Fatalf("expected a single error, got %v", findings)
	}
}
