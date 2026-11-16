# The OSF language, version 1

The OSF language is a small language for writing Open Story projects by hand. It
describes exactly the same things as the YAML format in the [Open Story spec](spec.md):
a Story, Characters, Documents, Variables, Dialogues and their translations. Files end in `.osf`; the `osf` command (`src/cli/`) reads, checks, formats,
builds and plays them.

```osf
Dialogue 01_intro_scene {
  Node node_03 {
    speaker: protagonist
    text: "Am I dreaming?"

    Option yes {
      text: "Yes, you are."
      set: { is_dreaming: true }
      next: node_04
    }

    Option no {
      text: "No. Fool"
      next: Dialogue.02_shadows_speak
    }
  }

  Node node_04 {
    text: "Strange"
  }
}
```

`osf convert <dir>` turns every `.osf` file in a folder into Open Story resources, so all
OSF tools (validation, translation checks, game export) work on these projects unchanged.
`osf convert -o osf` turns resources back into `.osf` source (§11).

Code blocks in this file are checked by a test: a block fenced `osf` is a
whole file that parses and is already formatted the way `osf fmt` writes it; `osf
fragment` marks lines taken out of a larger block; `osf invalid` marks an example of an
error.

## 1. Design rules

1. **Whitespace never matters.** Blocks are delimited by `{ }`, and every statement starts
   with a type or a field name, so there are no indentation rules and no required line
   breaks. Tabs, spaces and reformatting cannot change meaning. `;` may separate
   statements on one line but is never required.
2. **Types are capitalised, fields are not.** `Dialogue`, `Node`, `Option`, `Character`
   open a block; `next`, `speaker`, `text` are fields inside one. Types are spelled the
   same as OSF kinds and references (`Dialogue.02_shadows_speak`).
3. **One model.** Every construct maps to one OSF field (§8), with the same field names.
   The language adds no features the YAML format lacks.
4. **Two separators, one rule.** Fields of a resource (`Story`, `Character`, `Dialogue`
   name, `Versions`, `I18n`) are written `name = value`; the content of a node or an
   option is written `field: value`, like the YAML it compiles to. The wrong one is
   error OSF015, which says which one to use.

## 2. Lexical structure

- Files are UTF-8 (OSF001) and end in `.osf`. A byte order mark and `\r\n` line endings
  are accepted.
- Comments: `// to end of line` and `/* block */`. Block comments do not nest.
- **Types** (reserved, cannot be used as ids; OSF016): `Versions Story I18n Locale Character Document Variable Dialogue Node Option`, plus the variable types `Boolean Number UnsignedNumber`.
- **Other reserved words**: `start true false`.
- Field names (`name`, `text`, `next`, `source`, ...) are not reserved and can be ids.
- **Ids**: ASCII letters, digits and `_`, with single `-` allowed between them
  (`01_intro_scene`, `node-3`). No dots. Ids may start with a digit. Ids are case
  sensitive.
- **References**: `Kind.id`, e.g. `Dialogue.02_shadows_speak`. The language writes them
  in two places only: a Story's `start` and a `next` that jumps to another dialogue.
  `speaker:` and node targets of `next` are bare ids.
- **Strings**: `"one line"` with the escapes `\" \\ \n \t`, and `\{{` for literal braces
  in text (§6); any other escape is OSF003. A string cannot span lines (OSF004).
- **Long text**: `""" ... """`. When the text after the opening quotes starts with a
  line break, that line break is dropped, and so is a last line holding only
  whitespace before the closing quotes. Trailing whitespace on each line and the common
  leading whitespace of the lines are removed, so the text can be indented to match the
  code around it, and the text ends with a line break. Inside long text, a line starting
  with `//` is a writer's note and is removed; write `\//` for a real line starting with
  `//`. No other escapes apply in long text (`\{{` keeps its meaning in text). Long text
  written on one line (`"""like this"""`) is taken as it is.
- **Values**: `true`, `false` and whole numbers (`42`, `-3`) that fit in 32 bits,
  -2147483648 to 2147483647 (OSF204).
- **Value maps**: `{ is_dreaming: true, gold: 3 }`; commas are optional. An entry is
  `name: value`, or a number operation: `name += 2` and `name -= 2` in `set`, and
  `name != 2`, `name < 2`, `name <= 2`, `name > 2`, `name >= 2` in `requires` (§5.2).

## 3. Project settings

### 3.1 Versions

Every project has exactly one `Versions` block, normally in `versions.osf` at the project
root. It pins the version of the language and of every plugin the project uses, the way
Terraform's `versions.tf` pins providers:

```osf
Versions {
  language = "1"  // version of this syntax

  story    = "v1"  // built in: Story, Character, Document, Variable
  dialogue = "v1"  // built in: Dialogue
  i18n     = "v1"  // plugin: I18n settings and Locale blocks
}
```

- `language`, `story` and `dialogue` are required (OSF104). Plugins are listed only when
  used. The names on the left are OSF plugin names (the part before `/` in
  `apiVersion`), so they stay lowercase.
- This version of `osf` knows `language = "1"` and the plugins `story`, `dialogue` and
  `i18n`, each at `"v1"`. An unknown name is OSF102; a version it does not support is
  OSF103, so a compiler never guesses.
- Every resource compiles with the `apiVersion` taken from this block (`story/v1`,
  `dialogue/v1`, `i18n/v1`), so upgrading a plugin is a one-line change in one file.
- Using a block whose plugin is not listed is an error: an `I18n` or `Locale` block
  without `i18n = "v1"` is OSF105.
- No `Versions` block is OSF100; more than one is OSF101.

### 3.2 Story and translations

```osf
Story LDOD {
  name        = "Leviathan: The Last Day of The Decade"
  description = "..."                    // optional
  start       = Dialogue.01_intro_scene  // optional: the first dialogue
}

I18n {
  source = Eng                             // language the story files are written in
  Locale Rus "Russian" { enforce = true }  // missing translations are errors
  Locale Jpn "Japanese"                    // missing translations fall back to Eng
}
```

- Exactly one `Story` per project (OSF106, OSF107); `name` is required (OSF110).
  Without `start`, a player starts at the first `Dialogue` in file order (files sorted by
  path).
- The `I18n` block is optional, at most one per project (OSF300), and needs `i18n` in
  `Versions`. `source` is optional; it names the language the story files are written
  in. Each `Locale` line has a locale id, a display name and optionally
  `{ enforce = true }`; `enforce` defaults to `false`.
- Locale ids are three letters, capitalised: `Eng`, `Rus`, `Jpn` (OSF301). A locale
  listed twice is OSF302.

## 4. Characters, documents and variables

A `Character` is someone who speaks: any Character can be a dialogue's speaker. `name`
is required, `description` is optional; both are translatable texts.

A `Document` is a piece of the story world that dialogues refer to (a place, an item, a
lore entry), shown in text as `{{ Document.old_lighthouse }}`. It has only `name` for
now, and it is not a speaker. By convention documents live in `documents/documents.osf`.

```osf
Character stranger {
  name        = "Stranger"
  description = "A voice in the dark."  // optional
}

Document old_lighthouse {
  name = "The Old Lighthouse"
}

Variable is_dreaming: Boolean = false  // Boolean | Number | UnsignedNumber
Variable trust: Number = -1
Variable gold: UnsignedNumber = 0 { description = "Coins the player carries." }
```

A `Variable` is one line: id, type and default value, optionally followed by
`{ description = "..." }`, a note for writers and programmers that players never see and
that is not translated.

| Type | Holds |
|------|-------|
| `Boolean` | `true` or `false` |
| `Number` | a whole number, can be negative |
| `UnsignedNumber` | a whole number that never goes below 0 |

The default must have the variable's type, and an `UnsignedNumber` cannot start below 0
(OSF202). An unknown type is OSF203. Ids are unique per kind: two Characters named `stranger` are
OSF109, but a Character and a Document may share an id.

## 5. Dialogues

```osf
Dialogue 01_intro_scene {
  name = "Waking up"  // optional, translatable
  start node_02       // optional; defaults to the first node

  Node node_01 {
    text: "Before."
  }

  Node node_02 {
    text: "Where am I?"
    next: node_01
  }
}
```

`start` is followed by the node id, with no `=` or `:` (OSF015). A dialogue has at least
one node (OSF116); a start that is not one of its nodes is OSF115. A node that no `next`
reaches and that is not the start node is warning OSF900.

### 5.1 Nodes

A node is a list of `field: value` lines, in any order (branching `next` lines keep theirs, §5.3), plus its options:

```osf fragment
Node node_01 {
  speaker: protagonist
  title: "..."
  text: "Where am I?"
  next: node_02
}
```

| Field      | Value | OSF |
|------------|-------|-----|
| `text`     | Text. Required (OSF110). | `content.text` |
| `speaker`  | A Character id, written bare (OSF021 for `Character.<id>`). Omit for narration. | `content.speaker` (`Character.<id>`) |
| `title`    | Text: the shown speaker name, overriding the speaker Character's name. | `content.title` |
| `next`     | A node id in this dialogue, `<node>.options` to offer that node's options again, or `Dialogue.<id>` to jump to another dialogue's start node. Can branch on variables (§5.3). | `next` |
| `set`      | Value map (§5.2), applied when the node is reached. | `set` |
| `Option`   | A player choice (§5.2). Repeatable. | `options` |

A node has `next` or options, not both (OSF117). A node with neither ends the dialogue.
Each field appears at most once (OSF014), except branching `next` lines (§5.3), and node ids are unique in their dialogue
(OSF118). `requires` belongs to options only (OSF012).

```osf invalid
Dialogue d {
  Node a {
    speaker: Character.stranger  // OSF021: write speaker: stranger
    text: "Hello."
  }
}
```

### 5.2 Options

```osf fragment
Option node_03_option_01 {
  text: "Yes, you are."
  requires: { is_dreaming: false }  // shown only when every value matches
  set: { is_dreaming: true }        // applied when chosen
  next: node_04
}
```

| Field      | Value | OSF |
|------------|-------|-----|
| `text`     | Text. Required (OSF110). | `content.text` |
| `next`     | As for nodes; omit to end the dialogue. | `next` |
| `requires` | Value map of conditions; all must be met. Several maps joined by `or` are alternatives. | `requires` |
| `set`      | Value map of changes. | `set` |

Option ids are unique in their node (OSF119). Every name in `set` and `requires` is a
Variable (OSF200) and every value has its type (OSF201). A variable is named at most once
in `set` (OSF014). `requires` may compare it several times, as in
`{ gold >= 3, gold <= 10 }`, but the same condition twice is OSF014.

A `Boolean` is set with `name: true` and required with `name: false`. Numbers can do more:

```osf fragment
Option buy_bed {
  text: "Here are three coins."
  requires: { gold >= 3, trust != -1 }  // != < <= > >= compare; name: 3 is "equal to 3"
  set: { gold -= 3, trust += 1 }        // += adds, -= subtracts; name: 3 sets it to 3
}
```

| Entry | Where | Meaning | OSF |
|-------|-------|---------|-----|
| `name: value` | `set` | Set the variable. | the value |
| `name += n`, `name -= n` | `set` | Add or subtract `n`, which is 0 or more (OSF206). | `{ operation: add, value: n }`, `{ operation: subtract, value: n }` |
| `name: value` | `requires` | The variable equals the value. | the value |
| `name != n`, `name < n`, `name <= n`, `name > n`, `name >= n` | `requires` | The comparison is true. | `{ operator: ">=", value: n }` |

"Exactly 4 gold" is `requires: { gold: 4 }`.

For alternatives, write several maps joined by `or`. Every entry inside one `{ }` must be
met, and one met map is enough:

```osf fragment
Option buy_lantern {
  text: "I will take the lantern."
  requires:
    { gold >= 3, gold <= 20, has_lantern: false }
    or { god_mode: true }
}
```

The maps can also stay on one line, as `requires: { gold >= 3 } or { god_mode: true }`.
`osf fmt` keeps the line breaks you chose and indents maps written under `requires` one
level. `osf convert -o osf` writes alternatives one map per line, as above. A conditional
`next` takes the same form:

```osf fragment
next: dark requires
  { happiness < 0 }
  or { money: 0, god_mode: false }
```

In YAML and JSON a `requires` that repeats a variable or has alternatives is written in
the full form with `all` and `any` (spec §2.5.1).

A `requires` map can also check what was played: `rumours.stateData.played: true` once the
node `rumours` was entered, `hub.options.room.selected: false` while option `room` of node `hub` was
never chosen. Put `Dialogue.<id>.` in front for a node of another dialogue, as in
`Dialogue.cellar.door.stateData.played: false` (spec §2.5.3).

```osf fragment
Option room {
  text: "And about that room?"
  requires: { rumours.stateData.played: true }  // only after hearing the rumours
  next: room
}
```

Operations and comparisons are for `Number` and `UnsignedNumber` only (OSF205 on a
`Boolean`). An `UnsignedNumber` never goes below 0: subtracting more than it holds leaves
0, and setting it to a negative value is OSF201. A result stays within 32 bits.

### 5.3 Conditional next

`next` followed by `requires` and a value map of conditions is a branch. A node or an
option can have any number of branches and at most one plain `next`:

```osf fragment
Node guard {
  text: "The guard looks you over."
  next: rich requires { gold >= 10 }       // tried first
  next: friend requires { trusted: true }  // then this one
  next: away                               // when no branch matches
}
```

Branches are tried in the order they are written, and the first whose conditions are all
met is followed. The plain `next` is the fallback wherever it is written; without one, the
dialogue ends when no branch matches. The conditions are those of an option's `requires`
(§5.2). This is `next` as a list of `{ to, requires }` in YAML (spec §2.5.2).

`next: hub.options` goes back to the options of node `hub` without showing its line or
applying its `set` again, for a question hub where each answer returns to the questions
(spec §2.5.4). The node must have options (OSF128).

```osf fragment
Node rumours {
  text: "They say the cellar is haunted."
  next: hub.options  // back to the questions, without the greeting
}
```

### 5.4 Playing

How a dialogue runs (start node, `set`, `requires`, `next`, the end) is defined once for
both formats in [spec §5](spec.md#5-playing-a-dialogue); `osf play` and the
editor's Play panel follow it.

## 6. Text references

Any text can show another resource's text, using the same form as OSF:

```osf fragment
title: "{{ Character.stranger }}"
text: "The yard of {{ Document.east_gate }}."
text: "Dreaming: {{ Variable.is_dreaming }}"
text: "Literal \{{ braces }}"
```

The rules are those of [spec §4](spec.md#4-references-inside-text): the field
defaults to `name`; a Character and a Story have `name` and `description`, a Document and a
Dialogue `name`; a Variable reference takes no field and shows the
variable's current value. A reference to something that does not exist is OSF120, to a
field that is not there OSF121. References stay in the built resources and are filled in
when the text is shown, in the language it is shown in.

## 7. Translations

### 7.1 Folder layout

Translations live in **one folder per language** under `locales/`, mirroring the story
folder, so a translator works inside a single folder and never opens story files:

```
versions.osf
story.osf
characters/characters.osf
arc_1_war/01_intro_scene.osf
locales/
  Rus/
    story.osf
    characters/characters.osf
    arc_1_war/01_intro_scene.osf
  Jpn/
    ...
```

The folder name is the locale id from the `I18n` block. Mirroring the paths is a
convention for people, not a rule the compiler needs: a translation is matched to its
source by ids, not by file name.

### 7.2 `Locale` blocks

A translation file has the same shape as its source, with only the text fields kept:

```osf
Locale Rus {
  Character stranger {
    name = "Незнакомец"
  }

  Dialogue 01_intro_scene {
    Node node_01 {
      title: "..."
      text: "Где я?"
    }

    Node node_03 {
      text: "Я сплю?"

      Option node_03_option_01 { text: "Да, спишь." }
      Option node_03_option_02 { text: "Нет. Глупец" }
    }
  }
}
```

- Allowed inside `Locale`: `Story`, `Character`, `Document` and `Dialogue` blocks with
  their text fields only (`name`, `description`, `title`, `text`), and the `Node` and
  `Option` blocks that hold them. `next`, `speaker`, `set`, `requires` and `start` are
  errors (OSF309).
- Anything left out is untranslated: error OSF307 for `enforce = true` locales, otherwise
  one warning OSF901 per locale, and the source text is shown instead.
- A `Locale` block in a file under `locales/<X>/` must be `Locale X` (OSF308). A locale
  the `I18n` block does not list is OSF303.

Every rule from [spec §3.4](spec.md#34-rules) applies: unknown keys are errors
(OSF304, catches renamed nodes), the same text translated twice is an error (OSF305), and
`{{ }}` references must match the source (OSF306). In an editor, the language server's
code actions add missing translations to the right `Locale` block.

## 8. Compiling to OSF

| Language                              | OSF resource |
|---------------------------------------|--------------|
| `Versions`                            | `plugins/v1 Configuration` listing the non-built-in plugins (e.g. `[i18n/v1]`), written only when there are some; every other resource's `apiVersion` |
| `Story`                               | `story/v1 Story` |
| `Character`                           | `story/v1 Character` |
| `Document`                            | `story/v1 Document` |
| `Variable`                            | `story/v1 Variable` (`type` is `boolean`, `number` or `unsignedNumber`) |
| `Dialogue`                            | `dialogue/v1 Dialogue`, with `startNode` always written (the first node when `start` is omitted) |
| `I18n`                                | `i18n/v1 Configuration`; each `Locale` line becomes `locales.<display name>: { id, enforce }` |
| `Locale` block                        | `i18n/v1 Locale`; keys are the OSF translation keys, e.g. `Dialogue.01_intro_scene.nodes.node_03.options.node_03_option_01.content.text` |

A `speaker: stranger` compiles to `speaker: Character.stranger`. Text from `"""` blocks
ends with a newline, like a YAML `|` block.

The [`example/osf`](../example/osf) folder is the example project in this language. It
compiles to resources identical to the YAML example in [`example/yaml`](../example/yaml),
and both pass `osf validate` with 0 errors.

Every error `osf validate` can report, from a stray character in the lexer to a missing
translation, has a stable code: OSF001-OSF099 for lexing and parsing, OSF100-OSF199 for
structure and references, OSF200-OSF299 for variables, OSF300-OSF399 for translations,
OSF900-OSF999 for warnings. The table is [`diagnostics.md`](diagnostics.md).

## 9. Grammar

```
file        = { statement } ;
statement   = versions | story | i18n | character | document | variable | dialogue | locale ;
versions    = "Versions" "{" { id "=" string } "}" ;
story       = "Story" id "{" { ("name" | "description") "=" string | "start" "=" "Dialogue" "." id } "}" ;
i18n        = "I18n" "{" { "source" "=" id | "Locale" id string [ "{" "enforce" "=" bool "}" ] } "}" ;
character   = "Character" id "{" { ("name" | "description") "=" string } "}" ;
document    = "Document" id "{" { "name" "=" string } "}" ;
variable    = "Variable" id ":" ("Boolean" | "Number" | "UnsignedNumber") "=" value [ "{" "description" "=" string "}" ] ;
dialogue    = "Dialogue" id "{" { "name" "=" string | "start" id | node } "}" ;
node        = "Node" id "{" { text | "speaker" ":" id | next | "set" ":" sets | option } "}" ;
option      = "Option" id "{" { text | next | "requires" ":" requires | "set" ":" sets } "}" ;
text        = ("text" | "title") ":" string ;          (title on nodes only)
next        = "next" ":" ( id [ "." "options" ] | "Dialogue" "." id ) [ "requires" requires ] ;
sets        = "{" { id ( ":" value | ("+=" | "-=") number ) [ "," ] } "}" ;
requires    = conditions { "or" conditions } ;
conditions  = "{" { ( id ( ":" value | ("!=" | "<" | "<=" | ">" | ">=") number ) | played ":" bool ) [ "," ] } "}" ;
played      = [ "Dialogue" "." id "." ] id "." ( "stateData" "." "played" | "options" "." id "." "selected" ) ;
locale      = "Locale" id "{" { story | character | document | dialogue } "}" ;   (text fields only)
value       = number | bool ;
bool        = "true" | "false" ;
string      = '"..."' | '"""..."""' ;
```

`;` may appear between any two statements. Comments may appear between any two tokens.

## 10. Settled questions

These were open in earlier drafts.

1. **Implementation.** `osf` is written in Go (`src/cli/`), with the language server
   (`osf lsp`) and the VS Code extension on top.
2. **`=` and `:`.** Both stay (§1, rule 4): `=` for resource fields, `:` for node and
   option content. `osf fmt` keeps whichever is right and OSF015 names the fix.
3. **One line per node.** A node holds one line of dialogue; a sequence is a chain of
   nodes linked by `next`. Generated ids would make translation keys shift when a line is
   inserted, so the language does not generate them.
4. **What OSF v1 leaves out** (grammatical agreement, inline media, generated ids, engine
   export) is listed in [spec §6](spec.md#6-not-in-v1) and applies here too.

## 11. Canonical layout

Field order and line breaks never change meaning (§1), apart from the order of branching `next` lines (§5.3), and `osf fmt` keeps the line
breaks you chose. `osf fmt` normalises the rest: two spaces per block level, one space
between tokens, at most one blank line in a row, `=` aligned in runs of consecutive
`name = value` lines, trailing comments aligned in runs of consecutive lines, and `"""`
text indented one level deeper than the line that opens it. `osf fmt -check` reports a
file that is not formatted.

One layout is nevertheless *canonical*: it is what `osf convert -o osf` writes,
and a project written in it survives an `.osf` to YAML to `.osf` round trip byte for
byte. The example projects use it.

- Top-level blocks are separated by one blank line, except consecutive `Variable` lines,
  which stay together. Order within a file: `Versions`, `Story`, `I18n`, `Character`,
  `Document`, `Variable`, `Dialogue`, `Locale`.
- `Versions`: `language`, a blank line, then `story`, `dialogue` and the plugins in name
  order.
- `Story`: `name`, `description`, `start`. `Character`: `name`, `description`. `Document`: `name`.
  `I18n`: `source`, then one `Locale` line per language, with `{ enforce = true }` only
  when it is true.
- `Variable id: Type = value` on one line, followed by `{ description = "..." }` when
  it has a description.
- `Dialogue`: `name`, then `start` only when the start node is not the first node, then
  `Node` blocks separated by one blank line. No blank line after an opening brace; a
  blank line between the field lines and the first nested block.
- `Node`: `speaker`, `title`, `text`, `set`, the branching `next` lines in order, the
  plain `next`, then `Option` blocks, each preceded
  by a blank line. `Option`: `text`, `requires`, `set`, then `next` lines as for a node. Value maps are written
  `{ a: 1, b: true, c += 2 }`, with commas.
- Text with line breaks is written as a `"""` block, indented one level deeper than the
  field, when it ends with a line break and the block would read back unchanged (no
  trailing spaces, at least one line flush left, no `"""` inside); otherwise as a
  `"..."` string with `\n`. A line starting with `//` inside a block is written `\//`.
  Literal braces stay `\{{`.
- `Locale` blocks hold `Story`, `Character`, `Document`, then `Dialogue` blocks in that order, each
  in the order its texts first appear, separated by blank lines. The same field order
  applies, and an `Option` that has only `text` is written on one line:
  `Option id { text: "..." }`; consecutive one-line options are not separated by blank
  lines.
- Comments are kept as written (§11.2); `osf fmt` aligns trailing ones.

### 11.1 YAML, JSON and `.osf`

`osf convert` reads a project in any of the three forms and writes it in the one named
by `-o`:

| Command | Writes |
|---|---|
| `osf convert` or `osf convert -o json` | the resources of §8 as JSON, one per file |
| `osf convert -o yaml` | the same files as YAML, with the comments of the `.osf` source |
| `osf convert -o osf` | `.osf` source in the canonical layout |

Converting to the form a project already has is allowed and rewrites it in the standard
formatting; an `.osf` project keeps its files.

A YAML file written by `osf convert` names its source `.osf` file in its first comment
line and goes back there, so `.osf` to YAML to `.osf` returns the original files.
Anything else, JSON included, is laid out as `versions.osf`, `story.osf` (`Story` and
`I18n`), `characters.osf`, `documents.osf`, `variables.osf`, one
`<folder>/<dialogue id>.osf` per dialogue, and `locales/<Locale>/<path>.osf` for
translations.

### 11.2 Comments through YAML

Comments travel with the statement they belong to: the comment lines directly above a
statement, the lines above it that stand apart (a blank line between), and the comment
at the end of its last line. `osf convert -o yaml` writes them as YAML `#` comments in the same
places (a `// text` line becomes `# text`, each line of a `/* */` comment becomes `# `
plus the line), and `osf convert -o osf` reads them back, so YAML comments written by hand in
those places come back as `//` comments too.

What a round trip does not keep:

- a comment with no statement after it in its block (before a closing `}`, or at the
  end of the file);
- writer's notes inside `"""` text (they are not part of the text);
- the key order of hand-written YAML (the YAML form is regenerated), the
  `# yaml-language-server` line and the `# Generated by osf convert` line;
- which `.osf` file held a `Versions` block (it always returns to `versions.osf`);
- every comment, when the resources are JSON: a JSON project converts into the default
  layout above.
