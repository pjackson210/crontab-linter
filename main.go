package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	args := os.Args[1:]
	clean := true

	if len(args) == 0 {
		if !lintSource("<stdin>", os.Stdin) {
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
			ok := lintSource(path, f)
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

// lintSource reports every finding for r and returns false if any of them
// is an error (as opposed to a warning), so main can set the exit code.
func lintSource(name string, r io.Reader) bool {
	findings, err := Lint(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cronlint: %s: %v\n", name, err)
		return false
	}

	ok := true
	for _, f := range findings {
		fmt.Printf("%s:%d: %s: %s\n", name, f.Line, f.Severity, f.Message)
		if f.Severity == SeverityError {
			ok = false
		}
	}
	return ok
}
