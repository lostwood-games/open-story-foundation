package diag

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite docs/diagnostics.md from the rule table")

const docPath = "../../../../docs/diagnostics.md"

// TestDocsMatchRules keeps docs/diagnostics.md equal to the table in rules.go.
func TestDocsMatchRules(t *testing.T) {
	want := RulesMarkdown()
	if *update {
		if err := os.WriteFile(docPath, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is out of date; run: cd src/cli && go test ./internal/diag -update", docPath)
	}
}

// TestCodeRanges checks the numbering scheme: codes are
// OSFnnn, in ascending order, and only the 900s are warnings.
func TestCodeRanges(t *testing.T) {
	last := 0
	for _, r := range Rules {
		n, err := strconv.Atoi(strings.TrimPrefix(r.Code, "OSF"))
		if err != nil || len(r.Code) != 6 {
			t.Errorf("%s: code must be OSF followed by three digits", r.Code)
			continue
		}
		if n <= last {
			t.Errorf("%s comes after OSF%03d; keep the table in code order", r.Code, last)
		}
		last = n
		if (n >= 900) != (r.Severity == Warning) {
			t.Errorf("%s %s: warnings are 900-999, errors below", r.Code, r.Severity)
		}
		if r.Name == "" || r.Desc == "" || strings.ToLower(r.Name) != r.Name {
			t.Errorf("%s: name must be lowercase and the description set", r.Code)
		}
	}
}

func TestLookup(t *testing.T) {
	if r, ok := Lookup("OSF112"); !ok || r.Name != "unknown-speaker" {
		t.Errorf("Lookup(OSF112) = %+v, %v", r, ok)
	}
	if r, ok := Lookup("unknown-speaker"); !ok || r.Code != "OSF112" {
		t.Errorf("Lookup(unknown-speaker) = %+v, %v", r, ok)
	}
	if _, ok := Lookup("OSF999"); ok {
		t.Error("Lookup(OSF999) found a rule")
	}
}
