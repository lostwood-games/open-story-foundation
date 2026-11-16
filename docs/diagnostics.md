# Diagnostics

Every problem `osf validate` (and the language server) reports carries a stable rule
code. On the terminal a diagnostic looks like

```
story.osf:12:14: error: OSF112 Node node_01: speaker: unknown character "strnger"; did you mean "stranger"?
```

that is `file:line:col: severity: code message`, with a suggestion appended when a
reference is close to an existing id. `osf validate -o json` prints the same
diagnostics as JSON with start and end positions.

Code ranges: 001-099 lexing and parsing, 100-199 structure and references, 200-299
variables and values, 300-399 translations, 900-999 warnings.

This file is generated from `src/cli/internal/diag/rules.go`; regenerate it with
`cd src/cli && go test ./internal/diag -update`.

| Code | Name | Severity | Description |
|------|------|----------|-------------|
| OSF001 | invalid-utf8 | error | The file is not valid UTF-8. |
| OSF002 | unexpected-character | error | A character that is not part of the language, such as `@` or `#`. |
| OSF003 | bad-escape | error | An unknown escape in a string; only `\" \\ \n \t` and `\{{` are allowed. |
| OSF004 | unclosed-string | error | A `"` string is not closed before the end of its line. |
| OSF005 | unclosed-long-text | error | A `"""` text is not closed before the end of the file. |
| OSF006 | unclosed-comment | error | A `/*` comment is not closed before the end of the file. |
| OSF007 | bad-number | error | A token starting with `-` that is neither a number nor a name. |
| OSF010 | unexpected-token | error | The parser found a token it did not expect here. |
| OSF011 | unknown-block | error | A top-level block that is not Versions, Story, I18n, Character, Document, Variable, Dialogue or Locale. |
| OSF012 | misplaced-block | error | A block or field written where it does not belong, such as a Node outside a Dialogue or `requires` on a Node. |
| OSF013 | unknown-field | error | A field the block does not have. |
| OSF014 | duplicate-field | error | A field or map key written twice in one block. |
| OSF015 | wrong-separator | error | `=` where `:` is expected or the other way round; `start` written with a separator. |
| OSF016 | reserved-word | error | A reserved word (Node, start, true, ...) used as an id. |
| OSF017 | expected-id | error | An id was expected, something else was found. |
| OSF018 | expected-string | error | A string was expected for a text field, something else was found. |
| OSF019 | expected-value | error | A value (true, false, a number or a string) was expected, something else was found. |
| OSF020 | unclosed-block | error | A `{` block or value map is not closed before the end of the file. |
| OSF021 | bad-reference | error | A reference that is not written as `Kind.id`, or `speaker` written as `Character.<id>`. |
| OSF030 | yaml-syntax | error | The YAML file does not parse. |
| OSF031 | empty-file | error | A YAML file holds no resource. |
| OSF032 | multiple-documents | error | A YAML file holds more than one document. |
| OSF090 | no-project | error | The folder does not exist or holds no .osf or .yaml files. |
| OSF091 | read-error | error | A file or folder could not be read. |
| OSF100 | no-versions | error | The project has no Versions block. |
| OSF101 | duplicate-versions | error | More than one Versions block or plugins Configuration. |
| OSF102 | unknown-plugin | error | A plugin or apiVersion this version of osf does not know. |
| OSF103 | unsupported-version | error | A plugin version this version of osf does not support. |
| OSF104 | missing-version | error | Versions does not set language, story or dialogue. |
| OSF105 | plugin-not-enabled | error | I18n or Locale resources are used but the i18n plugin is not enabled. |
| OSF106 | no-story | error | The project has no Story. |
| OSF107 | duplicate-story | error | The project has more than one Story. |
| OSF108 | invalid-id | error | An id with characters other than letters, digits, `_` and `-`. |
| OSF109 | duplicate-id | error | A Story, Character, Document, Variable or Dialogue id defined twice. |
| OSF110 | missing-field | error | A required field is missing: a Story, Character or Document name, a Node or Option text, a YAML structural field. |
| OSF111 | unknown-start | error | The Story start points to a Dialogue that does not exist. |
| OSF112 | unknown-speaker | error | A speaker naming a Character that does not exist. |
| OSF113 | unknown-node | error | `next` or a played or selected condition points to a node that is not in the dialogue. |
| OSF114 | unknown-dialogue | error | `next` or a played or selected condition points to a Dialogue that does not exist. |
| OSF115 | unknown-start-node | error | The dialogue's start node is not one of its nodes. |
| OSF116 | no-nodes | error | A Dialogue with no nodes. |
| OSF117 | next-and-options | error | A node with both `next` and options. |
| OSF118 | duplicate-node | error | A node id used twice in one dialogue. |
| OSF119 | duplicate-option | error | An option id used twice in one node. |
| OSF120 | unknown-text-reference | error | A `{{ Kind.id }}` reference in text to something that does not exist. |
| OSF121 | bad-text-reference-field | error | A `{{ Kind.id.field }}` reference to a field the target has no text in: name or description for a Character or a Story, name for a Document or a Dialogue; a Variable reference takes no field. |
| OSF122 | wrong-type | error | A YAML value of the wrong shape: a string where a mapping is expected, and so on. |
| OSF123 | unknown-kind | error | A YAML kind that is not part of its apiVersion. |
| OSF124 | duplicate-plugin | error | A plugin listed twice in the plugins Configuration. |
| OSF125 | branch-after-fallback | error | A YAML `next` list entry without `requires` that is not the last one, so the entries after it are never taken. |
| OSF126 | unknown-option | error | A selected condition names an option the node does not have. |
| OSF127 | bad-history-condition | error | A played or selected condition that is not `<node>.played` or `<node>.<option>.selected` (optionally after `Dialogue.<id>.`), that is in `set`, or that is compared with something other than true or false. |
| OSF200 | unknown-variable | error | `set` or `requires` names a Variable that does not exist. |
| OSF201 | wrong-value-type | error | A `set` or `requires` value of a different type than the Variable, or a negative value set on an UnsignedNumber. |
| OSF202 | bad-default | error | A Variable default of a different type than the Variable, or a negative default for an UnsignedNumber. |
| OSF203 | unknown-variable-type | error | A Variable type other than Boolean, Number or UnsignedNumber. |
| OSF204 | number-too-large | error | A number that does not fit in 32 bits (-2147483648 to 2147483647). |
| OSF205 | not-a-number | error | Add, subtract or a comparison on a Boolean Variable, which can only be set or checked for equality. |
| OSF206 | negative-amount | error | An amount below 0 to add or subtract. |
| OSF300 | duplicate-i18n | error | More than one I18n configuration. |
| OSF301 | invalid-locale-id | error | A locale id that is not three letters, capitalised, like Rus. |
| OSF302 | duplicate-locale | error | A locale listed twice in the I18n configuration. |
| OSF303 | unknown-locale | error | A Locale block for a language the I18n configuration does not list. |
| OSF304 | unknown-translation-key | error | A translation for a text that is not in the story (renamed or removed?). |
| OSF305 | duplicate-translation | error | The same text translated twice for one locale. |
| OSF306 | references-differ | error | A translation whose `{{ }}` references differ from the source text. |
| OSF307 | missing-translation | error | A text with no translation for a locale with `enforce = true`. |
| OSF308 | locale-folder-mismatch | error | A Locale block in the folder of another locale. |
| OSF309 | not-translatable | error | A Locale block holds something other than name, description, body, title and text. |
| OSF310 | bad-translation-key | error | A YAML translation key that is not `Kind.id.field...`. |
| OSF900 | unreachable-node | warning | A node that no `next` leads to and that is not the start node. |
| OSF901 | untranslated | warning | Texts with no translation for a locale without `enforce = true`. |
