package osfwrite

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/format"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/lang"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

func str(s string) *osf.Str { return &osf.Str{Value: s} }

// parse reads src as a standalone .osf file.
func parse(t *testing.T, src []byte) *osf.Project {
	t.Helper()
	u := lang.NewUnit()
	if err := lang.ParseFile(u, "t.osf", "t.osf", src); err != nil {
		t.Fatalf("output does not parse: %v\n%s", err, src)
	}
	return u.Project
}

// formatted checks that osf fmt leaves src alone.
func formatted(t *testing.T, src []byte) {
	t.Helper()
	out, err := format.Source("t.osf", src)
	if err != nil {
		t.Fatalf("format: %v\n%s", err, src)
	}
	if !bytes.Equal(out, src) {
		t.Errorf("osf fmt would change the output:\n%s\nformatted:\n%s", src, out)
	}
}

func TestTextValues(t *testing.T) {
	// Every text must come back unchanged through the lexer, and texts that
	// end in a line break use a """ block unless that would change them.
	texts := []string{
		"plain", "with \"quotes\" and \\ backslash", "tab\there", "a\nb", "a\nb\n", "one\n",
		"\n", "\n\n", "\na\n", "a\n\nb\n", "  indented\nflush\n", "// not a note\n", "  // not a note either\n",
		"all\n  indented\n", "trailing \n", "has \"\"\" quotes\n", `\//escaped\n`, "braces \\{{ x }} kept", "unicode: Где я?\n",
	}
	for _, s := range texts {
		p := &osf.Project{Characters: []*osf.Character{{ID: "c", Name: str("n"), Description: str(s)}}}
		out, err := Source(p, false)
		if err != nil {
			t.Fatal(err)
		}
		got := parse(t, out)
		if d := got.Characters[0].Description.Value; d != s {
			t.Errorf("%q came back as %q:\n%s", s, d, out)
		}
		formatted(t, out)
		isLong := strings.Contains(string(out), `"""`)
		switch s {
		case "a\nb\n", "one\n", "\n", "\n\n", "\na\n", "a\n\nb\n", "  indented\nflush\n", "// not a note\n", "all\n  indented\n", "unicode: Где я?\n":
			if !isLong {
				t.Errorf("%q should be written as a \"\"\" block:\n%s", s, out)
			}
		default:
			if isLong {
				t.Errorf("%q should be written as a quoted string:\n%s", s, out)
			}
		}
	}
}

func TestLiteralBraces(t *testing.T) {
	// The lexer keeps \{ as \{ so OSF sees literal braces; the writer must
	// write it back as \{{, not \\{{.
	p := parse(t, []byte(`Character c { name = "\{{ x }}" }`))
	out, _ := Source(p, false)
	if want := `name = "\{{ x }}"`; !strings.Contains(string(out), want) {
		t.Errorf("want %s in:\n%s", want, out)
	}
}

func TestLayout(t *testing.T) {
	src := `Versions { language = "1" story = "v1" dialogue = "v1" i18n = "v1" }
Variable gold: UnsignedNumber = 3
Variable name: Number = -2
Character b { name = "B" }
Character a { name = "A" description = "long" }
Story s { name = "S" description = "D" start = Dialogue.d }
I18n { source = Eng Locale Rus "Russian" { enforce = true } Locale Jpn "Japanese" { enforce = false } }
Dialogue d { name = "Dee"  start second
  Node first { text: "1" next: second }
  Node second { speaker: a title: "T" text: "2" set: { gold += 4, name: 7 }
    Option o1 { text: "o1" requires: { gold >= 3 } set: { gold -= 5 } next: first }
    Option o2 { text: "o2" }
  }
}
Dialogue e { Node only { text: "e" } }
Locale Rus { Dialogue d { Node second { Option o2 { text: "о2" } Option o1 { text: "о1" } title: "Т" } Node first { text: "1" } name = "Ди" } Character a { name = "А" } Story s { name = "С" } }
`
	u := lang.NewUnit()
	if err := lang.ParseFile(u, "t.osf", "t.osf", []byte(src)); err != nil {
		t.Fatal(err)
	}
	lang.ResolveVersions(u)
	out, err := Source(u.Project, true)
	if err != nil {
		t.Fatal(err)
	}
	want := `Versions {
  language = "1"

  story    = "v1"
  dialogue = "v1"
  i18n     = "v1"
}

Story s {
  name        = "S"
  description = "D"
  start       = Dialogue.d
}

I18n {
  source = Eng
  Locale Rus "Russian" { enforce = true }
  Locale Jpn "Japanese"
}

Character b {
  name = "B"
}

Character a {
  name        = "A"
  description = "long"
}

Variable gold: UnsignedNumber = 3
Variable name: Number = -2

Dialogue d {
  name = "Dee"
  start second

  Node first {
    text: "1"
    next: second
  }

  Node second {
    speaker: a
    title: "T"
    text: "2"
    set: { gold += 4, name: 7 }

    Option o1 {
      text: "o1"
      requires: { gold >= 3 }
      set: { gold -= 5 }
      next: first
    }

    Option o2 {
      text: "o2"
    }
  }
}

Dialogue e {
  Node only {
    text: "e"
  }
}

Locale Rus {
  Story s {
    name = "С"
  }

  Character a {
    name = "А"
  }

  Dialogue d {
    name = "Ди"

    Node second {
      title: "Т"

      Option o2 { text: "о2" }
      Option o1 { text: "о1" }
    }

    Node first {
      text: "1"
    }
  }
}
`
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
	formatted(t, out)
}

func TestProjectLayout(t *testing.T) {
	p := &osf.Project{
		APIVersions: map[string]string{"story": "v1", "dialogue": "v1", "i18n": "v1", "plugins": "v1"},
		Plugins:     []string{"i18n/v1"},
		Stories:     []*osf.Story{{ID: "s", Name: str("S"), File: "story.yaml"}},
		Characters:  []*osf.Character{{ID: "a", Name: str("A"), File: "characters/a.yaml"}, {ID: "b", Name: str("B"), File: "people/b.yaml"}},
		Variables:   []*osf.Variable{{ID: "v", Type: osf.Boolean, File: "variables/v.yaml"}},
		Dialogues:   []*osf.DialogueModel{{ID: "d", Nodes: []*osf.NodeModel{{ID: "n", Text: str("t")}}, File: "act1/d.yaml"}},
		I18n:        []*osf.I18n{{Source: str("Eng"), Locales: []osf.LocaleDef{{Name: "Russian", ID: "Rus"}}, File: "plugins/i18n.yaml"}},
		Locales: []*osf.Locale{
			{Language: osf.Str{Value: "Rus"}, Values: []osf.LocaleValue{{Key: "Character.a.name", Text: osf.Str{Value: "А"}}}, File: "locales/Rus/characters/a.yaml"},
			{Language: osf.Str{Value: "Rus"}, Values: []osf.LocaleValue{{Key: "Character.b.name", Text: osf.Str{Value: "Б"}}}, File: "translations/b.yaml"},
			{Language: osf.Str{Value: "Jpn"}, Values: []osf.LocaleValue{{Key: "Character.b.name", Text: osf.Str{Value: "ビ"}}}, File: "locales/Rus/wrong.yaml"},
		},
	}
	files, err := Project(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
		formatted(t, f.Content)
	}
	want := "act1/d.osf characters.osf locales/Jpn/wrong.osf locales/Rus/characters/a.osf locales/Rus/translations/b.osf story.osf variables.osf versions.osf"
	if got := strings.Join(paths, " "); got != want {
		t.Errorf("files: %s\nwant:  %s", got, want)
	}

	// With origins, resources go back into the files they came from.
	origin := map[string]string{"story.yaml": "story.osf", "plugins/i18n.yaml": "story.osf", "variables/v.yaml": "story.osf", "characters/a.yaml": "cast.osf", "people/b.yaml": "cast.osf"}
	files, err = Project(p, func(file string) string { return origin[file] })
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = string(f.Content)
	}
	if got := byPath["story.osf"]; !strings.HasPrefix(got, "Story s {\n  name = \"S\"\n}\n\nI18n {\n") || !strings.HasSuffix(got, "\nVariable v: Boolean = false\n") {
		t.Errorf("story.osf:\n%s", got)
	}
	if got := byPath["cast.osf"]; !strings.Contains(got, "Character a {") || !strings.Contains(got, "Character b {") {
		t.Errorf("cast.osf:\n%s", got)
	}
}

func TestBadTranslationKey(t *testing.T) {
	p := &osf.Project{Locales: []*osf.Locale{{Language: osf.Str{Value: "Rus"}, Values: []osf.LocaleValue{{Key: "Dialogue.d.nodes.n.content.speaker", Text: osf.Str{Value: "x"}}}}}}
	if _, err := Source(p, false); err == nil || !strings.Contains(err.Error(), "cannot be written as a Locale block") {
		t.Errorf("err = %v", err)
	}
}

func TestEmptyBlocks(t *testing.T) {
	p := &osf.Project{Dialogues: []*osf.DialogueModel{{ID: "d"}}, Locales: []*osf.Locale{{Language: osf.Str{Value: "Rus"}}}}
	out, err := Source(p, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := "Dialogue d {}\n\nLocale Rus {}\n"; string(out) != want {
		t.Errorf("got:\n%s", out)
	}
	formatted(t, out)
}
