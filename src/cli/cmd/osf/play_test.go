package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the transcripts in tests/play/*.txt")

// TestTranscripts replays every transcript in tests/play against example/osf:
// the first line is the osf play command (without the project folder), the
// rest its stdout. `go test -update` rewrites them, and fills in a file that
// holds only the command line.
func TestTranscripts(t *testing.T) {
	files, _ := filepath.Glob("../../../../tests/play/*.txt")
	if len(files) == 0 {
		t.Fatal("no transcripts found")
	}
	for _, path := range files {
		t.Run(filepath.ToSlash(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			head, body, _ := strings.Cut(string(raw), "\n")
			if !strings.HasPrefix(head, "# osf play") {
				t.Fatalf("first line must be the command, like \"# osf play -choices 1\"; got %q", head)
			}
			args := strings.Fields(strings.TrimPrefix(head, "# osf"))
			got := playOutput(t, append(args, "../../../../example/osf"))
			if *update {
				if err := os.WriteFile(path, []byte(head+"\n"+got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			if got != body {
				t.Errorf("%s differs from `%s` (run go test -update to rewrite)\n--- got\n%s--- want\n%s", path, head, got, body)
			}
		})
	}
}

// playOutput runs the osf command line with an empty stdin and returns stdout.
func playOutput(t *testing.T, args []string) string {
	t.Helper()
	old := stdin
	stdin = strings.NewReader("")
	defer func() { stdin = old }()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	if code > 1 {
		t.Fatalf("osf %s: exit %d\n%s", strings.Join(args, " "), code, stderr.String())
	}
	return stdout.String()
}

func TestPlayFlags(t *testing.T) {
	dir := osfProject(t, map[string]string{"s.osf": baseStory + `
Dialogue d {
  Node a { text: "Pick" Option o1 { text: "Go" next: b } Option o2 { text: "Stop" } }
  Node b { text: "Done" }
}`})
	tests := []struct {
		name  string
		args  []string
		stdin string
		code  int
		out   string
		err   string
	}{
		{"out of range", []string{"play", "-choices", "3", dir}, "", 1, "> 3\n", "choice 3 is out of range (1-2)"},
		{"left over", []string{"play", "-choices", "2,1", dir}, "", 1, "== End\n", "1 choice left after the play ended"},
		{"runs out", []string{"play", "-choices", "1", "-dialogue", "d", dir}, "", 0, "== End\n", ""},
		{"interactive", []string{"play", dir}, "x\n1\n", 0, "> > Done\n== End\n", "choose a number between 1 and 2"},
		{"interactive quit", []string{"play", dir}, "q\n", 0, "> (stopped: 2 options pending)\n", ""},
		{"bad choices", []string{"play", "-choices", "1,x", dir}, "", 2, "", "-choices"},
		{"unknown dialogue", []string{"play", "-dialogue", "nope", dir}, "", 1, "", "Dialogue nope does not exist"},
		{"unknown locale", []string{"play", "-locale", "Xyz", dir}, "", 1, "", `unknown locale "Xyz"`},
		{"coverage clean", []string{"play", "-coverage", dir}, "", 0, "== No findings\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := stdin
			stdin = strings.NewReader(tt.stdin)
			defer func() { stdin = old }()
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.code || !strings.Contains(stdout.String(), tt.out) || !strings.Contains(stderr.String(), tt.err) {
				t.Errorf("exit %d, want %d; stdout:\n%s\nstderr:\n%s", code, tt.code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestPlayRefusesInvalidProject(t *testing.T) {
	dir := osfProject(t, map[string]string{"s.osf": baseStory + `Dialogue d { Node a { text: "x" next: nowhere } }`})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"play", "-choices", "1", dir}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "Nothing was played") || stdout.Len() != 0 {
		t.Errorf("exit %d; stdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
}

// An option with `requires: { ... } or { ... }` shows when one group is met.
func TestPlayRequiresOr(t *testing.T) {
	story := func(gold, god string) string {
		return baseStory + `
Variable gold: Number = ` + gold + `
Variable god_mode: Boolean = ` + god + `
Dialogue d {
  Node a {
    text: "Shop"
    Option buy { text: "Buy" requires: { gold >= 3, gold <= 20 } or { god_mode: true } }
    Option leave { text: "Leave" }
  }
}`
	}
	tests := []struct {
		name, gold, god string
		shown           bool
	}{
		{"first group met", "5", "false", true},
		{"second group met", "0", "true", true},
		{"first group half met", "50", "false", false},
		{"none met", "1", "false", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := osfProject(t, map[string]string{"s.osf": story(tt.gold, tt.god)})
			out := playOutput(t, []string{"play", dir})
			if got := strings.Contains(out, "Buy"); got != tt.shown {
				t.Errorf("Buy shown = %v, want %v\n%s", got, tt.shown, out)
			}
		})
	}
}
