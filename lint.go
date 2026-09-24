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
		for _, warning := range checkOverlap(def, fields[i]) {
			findings = append(findings, Finding{
				Line:     lineNo,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("%s field %q: %s", def.name, fields[i], warning),
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

		if problem, handled := checkExtension(def, part); handled {
			if problem != "" {
				problems = append(problems, problem)
			}
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

// checkOverlap flags comma-separated values that repeat or overlap earlier
// ones in the same field, such as "0,0,0" or "1-5,3-8". It's a warning
// rather than an error: the schedule still runs, it just has redundant
// entries that are almost always a typo rather than intentional. Parts that
// don't reduce to a plain value, range, or step (L/W/# extensions, or values
// checkField has already rejected) are silently skipped, since expandField
// can't assign them a meaningful set of values.
func checkOverlap(def fieldDef, raw string) []string {
	var problems []string
	seen := make(map[int]string)
	for _, part := range strings.Split(raw, ",") {
		if part == "" {
			continue
		}
		vals, ok := expandField(def, part)
		if !ok {
			continue
		}
		for _, v := range vals {
			if prev, dup := seen[v]; dup {
				problems = append(problems, fmt.Sprintf("value %d in %q is already covered by %q", v, part, prev))
				break
			}
		}
		for _, v := range vals {
			if _, exists := seen[v]; !exists {
				seen[v] = part
			}
		}
	}
	return problems
}

// expandField returns every value a single comma-separated part denotes -
// e.g. "3" -> [3], "1-3" -> [1,2,3], "*/15" on a minute field ->
// [0,15,30,45] - so checkOverlap can compare parts by the values they cover
// rather than by their literal text. ok is false for anything checkField
// would already reject, and for forms expandField can't reduce to a plain
// integer set (day-of-month/day-of-week's L, W and # extensions).
func expandField(def fieldDef, part string) ([]int, bool) {
	base, stepStr, hasStep := strings.Cut(part, "/")
	step := 1
	if hasStep {
		n, err := strconv.Atoi(stepStr)
		if err != nil || n <= 0 {
			return nil, false
		}
		step = n
	}

	var lo, hi int
	if base == "*" {
		lo, hi = def.min, def.max
	} else if l, h, isRange := strings.Cut(base, "-"); isRange {
		loVal, loOK := def.resolve(l)
		hiVal, hiOK := def.resolve(h)
		if !loOK || !hiOK || loVal > hiVal {
			return nil, false
		}
		lo, hi = loVal, hiVal
	} else {
		v, ok := def.resolve(base)
		if !ok {
			return nil, false
		}
		if !hasStep {
			return []int{v}, true
		}
		// "N/M" with a plain start (not a range) steps from N to the end of
		// the field's range, matching vixie-cron's interpretation.
		lo, hi = v, def.max
	}

	vals := make([]int, 0, (hi-lo)/step+1)
	for v := lo; v <= hi; v += step {
		vals = append(vals, v)
	}
	return vals, true
}

// checkExtension recognizes the vixie-cron/Quartz-derived extensions L, W
// and # that don't fit the plain value/range/step grammar checkField
// otherwise handles: L (last day of month, or last weekday-of-week in the
// month), W (nearest weekday to a given day of month), and # (nth weekday
// of the month, day-of-week field only). handled reports whether part was
// one of these forms at all, independent of whether it was valid - callers
// should fall back to ordinary field checks when handled is false.
func checkExtension(def fieldDef, part string) (problem string, handled bool) {
	switch def.name {
	case "day of month":
		if part == "L" {
			return "", true
		}
		if day, ok := strings.CutSuffix(part, "W"); ok {
			if day == "L" {
				return "", true
			}
			if n, err := strconv.Atoi(day); err != nil || n < 1 || n > 31 {
				return fmt.Sprintf("%q is not a valid day-of-month W expression", part), true
			}
			return "", true
		}
	case "day of week":
		if day, ok := strings.CutSuffix(part, "L"); ok && day != "" {
			if _, ok := def.resolve(day); !ok {
				return fmt.Sprintf("%q is not a valid day-of-week L expression", part), true
			}
			return "", true
		}
		if day, nth, ok := strings.Cut(part, "#"); ok {
			_, dayOK := def.resolve(day)
			n, err := strconv.Atoi(nth)
			if !dayOK || err != nil || n < 1 || n > 5 {
				return fmt.Sprintf("%q is not a valid day-of-week # expression", part), true
			}
			return "", true
		}
	}
	return "", false
}
