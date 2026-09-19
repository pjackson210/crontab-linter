package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	seconds := flag.Bool("seconds", false, "parse schedules using the 6-field seconds dialect (second minute hour day month weekday)")
	jsonOutput := flag.Bool("json", false, "print findings as newline-delimited JSON instead of plain text")
	flag.Parse()
	args := flag.Args()

	lint := Lint
	if *seconds {
		lint = LintSeconds
	}

	clean := true

	if len(args) == 0 {
		if !lintSource("<stdin>", os.Stdin, lint, *jsonOutput) {
			clean = false
		}
	} else {
		for _, path := range args {
			f, err := os.Open(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cronlint: %v\n", err)
				clean = false
				continue
			}
			ok := lintSource(path, f, lint, *jsonOutput)
			f.Close()
			if !ok {
				clean = false
			}
		}
	}

	if !clean {
		os.Exit(1)
	}
}

// jsonFinding is the wire format for -json output. It's a separate type from
// Finding rather than adding struct tags there, since Finding has no reason
// to know about its source file - lintSource is the only place that does.
type jsonFinding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// lintSource reports every finding for r and returns false if any of them
// is an error (as opposed to a warning), so main can set the exit code.
func lintSource(name string, r io.Reader, lint func(io.Reader) ([]Finding, error), asJSON bool) bool {
	findings, err := lint(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cronlint: %s: %v\n", name, err)
		return false
	}

	enc := json.NewEncoder(os.Stdout)

	ok := true
	for _, f := range findings {
		if asJSON {
			enc.Encode(jsonFinding{
				File:     name,
				Line:     f.Line,
				Severity: f.Severity.String(),
				Message:  f.Message,
			})
		} else {
			fmt.Printf("%s:%d: %s: %s\n", name, f.Line, f.Severity, f.Message)
		}
		if f.Severity == SeverityError {
			ok = false
		}
	}
	return ok
}
