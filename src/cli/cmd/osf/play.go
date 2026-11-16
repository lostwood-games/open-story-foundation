package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/diag"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/play"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/project"
)

// stdin is where interactive play reads choices; tests replace it.
var stdin io.Reader = os.Stdin

func runPlay(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("play", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dialogue := fs.String("dialogue", "", "dialogue id to start at (default: the Story's start)")
	locale := fs.String("locale", "", "locale id to show texts in (default: the source language)")
	choices := fs.String("choices", "", "comma-separated 1-based choices to play without asking, e.g. 1,2,1")
	coverage := fs.Bool("coverage", false, "explore every reachable path and report problems instead of playing")
	fs.Usage = func() {
		fmt.Fprint(stderr, `Usage: osf play [-dialogue id] [-locale X] [-choices 1,2,1] [dir]
       osf play -coverage [-dialogue id] [dir]

Plays a dialogue of the project in dir (default: the current folder) in the
terminal. Each node is printed as "Speaker: text" (narration as plain text),
variable changes as "  [set name = value]", options as numbered lines; type
a number at the "> " prompt, or an empty line to stop.

Options:
  -dialogue id   Start at this dialogue. Default: the Story's start, else the
                 first dialogue.
  -locale X      Show texts in locale X, falling back to the source text.
  -choices list  Play these choices without asking and print the transcript.
                 Exits 0 when the play ends or the choices run out, 1 when a
                 choice is out of range or left over after the end.
  -coverage      Explore every reachable path from the start dialogue and
                 report dead ends, unreachable options and loops with no
                 exit. Exits 1 when there are findings.
`)
	}
	if err := parseArgs(fs, args); err != nil {
		return flagExit(err)
	}
	dir, ok := oneDir(fs, stderr)
	if !ok {
		return 2
	}
	fixed, err := parseChoices(*choices)
	if err != nil {
		fmt.Fprintf(stderr, "osf: -choices: %v\n", err)
		return 2
	}
	p, diags := project.Analyze(dir)
	printDiags(stderr, diags)
	if errors, warnings := diag.Count(diags); errors > 0 || p == nil {
		fmt.Fprintf(stderr, "\n%s, %s.\nNothing was played.\n", plural(errors, "error"), plural(warnings, "warning"))
		return 1
	}
	if *coverage {
		r, err := play.Coverage(p, *dialogue)
		if err != nil {
			fmt.Fprintf(stderr, "osf: %v\n", err)
			return 1
		}
		r.Write(stdout)
		if len(r.Findings) > 0 {
			return 1
		}
		return 0
	}
	s, err := play.New(p, play.Options{Dialogue: *dialogue, Locale: *locale})
	if err != nil {
		fmt.Fprintf(stderr, "osf: %v\n", err)
		return 1
	}
	t := &transcript{w: stdout, stderr: stderr, fixed: fixed, interactive: *choices == ""}
	return t.play(s)
}

func parseChoices(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	var out []int
	for _, part := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 1 {
			return nil, fmt.Errorf("%q is not a choice number", part)
		}
		out = append(out, n)
	}
	return out, nil
}

// transcript prints a play in the format of tests/play/README.md.
type transcript struct {
	w, stderr   io.Writer
	fixed       []int
	interactive bool
	dialogue    string // last dialogue header printed
	in          *bufio.Scanner
}

func (t *transcript) play(s *play.Session) int {
	st, err := s.Start()
	t.lines(st.Lines)
	if err != nil {
		fmt.Fprintf(t.stderr, "osf: %v\n", err)
		return 1
	}
	for !st.Finished {
		for _, o := range st.Options {
			fmt.Fprintf(t.w, "  %d. %s\n", o.Index, o.Text)
		}
		n, ok := t.next(len(st.Options))
		if !ok {
			fmt.Fprintf(t.w, "(stopped: %s pending)\n", plural(len(st.Options), "option"))
			return 0
		}
		if n < 1 || n > len(st.Options) {
			if t.interactive {
				fmt.Fprintf(t.stderr, "osf: choose a number between 1 and %d\n", len(st.Options))
				continue
			}
			fmt.Fprintf(t.stderr, "osf: choice %d is out of range (1-%d)\n", n, len(st.Options))
			return 1
		}
		st, err = s.Choose(n)
		t.changes(st.Chosen.Set)
		t.lines(st.Lines)
		if err != nil {
			fmt.Fprintf(t.stderr, "osf: %v\n", err)
			return 1
		}
	}
	fmt.Fprintln(t.w, "== End")
	if len(t.fixed) > 0 {
		fmt.Fprintf(t.stderr, "osf: %s left after the play ended\n", plural(len(t.fixed), "choice"))
		return 1
	}
	return 0
}

// next returns the next choice: from -choices, or read from stdin after a
// prompt. ok is false when there are no more.
func (t *transcript) next(count int) (n int, ok bool) {
	if !t.interactive {
		if len(t.fixed) == 0 {
			return 0, false
		}
		n, t.fixed = t.fixed[0], t.fixed[1:]
		fmt.Fprintf(t.w, "> %d\n", n)
		return n, true
	}
	if t.in == nil {
		t.in = bufio.NewScanner(stdin)
	}
	for {
		fmt.Fprint(t.w, "> ")
		if !t.in.Scan() {
			fmt.Fprintln(t.w)
			return 0, false
		}
		text := strings.TrimSpace(t.in.Text())
		if text == "" || text == "q" || text == "quit" {
			return 0, false
		}
		n, err := strconv.Atoi(text)
		if err != nil {
			fmt.Fprintf(t.stderr, "osf: choose a number between 1 and %d\n", count)
			continue
		}
		return n, true
	}
}

func (t *transcript) lines(lines []play.Line) {
	for _, l := range lines {
		if l.Dialogue != t.dialogue {
			t.dialogue = l.Dialogue
			fmt.Fprintf(t.w, "== Dialogue %s\n", l.Dialogue)
		}
		t.changes(l.Set)
		text := strings.TrimRight(l.Text, "\n")
		if l.Speaker != "" {
			fmt.Fprintf(t.w, "%s: %s\n", l.Speaker, text)
		} else {
			fmt.Fprintln(t.w, text)
		}
	}
}

func (t *transcript) changes(set []play.Change) {
	for _, c := range set {
		fmt.Fprintf(t.w, "  [set %s = %s]\n", c.Name, play.FormatValue(c.Value))
	}
}
