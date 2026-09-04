package main

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Severity distinguishes findings that make a schedule wrong (Error) from
// findings that are legal but probably not what the author meant (Warning).
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "unknown"
	}
}

// Finding is one problem reported against a single line of input.
type Finding struct {
	Line     int
	Severity Severity
	Message  string
}

type fieldDef struct {
	name  string
	min   int
	max   int
	names map[string]int
}

func (d fieldDef) resolve(s string) (int, bool) {
	if d.names != nil {
		if v, ok := d.names[strings.ToLower(s)]; ok {
			return v, true
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	if n < d.min || n > d.max {
		return n, false
	}
	return n, true
}

var monthNames = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var weekdayNames = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// fieldDefs is ordered minute, hour, day-of-month, month, day-of-week,
// matching the standard 5-field crontab layout.
var fieldDefs = [5]fieldDef{
	{name: "minute", min: 0, max: 59},
	{name: "hour", min: 0, max: 23},
	{name: "day of month", min: 1, max: 31},
	{name: "month", min: 1, max: 12, names: monthNames},
	{name: "day of week", min: 0, max: 7, names: weekdayNames}, // 0 and 7 both mean Sunday
}

// fieldDefsSeconds is the same layout with a leading seconds field, matching
// the 6-field dialect used by some schedulers (Quartz-derived tools, several
// Node and Python cron libraries).
var fieldDefsSeconds = [6]fieldDef{
	{name: "second", min: 0, max: 59},
	{name: "minute", min: 0, max: 59},
	{name: "hour", min: 0, max: 23},
	{name: "day of month", min: 1, max: 31},
	{name: "month", min: 1, max: 12, names: monthNames},
	{name: "day of week", min: 0, max: 7, names: weekdayNames},
}

var macros = map[string]bool{
	"@yearly": true, "@annually": true, "@monthly": true, "@weekly": true,
	"@daily": true, "@midnight": true, "@hourly": true, "@reboot": true,
}

// envAssignment matches crontab lines like "PATH=/usr/bin" or "MAILTO=me@example.com",
// which are valid crontab syntax but not schedules.
var envAssignment = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\s*=`)

// Lint reads standard 5-field cron schedule lines from r and returns every
// finding, in the order the lines appear. It reads one line at a time via
// bufio.Scanner, so input size is bounded by the longest single line, not
// the total input - a multi-gigabyte crontab-style file costs no more
// memory than a small one.
func Lint(r io.Reader) ([]Finding, error) {
	return lint(r, fieldDefs[:])
}

// LintSeconds is Lint for the 6-field "with seconds" dialect: second,
// minute, hour, day of month, month, day of week.
func LintSeconds(r io.Reader) ([]Finding, error) {
	return lint(r, fieldDefsSeconds[:])
}

func lint(r io.Reader, defs []fieldDef) ([]Finding, error) {
	var findings []Finding
	scanner := bufio.NewScanner(r)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		findings = append(findings, checkLine(lineNo, scanner.Text(), defs)...)
	}
	if err := scanner.Err(); err != nil {
		return findings, err
	}
	return findings, nil
}

func checkLine(lineNo int, raw string, defs []fieldDef) []Finding {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil
	}
	if envAssignment.MatchString(line) {
		return nil
	}

	fields := strings.Fields(line)

	if strings.HasPrefix(fields[0], "@") {
		if !macros[strings.ToLower(fields[0])] {
			return []Finding{{
				Line:     lineNo,
				Severity: SeverityError,
				Message:  fmt.Sprintf("unknown schedule macro %q", fields[0]),
			}}
		}
		return nil
	}

	if len(fields) < len(defs) {
		return []Finding{{
			Line:     lineNo,
			Severity: SeverityError,
			Message:  fmt.Sprintf("expected %d schedule fields, found %d", len(defs), len(fields)),
		}}
	}

	var findings []Finding
	for i, def := range defs {
		for _, problem := range checkField(def, fields[i]) {
			findings = append(findings, Finding{
				Line:     lineNo,
				Severity: SeverityError,
				Message:  fmt.Sprintf("%s field %q: %s", def.name, fields[i], problem),
			})
		}
	}

	// Cron's day-of-month/day-of-week interaction is one of the most common
	// sources of surprise: when both are restricted, most implementations
	// fire on either match, not on their intersection. Day of month is
	// always third from last and day of week always last, whether or not a
	// leading seconds field is present.
	domField := fields[len(defs)-3]
	dowField := fields[len(defs)-1]
	if domField != "*" && dowField != "*" {
		findings = append(findings, Finding{
			Line:     lineNo,
			Severity: SeverityWarning,
			Message:  "day of month and day of week are both restricted; most cron implementations run the job when EITHER matches, not when both do",
		})
	}

	return findings
}

func checkField(def fieldDef, raw string) []string {
	var problems []string
	for _, part := range strings.Split(raw, ",") {
		if part == "" {
			problems = append(problems, "empty value in list")
			continue
		}

		base, step, hasStep := strings.Cut(part, "/")
		if hasStep {
			n, err := strconv.Atoi(step)
			if err != nil || n <= 0 {
				problems = append(problems, fmt.Sprintf("step %q must be a positive integer", step))
			}
		}

		if base == "*" {
			continue
		}

		if lo, hi, isRange := strings.Cut(base, "-"); isRange {
			loVal, loOK := def.resolve(lo)
			hiVal, hiOK := def.resolve(hi)
			if !loOK {
				problems = append(problems, fmt.Sprintf("%q is not a valid %s", lo, def.name))
			}
			if !hiOK {
				problems = append(problems, fmt.Sprintf("%q is not a valid %s", hi, def.name))
			}
			if loOK && hiOK && loVal > hiVal {
				problems = append(problems, fmt.Sprintf("range %q has a start greater than its end", part))
			}
		} else if _, ok := def.resolve(base); !ok {
			problems = append(problems, fmt.Sprintf("%q is not a valid %s", base, def.name))
		}
	}
	return problems
}
