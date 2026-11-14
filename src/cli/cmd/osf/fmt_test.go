package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/format"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/source"
)

func TestFmt(t *testing.T) {
	src := "// header\n\n\n" +
		"Character  c{name=\"C\"   // the name\n" +
		"description   =   \"D\" }\n" +
		"Dialogue d {\n" +
		"\tNode a {   text : \"\"\"\n" +
		"            One\n" +
		"              Two\n" +
		"            // note\n" +
		"         \"\"\"\n" +
		"   set:{x:1 , y:true}\n" +
		"}}\n"
	want := "// header\n\n" +
		"Character c { name = \"C\"  // the name\n" +
		"  description = \"D\" }\n" +
		"Dialogue d {\n" +
		"  Node a { text: \"\"\"\n" +
		"    One\n" +
		"      Two\n" +
		"    // note\n" +
		"  \"\"\"\n" +
		"    set: { x: 1, y: true }\n" +
		"  } }\n"
	got, err := format.Source("t.osf", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	again, _ := format.Source("t.osf", got)
	if !bytes.Equal(again, got) {
		t.Errorf("not idempotent:\n%s", again)
	}
}

func TestFmtExampleIsStable(t *testing.T) {
	files, _ := source.Files(osfExample, ".osf")
	for _, f := range files {
		src, _ := os.ReadFile(f)
		once, err := format.Source(f, src)
		if err != nil {
			t.Fatal(err)
		}
		twice, _ := format.Source(f, once)
		if !bytes.Equal(once, twice) {
			t.Errorf("%s: formatting is not idempotent", f)
		}
	}
}

// TestFmtCommand: -check reports an unformatted file with status 3 and
// changes nothing; without it the file is rewritten and a second run is clean.
func TestFmtCommand(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "c.osf")
	const messy = "Character   c{name=\"C\"}\n"
	os.WriteFile(file, []byte(messy), 0o644)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"fmt", "-check", dir}, &stdout, &stderr); code != 3 {
		t.Fatalf("fmt -check exit %d, want 3:\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "c.osf") {
		t.Errorf("fmt -check did not list the file:\n%s", stdout.String())
	}
	if src, _ := os.ReadFile(file); string(src) != messy {
		t.Errorf("fmt -check changed the file:\n%s", src)
	}

	stdout.Reset()
	if code := run([]string{"fmt", "-diff", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("fmt exit %d:\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "+Character c { name = \"C\" }") {
		t.Errorf("fmt -diff did not show the change:\n%s", stdout.String())
	}
	if src, _ := os.ReadFile(file); string(src) != "Character c { name = \"C\" }\n" {
		t.Errorf("fmt wrote:\n%s", src)
	}
	if code := run([]string{"fmt", "-check", dir}, &stdout, &stderr); code != 0 {
		t.Errorf("fmt -check after fmt exit %d", code)
	}
}

// TestRepositoryIsFormatted keeps every .osf file in the repository, the
// example and the test projects, in the standard formatting.
func TestRepositoryIsFormatted(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"fmt", "-check", "../../../../example", "../../../../tests"}, &stdout, &stderr); code != 0 {
		t.Errorf("fmt -check exit %d; not formatted:\n%s%s", code, stdout.String(), stderr.String())
	}
}
