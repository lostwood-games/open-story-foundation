package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/diag"
)

// diagCase is one broken project. name is a rule name, optionally followed
// by "-<variant>". want is a substring of the message, hint the expected
// "did you mean" suggestion.
type diagCase struct {
	name  string
	files map[string]string
	links map[string]string // symlink name -> target
	want  string
	hint  string
}

const versions = `Versions { language = "1" story = "v1" dialogue = "v1" }
`

const versionsI18n = `Versions { language = "1" story = "v1" dialogue = "v1" i18n = "v1" }
`

const head = versions + `
Story s { name = "S" start = Dialogue.intro }
Character hero { name = "Hero" }
Variable brave: Boolean = false
`

const headI18n = versionsI18n + `
Story s { name = "S" start = Dialogue.intro }
Character hero { name = "Hero" }
Variable brave: Boolean = false
I18n { source = Eng Locale Rus "Russian" }
`

const intro = `
Dialogue intro {
  Node greet { speaker: hero text: "Hi" next: leave }
  Node leave { text: "Bye" }
}
`

func story(body string) map[string]string { return map[string]string{"story.osf": head + body} }

func i18nStory(body string, more map[string]string) map[string]string {
	files := map[string]string{"story.osf": headI18n + body}
	for k, v := range more {
		files[k] = v
	}
	return files
}

// i18nWith is i18nStory with the I18n block's locale list replaced.
func i18nWith(locales, body string, more map[string]string) map[string]string {
	files := i18nStory(body, more)
	files["story.osf"] = strings.Replace(files["story.osf"], `Locale Rus "Russian"`, locales, 1)
	return files
}

const (
	yamlStory = "apiVersion: story/v1\nkind: Story\ndata:\n  id: s\n  name: S\n  start: Dialogue.intro\n"
	yamlHero  = "apiVersion: story/v1\nkind: Character\ndata:\n  id: hero\n  name: Hero\n"
	yamlIntro = "apiVersion: dialogue/v1\nkind: Dialogue\ndata:\n  id: intro\n  startNode: greet\n  nodes:\n    greet:\n      content:\n        speaker: Character.hero\n        text: Hi\n"
)

func yamlProject(more map[string]string) map[string]string {
	files := map[string]string{"story.yaml": yamlStory, "hero.yaml": yamlHero, "intro.yaml": yamlIntro}
	for k, v := range more {
		files[k] = v
	}
	return files
}

var diagCases = []diagCase{
	// Lexer and parser.
	{name: "invalid-utf8", files: story(intro + "// caf\xe9\n"), want: "not valid UTF-8"},
	{name: "unexpected-character", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" } @\n}\n"), want: "unexpected character '@'"},
	{name: "bad-escape", files: story("\nDialogue intro {\n  Node greet { text: \"Hi \\q\" }\n}\n"), want: `unknown escape \q`},
	{name: "unclosed-string", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\n  }\n}\n"), want: "string is not closed on its line"},
	{name: "unclosed-long-text", files: story("\nDialogue intro {\n  Node greet { text: \"\"\"\n    Hi\n  }\n}\n"), want: "long text is not closed"},
	{name: "unclosed-comment", files: story(intro + "/* a comment that never ends\n"), want: "comment is not closed"},
	{name: "bad-number", files: story("Variable gold: Number = -1x\n" + intro), want: `"-1x" is neither a number nor a name`},
	{name: "unexpected-token", files: story(intro + "}\n"), want: "expected a block such as Dialogue or Character, found '}'"},
	{name: "unknown-block", files: story("Documnt villain { name = \"Villain\" }\n" + intro), want: `unknown block "Documnt"`},
	{name: "misplaced-block", files: story(intro + "Node stray { text: \"Lost\" }\n"), want: "Node blocks belong inside a Dialogue"},
	{name: "misplaced-block-nested", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" Node inner { text: \"x\" } }\n}\n"), want: "Node blocks do not belong inside Node"},
	{name: "misplaced-block-requires", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" requires: { brave: true } }\n}\n"), want: "requires is only allowed on an Option"},
	{name: "unknown-field", files: story("\nDialogue intro {\n  Node greet { txt: \"Hi\" }\n}\n"), want: `unknown field "txt" in Node`},
	{name: "duplicate-field", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" text: \"Again\" }\n}\n"), want: "text is already set on line"},
	{name: "wrong-separator", files: story("Character villain { name: \"Villain\" }\n" + intro), want: "expected '=' after name, found ':'"},
	{name: "wrong-separator-start", files: story("\nDialogue intro {\n  start = greet\n  Node greet { text: \"Hi\" }\n}\n"), want: "write start followed by the node id"},
	{name: "reserved-word", files: story("Character start { name = \"Start\" }\n" + intro), want: `"start" is a reserved word`},
	{name: "expected-id", files: story("\nDialogue intro {\n  Node { text: \"Hi\" }\n}\n"), want: "expected a node id, found '{'"},
	{name: "expected-string", files: story("Character villain { name = Villain }\n" + intro), want: `expected a string for name, found "Villain"`},
	{name: "expected-value", files: story("Variable gold: Number = lots\n" + intro), want: "expected a value for gold"},
	{name: "expected-value-enforce", files: i18nWith(`Locale Rus "Russian" { enforce = yes }`, intro, nil), want: "expected true or false for enforce"},
	{name: "unclosed-block", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" }\n"), want: "Dialogue block is not closed"},
	{name: "unclosed-block-map", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" set: { brave: true\n"), want: "set map is not closed"},
	{name: "bad-reference", files: story("Story s2 { name = \"S\" start = intro }\n" + intro), want: `expected Dialogue.<id>, found "intro"`},
	{name: "bad-reference-speaker", files: story("\nDialogue intro {\n  Node greet { speaker: Character.hero text: \"Hi\" }\n}\n"), want: "write the document id alone"},
	{name: "bad-reference-yaml", files: yamlProject(map[string]string{"story.yaml": strings.Replace(yamlStory, "Dialogue.intro", "intro", 1)}), want: `start "intro" must be Dialogue.<id>`},
	{name: "yaml-syntax", files: yamlProject(map[string]string{"hero.yaml": "apiVersion: story/v1\nkind: Character\ndata: [\n"}), want: "did not find expected"},
	{name: "empty-file", files: yamlProject(map[string]string{"notes.yaml": ""}), want: "file is empty"},
	{name: "multiple-documents", files: yamlProject(map[string]string{"hero.yaml": yamlHero + "---\n" + strings.Replace(yamlHero, "hero", "villain", 2)}), want: "a file holds one resource"},
	{name: "no-project", files: map[string]string{}, want: "no .yaml or .json files found"},
	{name: "read-error", files: map[string]string{}, links: map[string]string{"story.osf": "missing.osf"}, want: "no such file"},

	// Structure and references.
	{name: "no-versions", files: map[string]string{"story.osf": strings.TrimPrefix(head, versions) + intro}, want: "no Versions block found"},
	{name: "duplicate-versions", files: story(versions + intro), want: "a project has one Versions block"},
	{name: "duplicate-versions-yaml", files: yamlProject(map[string]string{"plugins/plugins.yaml": "apiVersion: plugins/v1\nkind: Configuration\ndata:\n  plugins: [i18n/v1]\n", "plugins/more.yaml": "apiVersion: plugins/v1\nkind: Configuration\ndata:\n  plugins: [i18n/v1]\n"}), want: "a project has one plugins/v1 Configuration"},
	{name: "unknown-plugin", files: map[string]string{"story.osf": strings.Replace(head, `dialogue = "v1" }`, `dialogue = "v1" magic = "v1" }`, 1) + intro}, want: `unknown plugin "magic"`},
	{name: "unknown-plugin-yaml", files: yamlProject(map[string]string{"magic.yaml": "apiVersion: magic/v1\nkind: Spell\ndata: {}\n"}), want: `unknown apiVersion "magic/v1"`},
	{name: "unknown-plugin-yaml-list", files: yamlProject(map[string]string{"plugins/plugins.yaml": "apiVersion: plugins/v1\nkind: Configuration\ndata:\n  plugins: [magic/v1]\n"}), want: `unknown plugin "magic/v1"`},
	{name: "unsupported-version", files: map[string]string{"story.osf": strings.Replace(head, `story = "v1"`, `story = "v2"`, 1) + intro}, want: `story version "v2" is not supported`},
	{name: "unsupported-version-yaml", files: yamlProject(map[string]string{"hero.yaml": strings.Replace(yamlHero, "story/v1", "story/v2", 1)}), want: "apiVersion story/v2 is not supported"},
	{name: "unsupported-version-yaml-list", files: yamlProject(map[string]string{"plugins/plugins.yaml": "apiVersion: plugins/v1\nkind: Configuration\ndata:\n  plugins: [i18n/v2]\n"}), want: "plugin i18n/v2 is not supported"},
	{name: "missing-version", files: map[string]string{"story.osf": strings.Replace(head, ` dialogue = "v1"`, "", 1) + intro}, want: "Versions must set dialogue"},
	{name: "plugin-not-enabled", files: story("I18n { source = Eng Locale Rus \"Russian\" }\n" + intro), want: "need the i18n plugin"},
	{name: "plugin-not-enabled-yaml", files: yamlProject(map[string]string{"locales/Rus/hero.yaml": "apiVersion: i18n/v1\nkind: Locale\ndata:\n  language: Rus\n  values:\n    Character.hero.name: Герой\n"}), want: "no plugins/v1 Configuration"},
	{name: "no-story", files: map[string]string{"story.osf": versions + "\nCharacter hero { name = \"Hero\" }\n" + intro}, want: "the project has no Story"},
	{name: "duplicate-story", files: story("Story s2 { name = \"Second\" }\n" + intro), want: "a project has exactly one Story"},
	{name: "invalid-id", files: yamlProject(map[string]string{"intro.yaml": strings.Replace(yamlIntro, "greet", "greet.1", 2)}), want: `node id "greet.1" may only contain`},
	{name: "duplicate-id", files: story("Character hero { name = \"Hero again\" }\n" + intro), want: "Character.hero is already defined at"},
	{name: "missing-field", files: story("Character villain { }\n" + intro), want: "Character villain has no name"},
	{name: "missing-field-text", files: story("\nDialogue intro {\n  Node greet { speaker: hero }\n}\n"), want: "Node greet has no text"},
	{name: "missing-field-yaml", files: yamlProject(map[string]string{"hero.yaml": "apiVersion: story/v1\ndata:\n  id: hero\n  name: Hero\n"}), want: `missing required field "kind"`},
	{name: "unknown-start", files: map[string]string{"story.osf": strings.Replace(head, "Dialogue.intro", "Dialogue.intr", 1) + intro}, want: "start: Dialogue.intr does not exist", hint: "Dialogue.intro"},
	{name: "unknown-speaker", files: story("\nDialogue intro {\n  Node greet { speaker: hreo text: \"Hi\" }\n}\n"), want: `speaker: unknown character "hreo"`, hint: "hero"},
	{name: "unknown-speaker-yaml", files: yamlProject(map[string]string{"intro.yaml": strings.Replace(yamlIntro, "Character.hero", "Character.hreo", 1)}), want: `speaker: unknown character "hreo"`, hint: "hero"},
	{name: "unknown-node", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" next: leav }\n  Node leave { text: \"Bye\" }\n}\n"), want: `next: node "leav" does not exist`, hint: "leave"},
	{name: "unknown-dialogue", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" next: Dialogue.intor }\n}\n"), want: "next: Dialogue.intor does not exist", hint: "Dialogue.intro"},
	{name: "unknown-start-node", files: story("\nDialogue intro {\n  start gret\n  Node greet { text: \"Hi\" }\n}\n"), want: `start node "gret" is not a node`, hint: "greet"},
	{name: "no-nodes", files: story("\nDialogue intro { }\n"), want: "Dialogue intro has no nodes"},
	{name: "next-and-options", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" next: leave Option go { text: \"Go\" } }\n  Node leave { text: \"Bye\" }\n}\n"), want: "has both next and options"},
	{name: "duplicate-node", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" }\n  Node greet { text: \"Hi again\" }\n}\n"), want: "Node greet is already defined on line"},
	{name: "duplicate-node-yaml", files: yamlProject(map[string]string{"intro.yaml": yamlIntro + "    greet:\n      content:\n        text: Again\n"}), want: `nodes: key "greet" is repeated`},
	{name: "duplicate-option", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" Option go { text: \"Go\" } Option go { text: \"Stay\" } }\n}\n"), want: "Option go is already defined on line"},
	{name: "unknown-text-reference", files: story("\nDialogue intro {\n  Node greet { text: \"Hi {{ Character.hreo }}\" }\n}\n"), want: "reference {{ Character.hreo }} does not exist", hint: "Character.hero"},
	{name: "bad-text-reference-field", files: story("\nDialogue intro {\n  Node greet { text: \"Hi {{ Character.hero.age }}\" }\n}\n"), want: `has no text field "age"`},
	{name: "bad-text-reference-field-variable", files: story("\nDialogue intro {\n  Node greet { text: \"Brave: {{ Variable.brave.name }}\" }\n}\n"), want: "a Variable reference takes no field"},
	{name: "wrong-type", files: yamlProject(map[string]string{"intro.yaml": "apiVersion: dialogue/v1\nkind: Dialogue\ndata:\n  id: intro\n  startNode: greet\n  nodes: [greet]\n"}), want: "nodes must be a mapping"},
	{name: "unknown-kind", files: yamlProject(map[string]string{"monster.yaml": "apiVersion: story/v1\nkind: Monster\ndata:\n  id: m\n"}), want: `kind "Monster" is not part of story/v1`},
	{name: "branch-after-fallback", files: yamlProject(map[string]string{"intro.yaml": yamlIntro + "      next:\n        - to: greet\n        - to: greet\n          requires: { brave: true }\n", "brave.yaml": "apiVersion: story/v1\nkind: Variable\ndata:\n  id: brave\n  type: boolean\n  default: false\n"}), want: "next[0] has no requires, so the branches after it are never taken"},
	{name: "next-and-options-branch", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" next: leave requires { brave: true } Option go { text: \"Go\" } }\n  Node leave { text: \"Bye\" }\n}\n"), want: "has both next and options"},
	{name: "unknown-node-branch", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" next: leav requires { brave: true } next: leave }\n  Node leave { text: \"Bye\" }\n}\n"), want: `next: node "leav" does not exist`, hint: "leave"},
	{name: "unknown-variable-branch", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" next: leave requires { brav: true } }\n  Node leave { text: \"Bye\" }\n}\n"), want: `next leave requires: unknown variable "brav"`, hint: "brave"},
	{name: "duplicate-field-next", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" next: leave next: leave }\n  Node leave { text: \"Bye\" }\n}\n"), want: "next is already set on line"},
	{name: "unknown-option", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" Option go { text: \"Go\" requires: { greet.stay.selected: true } } }\n}\n"), want: `Node greet in Dialogue intro has no option "stay"`},
	{name: "bad-history-condition", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" Option go { text: \"Go\" requires: { greet.done: true } } }\n}\n"), want: `"greet.done" is neither a variable nor <node>.played`},
	{name: "bad-history-condition-value", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" Option go { text: \"Go\" requires: { greet.played: 1 } } }\n}\n"), want: "greet.played can only be compared with true or false"},
	{name: "bad-history-condition-set", files: yamlProject(map[string]string{"intro.yaml": yamlIntro + "      set: { greet.played: true }\n"}), want: "greet.played is a condition; played and selected belong in requires"},
	{name: "unexpected-token-set-played", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" set: { greet.played: true } }\n}\n"), want: "played and selected conditions belong in requires"},
	{name: "unknown-node-played", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" Option go { text: \"Go\" requires: { gret.played: true } } }\n}\n"), want: `node "gret" does not exist in Dialogue intro`, hint: "greet"},
	{name: "unknown-dialogue-played", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" Option go { text: \"Go\" requires: { Dialogue.outro.greet.played: true } } }\n}\n"), want: "Dialogue.outro does not exist", hint: "Dialogue.intro"},
	{name: "duplicate-field-requires-op", files: story("Variable gold: Number = 3\n\nDialogue intro {\n  Node greet { text: \"Hi\" Option go { text: \"Go\" requires: { gold >= 3, gold >= 5 } } }\n}\n"), want: "gold >= is already required on line"},
	{name: "duplicate-field-requires-equal", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" Option go { text: \"Go\" requires: { brave: true, brave: false } } }\n}\n"), want: "brave is already required on line"},
	{name: "duplicate-field-requires-yaml", files: yamlProject(map[string]string{"gold.yaml": "apiVersion: story/v1\nkind: Variable\ndata:\n  id: gold\n  type: number\n  default: 0\n", "extra.yaml": "apiVersion: dialogue/v1\nkind: Dialogue\ndata:\n  id: extra\n  startNode: greet\n  nodes:\n    greet:\n      content: { text: Hi }\n      options:\n        go:\n          content: { text: Go }\n          requires: { gold: [{ operator: \">=\", value: 1 }, { operator: \">=\", value: 2 }] }\n"}), want: "gold >= is already required"},
	{name: "no-options-to-return", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" next: leave }\n  Node leave { text: \"Bye\" next: greet.options }\n}\n"), want: "Node greet has no options to go back to"},
	{name: "unknown-node-options", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" Option go { text: \"Go\" next: leave } }\n  Node leave { text: \"Bye\" next: gret.options }\n}\n"), want: `next: node "gret" does not exist`, hint: "greet"},
	{name: "bad-reference-options", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" Option go { text: \"Go\" next: leave } }\n  Node leave { text: \"Bye\" next: greet.choices }\n}\n"), want: "expected options after greet."},
	{name: "duplicate-plugin", files: yamlProject(map[string]string{"plugins/plugins.yaml": "apiVersion: plugins/v1\nkind: Configuration\ndata:\n  plugins: [i18n/v1, i18n/v1]\n"}), want: "plugin i18n/v1 is listed twice"},

	// Variables and values.
	{name: "unknown-variable", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" set: { brav: true } }\n}\n"), want: `unknown variable "brav"`, hint: "brave"},
	{name: "wrong-value-type", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" set: { brave: 3 } }\n}\n"), want: "brave is boolean, got 3"},
	{name: "wrong-value-type-unsigned", files: story("Variable gold: UnsignedNumber = 3\n\nDialogue intro {\n  Node greet { text: \"Hi\" set: { gold: -1 } }\n}\n"), want: "gold is unsignedNumber, got -1"},
	{name: "wrong-value-type-requires", files: story("Variable gold: Number = 3\n\nDialogue intro {\n  Node greet { text: \"Hi\" Option o { text: \"Go\" requires: { gold: true } } }\n}\n"), want: "gold is number, got true"},
	{name: "bad-default", files: story("Variable gold: Number = true\n" + intro), want: `Variable gold is number but its default is true`},
	{name: "bad-default-unsigned", files: story("Variable gold: UnsignedNumber = -1\n" + intro), want: `Variable gold is unsignedNumber but its default is -1`},
	{name: "unknown-variable-type", files: story("Variable gold: Integer = 3\n" + intro), want: `unknown variable type "Integer"`},
	{name: "unknown-variable-type-yaml", files: yamlProject(map[string]string{"gold.yaml": "apiVersion: story/v1\nkind: Variable\ndata:\n  id: gold\n  type: string\n  default: 3\n"}), want: `unknown variable type "string"`},
	{name: "number-too-large", files: story("Variable gold: Number = 99999999999999999999\n" + intro), want: "number 99999999999999999999 is too large"},
	{name: "number-too-large-32-bit", files: story("Variable gold: Number = 2147483648\n" + intro), want: "number 2147483648 is too large"},
	{name: "not-a-number", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" set: { brave += 1 } }\n}\n"), want: "brave is boolean, so it can only be set to true or false"},
	{name: "not-a-number-requires", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" Option o { text: \"Go\" requires: { brave != true } } }\n}\n"), want: "brave is boolean, so it can only be compared with true or false"},
	{name: "not-a-number-yaml", files: yamlProject(map[string]string{"brave.yaml": "apiVersion: story/v1\nkind: Variable\ndata:\n  id: brave\n  type: boolean\n  default: false\n", "extra.yaml": "apiVersion: dialogue/v1\nkind: Dialogue\ndata:\n  id: extra\n  startNode: greet\n  nodes:\n    greet:\n      content: { text: Hi }\n      set: { brave: { operation: add, value: 1 } }\n"}), want: "brave is boolean, so it can only be set to true or false"},
	{name: "unexpected-token-operator", files: story("Variable gold: Number = 3\n\nDialogue intro {\n  Node greet { text: \"Hi\" set: { gold >= 3 } }\n}\n"), want: "set takes name: value, name += number or name -= number, found '>='"},
	{name: "wrong-type-operator-yaml", files: yamlProject(map[string]string{"gold.yaml": "apiVersion: story/v1\nkind: Variable\ndata:\n  id: gold\n  type: number\n  default: 0\n", "extra.yaml": "apiVersion: dialogue/v1\nkind: Dialogue\ndata:\n  id: extra\n  startNode: greet\n  nodes:\n    greet:\n      content: { text: Hi }\n      options:\n        go:\n          content: { text: Go }\n          requires: { gold: { operator: \"=>\", value: 1 } }\n"}), want: "operator must be one of !=, <, <=, >, >="},
	{name: "negative-amount", files: story("Variable gold: Number = 3\n\nDialogue intro {\n  Node greet { text: \"Hi\" set: { gold += -2 } }\n}\n"), want: "cannot add -2; use a number of 0 or more"},

	// Translations.
	{name: "duplicate-i18n", files: i18nStory("I18n { Locale Jpn \"Japanese\" }\n"+intro, nil), want: "a project has one I18n configuration"},
	{name: "invalid-locale-id", files: i18nWith(`Locale rus "Russian"`, intro, nil), want: `locale id "rus" must be three letters`},
	{name: "duplicate-locale", files: i18nWith(`Locale Rus "Russian" Locale Rus "Russian again"`, intro, nil), want: "locale Rus is listed twice"},
	{name: "unknown-locale", files: i18nStory(intro, map[string]string{"locales/Rsu/story.osf": "Locale Rsu { Character hero { name = \"Герой\" } }\n"}), want: "language Rsu is not listed", hint: "Rus"},
	{name: "unknown-translation-key", files: i18nStory(intro, map[string]string{"locales/Rus/story.osf": "Locale Rus { Dialogue intro { Node gret { text: \"Привет\" } } }\n"}), want: "Dialogue.intro.nodes.gret.content.text is not a text in the story", hint: "Dialogue.intro.nodes.greet.content.text"},
	{name: "duplicate-translation", files: i18nStory(intro, map[string]string{"locales/Rus/story.osf": "Locale Rus {\n  Character hero { name = \"Герой\" }\n  Character hero { name = \"Богатырь\" }\n}\n"}), want: "Character.hero.name is already translated at"},
	{name: "references-differ", files: i18nStory("\nDialogue intro {\n  Node greet { text: \"{{ Character.hero }} waves\" }\n}\n", map[string]string{"locales/Rus/story.osf": "Locale Rus { Dialogue intro { Node greet { text: \"Он машет\" } } }\n"}), want: "references differ from the source text"},
	{name: "missing-translation", files: map[string]string{"story.osf": strings.Replace(headI18n, `Locale Rus "Russian"`, `Locale Rus "Russian" { enforce = true }`, 1) + intro}, want: "missing Rus translation for Story.s.name"},
	{name: "locale-folder-mismatch", files: i18nStory(intro, map[string]string{"locales/Rus/story.osf": "Locale Jpn { Character hero { name = \"ヒーロー\" } }\n"}), want: "Locale Jpn is in the locales/Rus folder"},
	{name: "not-translatable", files: i18nStory(intro, map[string]string{"locales/Rus/story.osf": "Locale Rus { Dialogue intro { Node greet { next: leave } } }\n"}), want: "next cannot be translated"},
	{name: "not-translatable-block", files: i18nStory(intro, map[string]string{"locales/Rus/story.osf": "Locale Rus { Variable brave: Boolean = true }\n"}), want: "only Story, Character, Document and Dialogue blocks can be translated"},
	{name: "bad-translation-key", files: yamlProject(map[string]string{"plugins/plugins.yaml": "apiVersion: plugins/v1\nkind: Configuration\ndata:\n  plugins: [i18n/v1]\n", "locales/Rus/hero.yaml": "apiVersion: i18n/v1\nkind: Locale\ndata:\n  language: Rus\n  values:\n    hello: Привет\n"}), want: `"hello" is not a translation key`},

	// Warnings.
	{name: "unreachable-node", files: story("\nDialogue intro {\n  Node greet { text: \"Hi\" }\n  Node leave { text: \"Bye\" }\n}\n"), want: "Node leave in Dialogue intro is never reached"},
	{name: "untranslated", files: i18nStory(intro, nil), want: "Rus: 4 of 4 texts untranslated"},
}

// ruleOf returns the rule a case is named after.
func ruleOf(t *testing.T, name string) diag.Rule {
	for n := name; n != ""; n = n[:max(strings.LastIndex(n, "-"), 0)] {
		if r, ok := diag.Lookup(n); ok {
			return r
		}
	}
	t.Fatalf("case %q is not named after a rule", name)
	return diag.Rule{}
}

func writeCase(t *testing.T, dir string, c diagCase) {
	for name, src := range c.files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range c.links {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func validateJSON(t *testing.T, dir string) (diag.JSONReport, int) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"validate", "-o", "json", dir}, &stdout, &stderr)
	var report diag.JSONReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("bad JSON (%v):\n%s%s", err, stdout.String(), stderr.String())
	}
	return report, code
}

// TestRules runs every case and checks the code, message and suggestion.
func TestRules(t *testing.T) {
	for _, c := range diagCases {
		t.Run(c.name, func(t *testing.T) {
			rule := ruleOf(t, c.name)
			dir := t.TempDir()
			writeCase(t, dir, c)
			report, code := validateJSON(t, dir)
			wantCode := 0
			if rule.Severity == diag.Error {
				wantCode = 1
			}
			if code != wantCode {
				t.Errorf("exit %d, want %d", code, wantCode)
			}
			found := false
			for _, d := range report.Diagnostics {
				if d.Code == rule.Code && strings.Contains(d.Message, c.want) && d.Suggestion == c.hint {
					found = true
				}
				if d.Code == "" {
					t.Errorf("diagnostic without a code: %+v", d)
				}
				if _, ok := diag.Lookup(d.Code); !ok {
					t.Errorf("code %s is not in the table", d.Code)
				}
				if d.Severity != "error" && d.Severity != "warning" {
					t.Errorf("bad severity %q", d.Severity)
				}
				if d.Range.End.Line < d.Range.Start.Line || d.Range.End.Line == d.Range.Start.Line && d.Range.End.Col < d.Range.Start.Col {
					t.Errorf("range ends before it starts: %+v", d.Range)
				}
			}
			if !found {
				b, _ := json.MarshalIndent(report, "", "  ")
				t.Errorf("no %s diagnostic with %q and suggestion %q in:\n%s", rule.Code, c.want, c.hint, b)
			}
		})
	}
}

// TestEveryRuleHasACase keeps the table free of dead entries.
func TestEveryRuleHasACase(t *testing.T) {
	covered := map[string]bool{}
	for _, c := range diagCases {
		covered[ruleOf(t, c.name).Code] = true
	}
	for _, r := range diag.Rules {
		if !covered[r.Code] {
			t.Errorf("%s %s has no case in diagCases", r.Code, r.Name)
		}
	}
}

// TestReportsEveryProblem puts a lexer error, two parser errors and a
// reference error in one project, across two files, and expects all of them
// from a single run, in file and position order.
func TestReportsEveryProblem(t *testing.T) {
	dir := t.TempDir()
	writeCase(t, dir, diagCase{files: map[string]string{
		"story.osf": head + "\nDialogue intro {\n  Node greet { txt: \"Hi\" next: leav }\n  Node leave { text: \"Bye\" @ }\n}\n",
		"more.osf":  "Documnt villain { name = \"V\" }\nCharacter guard { name = Guard }\n",
	}})
	report, code := validateJSON(t, dir)
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	got := map[string]bool{}
	var files []string
	for _, d := range report.Diagnostics {
		got[filepath.Base(d.File)+":"+d.Code] = true
		files = append(files, d.File)
		if d.Code == "OSF113" && d.Suggestion != "leave" {
			t.Errorf("OSF113 suggestion %q, want leave", d.Suggestion)
		}
	}
	for _, want := range []string{"more.osf:OSF011", "more.osf:OSF018", "story.osf:OSF002", "story.osf:OSF013", "story.osf:OSF113"} {
		if !got[want] {
			b, _ := json.MarshalIndent(report, "", "  ")
			t.Errorf("missing %s in:\n%s", want, b)
		}
	}
	if !sort.StringsAreSorted(files) {
		t.Errorf("diagnostics are not grouped by file: %v", files)
	}
}

// TestValidProjectsHaveNoErrors guards against the recovery logic inventing
// diagnostics on good input.
func TestValidProjectsHaveNoErrors(t *testing.T) {
	for _, dir := range []string{osfExample, yamlExample} {
		report, code := validateJSON(t, dir)
		if code != 0 || report.Summary.Errors != 0 {
			t.Errorf("%s: exit %d, %d errors", dir, code, report.Summary.Errors)
		}
	}
}
