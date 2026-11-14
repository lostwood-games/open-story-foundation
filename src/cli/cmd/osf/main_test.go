package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	osfExample  = "../../../../example/osf"
	yamlExample = "../../../../example/yaml"
)

func writeProject(t *testing.T, files map[string]string) string {
	dir := t.TempDir()
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestYAMLExampleIsValid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"validate", yamlExample}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d:\n%s", code, stderr.String())
	}
}

const baseStory = `
Story s { name = "S" start = Dialogue.d }
Character hero { name = "Hero" }
Variable brave: Boolean = false
`

func TestErrors(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"unknown speaker", map[string]string{"s.osf": baseStory + `Dialogue d { Node a { speaker: ghost text: "x" } }`}, `OSF112 Node a: speaker: unknown character "ghost"`},
		{"duplicate node", map[string]string{"s.osf": baseStory + `Dialogue d { Node a { text: "x" } Node a { text: "y" } }`}, "Node a is already defined on line 5"},
		{"next and options", map[string]string{"s.osf": baseStory + `Dialogue d { Node a { text: "x" next: b Option o { text: "o" } } Node b { text: "b" } }`}, "has both next and options"},
		{"missing node", map[string]string{"s.osf": baseStory + `Dialogue d { Node a { text: "x" next: nowhere } }`}, `node "nowhere" does not exist`},
		{"missing dialogue", map[string]string{"s.osf": baseStory + `Dialogue d { Node a { text: "x" next: Dialogue.nope } }`}, "next: Dialogue.nope does not exist"},
		{"no text", map[string]string{"s.osf": baseStory + `Dialogue d { Node a { next: a } }`}, "Node a has no text"},
		{"variable type", map[string]string{"s.osf": baseStory + `Dialogue d { Node a { text: "x" set: { brave: 3 } } }`}, "brave is boolean, got 3"},
		{"unknown variable", map[string]string{"s.osf": baseStory + `Dialogue d { Node a { text: "x" Option o { text: "o" requires: { rich: true } } } }`}, `unknown variable "rich"`},
		{"bad default", map[string]string{"s.osf": baseStory + `Variable gold: UnsignedNumber = -1
Dialogue d { Node a { text: "x" } }`}, `Variable gold is unsignedNumber but its default is -1`},
		{"text reference", map[string]string{"s.osf": baseStory + `Dialogue d { Node a { text: "Hi {{ Character.ghost }}" } }`}, "reference {{ Character.ghost }} does not exist"},
		{"escaped reference is fine, field is not", map[string]string{"s.osf": baseStory + `Dialogue d { Node a { text: "\{{ x }} {{ Character.hero.age }}" } }`}, `has no text field "age"`},
		{"duplicate field", map[string]string{"s.osf": baseStory + `Dialogue d { Node a { text: "x" text: "y" } }`}, "text is already set on line 5"},
		{"wrong separator", map[string]string{"s.osf": `Character c { name: "C" }`}, "expected '=' after name, found ':'"},
		{"reserved id", map[string]string{"s.osf": `Character start { name = "C" }`}, `"start" is a reserved word`},
		{"unclosed block", map[string]string{"s.osf": `Dialogue d { Node a { text: "x" }`}, "Dialogue block is not closed"},
		{"no versions", map[string]string{"versions.osf": "", "s.osf": baseStory}, "no Versions block found"},
		{"unsupported version", map[string]string{"versions.osf": `Versions { language = "1" story = "v2" dialogue = "v1" }`, "s.osf": baseStory}, `story version "v2" is not supported`},
		{"i18n not listed", map[string]string{"versions.osf": `Versions { language = "1" story = "v1" dialogue = "v1" }`, "s.osf": baseStory + `I18n { source = Eng Locale Rus "Russian" }`}, "need the i18n plugin"},
		{"locale folder", map[string]string{"s.osf": baseStory + `I18n { source = Eng Locale Rus "Russian" }
Dialogue d { Node a { text: "x" } }`, "locales/Rus/s.osf": `Locale Jpn { Character hero { name = "Герой" } }`}, "Locale Jpn is in the locales/Rus folder"},
		{"not translatable", map[string]string{"s.osf": baseStory + `I18n { source = Eng Locale Rus "Russian" }
Dialogue d { Node a { text: "x" } }`, "locales/Rus/s.osf": `Locale Rus { Dialogue d { Node a { next: b } } }`}, "next cannot be translated"},
		{"renamed node", map[string]string{"s.osf": baseStory + `I18n { source = Eng Locale Rus "Russian" }
Dialogue d { Node a { text: "x" } }`, "locales/Rus/s.osf": `Locale Rus { Dialogue d { Node old { text: "y" } } }`}, "Dialogue.d.nodes.old.content.text is not a text in the story"},
		{"enforced locale", map[string]string{"s.osf": baseStory + `I18n { source = Eng Locale Rus "Russian" { enforce = true } }
Dialogue d { Node a { text: "x" } }`}, "missing Rus translation for Dialogue.d.nodes.a.content.text"},
		{"reference changed in translation", map[string]string{"s.osf": baseStory + `I18n { source = Eng Locale Rus "Russian" }
Dialogue d { Node a { text: "{{ Character.hero }} waves" } }`, "locales/Rus/s.osf": `Locale Rus { Dialogue d { Node a { text: "Он машет" } } }`}, "references differ from the source text"},
		{"document without name", map[string]string{"s.osf": baseStory + `Document lighthouse {}
Dialogue d { Node a { text: "x" } }`}, "Document lighthouse has no name"},
		{"document field", map[string]string{"s.osf": `Document lighthouse { name = "L" description = "Tall" }`}, `unknown field "description" in Document; expected name`},
		{"document reference", map[string]string{"s.osf": baseStory + `Dialogue d { Node a { text: "At {{ Document.ghost }}" } }`}, "reference {{ Document.ghost }} does not exist"},
		{"document text field", map[string]string{"s.osf": baseStory + `Document lighthouse { name = "L" }
Dialogue d { Node a { text: "{{ Document.lighthouse.description }}" } }`}, `has no text field "description"`},
		{"duplicate document", map[string]string{"s.osf": baseStory + `Document lighthouse { name = "L" }
Document lighthouse { name = "M" }
Dialogue d { Node a { text: "x" } }`}, "Document.lighthouse is already defined"},
		{"unknown locale", map[string]string{"s.osf": baseStory + `I18n { source = Eng Locale Rus "Russian" }
Dialogue d { Node a { text: "x" } }`, "locales/Jpn/s.osf": `Locale Jpn { Character hero { name = "ヒーロー" } }`}, "language Jpn is not listed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := tt.files["versions.osf"]; !ok {
				tt.files["versions.osf"] = `Versions { language = "1" story = "v1" dialogue = "v1" i18n = "v1" }`
			}
			dir := writeProject(t, tt.files)
			var stderr bytes.Buffer
			code := run([]string{"validate", dir}, &bytes.Buffer{}, &stderr)
			if code != 1 || !strings.Contains(stderr.String(), tt.want) {
				t.Errorf("exit %d, want 1 with %q in:\n%s", code, tt.want, stderr.String())
			}
		})
	}
}

func TestYAMLErrors(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.yaml": "apiVersion: story/v1\nkind: Character\ndata:\n  id: a\n  name: A\n  name: B\n",
		"b.yaml": "apiVersion: story/v1\nkind: Character\ndata:\n  id: b\n  nickname: B\n",
	})
	var stderr bytes.Buffer
	if code := run([]string{"validate", dir}, &bytes.Buffer{}, &stderr); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	for _, want := range []string{`a.yaml:6:3: error: OSF014 Character: key "name" is repeated`, `OSF013 Character: unknown field "nickname"`, `b.yaml:1:1: error: OSF110 Character b has no name`} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("want %q in:\n%s", want, stderr.String())
		}
	}
}

func TestNoProject(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"validate", t.TempDir()}, &bytes.Buffer{}, &stderr); code == 0 {
		t.Error("an empty folder passed validation")
	}
	if !strings.Contains(stderr.String(), "OSF090") {
		t.Errorf("want OSF090 in:\n%s", stderr.String())
	}
}
