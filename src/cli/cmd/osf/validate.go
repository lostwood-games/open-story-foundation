package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/diag"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/project"
)

func printDiags(w io.Writer, diags []diag.Diagnostic) {
	for _, d := range diags {
		fmt.Fprintln(w, d)
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "")
	fs.Usage = func() {
		fmt.Fprint(stderr, `Usage: osf validate [-format text|json] [dir]

Checks the project in dir (default: the current folder) against the Open Story
spec: structure, ids, references, variable values and translations. Every
problem is reported with its file, line, column, severity and rule code
(docs/diagnostics.md), and a close id is suggested for a misspelled reference.

dir is read as .osf source if it holds any .osf file, otherwise as OSF YAML
or JSON resources.

Options:
  -format json   Print the diagnostics as JSON on stdout instead of text on
                 stderr: {"diagnostics": [...], "summary": {...}}.
`)
	}
	if err := parseArgs(fs, args); err != nil {
		return flagExit(err)
	}
	dir, ok := oneDir(fs, stderr)
	if !ok {
		return 2
	}
	switch *format {
	case "json":
		p, diags := project.Analyze(dir)
		out, err := diag.Marshal(diags)
		if err != nil {
			fmt.Fprintf(stderr, "osf: %v\n", err)
			return 1
		}
		stdout.Write(out)
		if errors, _ := diag.Count(diags); errors > 0 || p == nil {
			return 1
		}
		return 0
	case "text":
	default:
		fmt.Fprintf(stderr, "osf: unknown format %q; use text or json\n", *format)
		return 2
	}
	p, diags := project.Analyze(dir)
	printDiags(stderr, diags)
	if errors, warnings := diag.Count(diags); errors > 0 || p == nil {
		fmt.Fprintf(stderr, "\n%s, %s.\n", plural(errors, "error"), plural(warnings, "warning"))
		return 1
	}
	fmt.Fprintf(stdout, "Success! The project is valid (%s).\n", summary(p))
	return 0
}

func summary(p *osf.Project) string {
	parts := []string{plural(len(p.Dialogues), "dialogue"), plural(len(p.Characters), "character")}
	if len(p.Documents) > 0 {
		parts = append(parts, plural(len(p.Documents), "document"))
	}
	parts = append(parts, plural(len(p.Variables), "variable"), plural(len(p.Texts()), "text"))
	return strings.Join(parts, ", ")
}

func oneDir(fs *flag.FlagSet, stderr io.Writer) (string, bool) {
	switch fs.NArg() {
	case 0:
		return ".", true
	case 1:
		return fs.Arg(0), true
	}
	fmt.Fprintln(stderr, "osf: expected at most one folder")
	return "", false
}

// flagExit is the exit status for a flag parsing error: 0 for -help.
func flagExit(err error) int {
	if err == flag.ErrHelp {
		return 0
	}
	return 2
}

// parseArgs parses flags that come before or after the positional arguments.
func parseArgs(fs *flag.FlagSet, args []string) error {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return err
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
	return fs.Parse(append([]string{"--"}, positional...))
}
