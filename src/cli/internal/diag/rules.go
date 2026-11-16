package diag

import (
	"fmt"
	"sort"
	"strings"
)

// Rule is one kind of diagnostic. Every diagnostic osf produces, from the
// lexer to the translation checks, carries the code of a rule in this table.
// Codes are stable: a rule is never renumbered, and a retired code is not
// reused.
//
// Ranges: 001-099 lexing and parsing (.osf and YAML syntax, files), 100-199
// structure and references, 200-299 variables and values, 300-399
// translations, 900-999 warnings.
type Rule struct {
	Code     string
	Name     string
	Desc     string
	Severity Severity
}

func (r Rule) At(pos, end Pos, format string, args ...any) Diagnostic {
	return Diagnostic{Pos: pos, End: end, Code: r.Code, Severity: r.Severity, Msg: fmt.Sprintf(format, args...)}
}

// Rules is the table, in code order. docs/diagnostics.md is generated from
// it (see RulesMarkdown).
var Rules = []Rule{
	// Lexer and parser.
	{"OSF001", "invalid-utf8", "The file is not valid UTF-8.", Error},
	{"OSF002", "unexpected-character", "A character that is not part of the language, such as `@` or `#`.", Error},
	{"OSF003", "bad-escape", "An unknown escape in a string; only `\\\" \\\\ \\n \\t` and `\\{{` are allowed.", Error},
	{"OSF004", "unclosed-string", "A `\"` string is not closed before the end of its line.", Error},
	{"OSF005", "unclosed-long-text", "A `\"\"\"` text is not closed before the end of the file.", Error},
	{"OSF006", "unclosed-comment", "A `/*` comment is not closed before the end of the file.", Error},
	{"OSF007", "bad-number", "A token starting with `-` that is neither a number nor a name.", Error},
	{"OSF010", "unexpected-token", "The parser found a token it did not expect here.", Error},
	{"OSF011", "unknown-block", "A top-level block that is not Versions, Story, I18n, Character, Document, Variable, Dialogue or Locale.", Error},
	{"OSF012", "misplaced-block", "A block or field written where it does not belong, such as a Node outside a Dialogue or `requires` on a Node.", Error},
	{"OSF013", "unknown-field", "A field the block does not have.", Error},
	{"OSF014", "duplicate-field", "A field or map key written twice in one block.", Error},
	{"OSF015", "wrong-separator", "`=` where `:` is expected or the other way round; `start` written with a separator.", Error},
	{"OSF016", "reserved-word", "A reserved word (Node, start, true, ...) used as an id.", Error},
	{"OSF017", "expected-id", "An id was expected, something else was found.", Error},
	{"OSF018", "expected-string", "A string was expected for a text field, something else was found.", Error},
	{"OSF019", "expected-value", "A value (true, false, a number or a string) was expected, something else was found.", Error},
	{"OSF020", "unclosed-block", "A `{` block or value map is not closed before the end of the file.", Error},
	{"OSF021", "bad-reference", "A reference that is not written as `Kind.id`, or `speaker` written as `Character.<id>`.", Error},
	{"OSF030", "yaml-syntax", "The YAML file does not parse.", Error},
	{"OSF031", "empty-file", "A YAML file holds no resource.", Error},
	{"OSF032", "multiple-documents", "A YAML file holds more than one document.", Error},
	{"OSF090", "no-project", "The folder does not exist or holds no .osf or .yaml files.", Error},
	{"OSF091", "read-error", "A file or folder could not be read.", Error},

	// Structure and references.
	{"OSF100", "no-versions", "The project has no Versions block.", Error},
	{"OSF101", "duplicate-versions", "More than one Versions block or plugins Configuration.", Error},
	{"OSF102", "unknown-plugin", "A plugin or apiVersion this version of osf does not know.", Error},
	{"OSF103", "unsupported-version", "A plugin version this version of osf does not support.", Error},
	{"OSF104", "missing-version", "Versions does not set language, story or dialogue.", Error},
	{"OSF105", "plugin-not-enabled", "I18n or Locale resources are used but the i18n plugin is not enabled.", Error},
	{"OSF106", "no-story", "The project has no Story.", Error},
	{"OSF107", "duplicate-story", "The project has more than one Story.", Error},
	{"OSF108", "invalid-id", "An id with characters other than letters, digits, `_` and `-`.", Error},
	{"OSF109", "duplicate-id", "A Story, Character, Document, Variable or Dialogue id defined twice.", Error},
	{"OSF110", "missing-field", "A required field is missing: a Story, Character or Document name, a Node or Option text, a YAML structural field.", Error},
	{"OSF111", "unknown-start", "The Story start points to a Dialogue that does not exist.", Error},
	{"OSF112", "unknown-speaker", "A speaker naming a Character that does not exist.", Error},
	{"OSF113", "unknown-node", "`next` points to a node that is not in the dialogue.", Error},
	{"OSF114", "unknown-dialogue", "`next` points to a Dialogue that does not exist.", Error},
	{"OSF115", "unknown-start-node", "The dialogue's start node is not one of its nodes.", Error},
	{"OSF116", "no-nodes", "A Dialogue with no nodes.", Error},
	{"OSF117", "next-and-options", "A node with both `next` and options.", Error},
	{"OSF118", "duplicate-node", "A node id used twice in one dialogue.", Error},
	{"OSF119", "duplicate-option", "An option id used twice in one node.", Error},
	{"OSF120", "unknown-text-reference", "A `{{ Kind.id }}` reference in text to something that does not exist.", Error},
	{"OSF121", "bad-text-reference-field", "A `{{ Kind.id.field }}` reference to a field the target has no text in: name or description for a Character or a Story, name for a Document or a Dialogue; a Variable reference takes no field.", Error},
	{"OSF122", "wrong-type", "A YAML value of the wrong shape: a string where a mapping is expected, and so on.", Error},
	{"OSF123", "unknown-kind", "A YAML kind that is not part of its apiVersion.", Error},
	{"OSF124", "duplicate-plugin", "A plugin listed twice in the plugins Configuration.", Error},
	{"OSF125", "branch-after-fallback", "A YAML `next` list entry without `requires` that is not the last one, so the entries after it are never taken.", Error},

	// Variables and values.
	{"OSF200", "unknown-variable", "`set` or `requires` names a Variable that does not exist.", Error},
	{"OSF201", "wrong-value-type", "A `set` or `requires` value of a different type than the Variable, or a negative value set on an UnsignedNumber.", Error},
	{"OSF202", "bad-default", "A Variable default of a different type than the Variable, or a negative default for an UnsignedNumber.", Error},
	{"OSF203", "unknown-variable-type", "A Variable type other than Boolean, Number or UnsignedNumber.", Error},
	{"OSF204", "number-too-large", "A number that does not fit in 32 bits (-2147483648 to 2147483647).", Error},
	{"OSF205", "not-a-number", "Add, subtract or a comparison on a Boolean Variable, which can only be set or checked for equality.", Error},
	{"OSF206", "negative-amount", "An amount below 0 to add or subtract.", Error},

	// Translations.
	{"OSF300", "duplicate-i18n", "More than one I18n configuration.", Error},
	{"OSF301", "invalid-locale-id", "A locale id that is not three letters, capitalised, like Rus.", Error},
	{"OSF302", "duplicate-locale", "A locale listed twice in the I18n configuration.", Error},
	{"OSF303", "unknown-locale", "A Locale block for a language the I18n configuration does not list.", Error},
	{"OSF304", "unknown-translation-key", "A translation for a text that is not in the story (renamed or removed?).", Error},
	{"OSF305", "duplicate-translation", "The same text translated twice for one locale.", Error},
	{"OSF306", "references-differ", "A translation whose `{{ }}` references differ from the source text.", Error},
	{"OSF307", "missing-translation", "A text with no translation for a locale with `enforce = true`.", Error},
	{"OSF308", "locale-folder-mismatch", "A Locale block in the folder of another locale.", Error},
	{"OSF309", "not-translatable", "A Locale block holds something other than name, description, body, title and text.", Error},
	{"OSF310", "bad-translation-key", "A YAML translation key that is not `Kind.id.field...`.", Error},

	// Warnings.
	{"OSF900", "unreachable-node", "A node that no `next` leads to and that is not the start node.", Warning},
	{"OSF901", "untranslated", "Texts with no translation for a locale without `enforce = true`.", Warning},
}

var byCode, byName = index()

func index() (map[string]Rule, map[string]Rule) {
	codes, names := map[string]Rule{}, map[string]Rule{}
	for _, r := range Rules {
		if _, dup := codes[r.Code]; dup {
			panic("diag: rule code " + r.Code + " is listed twice")
		}
		if _, dup := names[r.Name]; dup {
			panic("diag: rule name " + r.Name + " is listed twice")
		}
		codes[r.Code], names[r.Name] = r, r
	}
	return codes, names
}

func Lookup(codeOrName string) (Rule, bool) {
	if r, ok := byCode[codeOrName]; ok {
		return r, true
	}
	r, ok := byName[codeOrName]
	return r, ok
}

// RulesMarkdown renders the table as docs/diagnostics.md.
func RulesMarkdown() string {
	var b strings.Builder
	b.WriteString(`# Diagnostics

Every problem ` + "`osf validate`" + ` (and the language server) reports carries a stable rule
code. On the terminal a diagnostic looks like

` + "```" + `
story.osf:12:14: error: OSF112 Node node_01: speaker: unknown character "strnger"; did you mean "stranger"?
` + "```" + `

that is ` + "`file:line:col: severity: code message`" + `, with a suggestion appended when a
reference is close to an existing id. ` + "`osf validate -o json`" + ` prints the same
diagnostics as JSON with start and end positions.

Code ranges: 001-099 lexing and parsing, 100-199 structure and references, 200-299
variables and values, 300-399 translations, 900-999 warnings.

This file is generated from ` + "`src/cli/internal/diag/rules.go`" + `; regenerate it with
` + "`cd src/cli && go test ./internal/diag -update`" + `.

| Code | Name | Severity | Description |
|------|------|----------|-------------|
`)
	for _, r := range Rules {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.Code, r.Name, r.Severity, r.Desc)
	}
	return b.String()
}

// Names returns the rule names, sorted.
func Names() []string {
	var names []string
	for _, r := range Rules {
		names = append(names, r.Name)
	}
	sort.Strings(names)
	return names
}
