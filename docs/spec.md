# Open Story format, v1

This page defines what an Open Story (OSF) project contains. The machine-readable
version is [`schema/osf-v1.schema.json`](../schema/osf-v1.schema.json). `osf validate`
checks the schema and everything a schema cannot express, and reports every rule below
with a stable code (`OSF112`, ...); the full list is [`diagnostics.md`](diagnostics.md). The same projects can be written by hand in
the [OSF language](language.md) (`.osf` files), which compiles to these
resources.

Code blocks here are checked by a test: a block fenced `yaml` is a whole
resource file that `osf` loads without an error; `yaml fragment` marks part of one.

## 1. Files

A project is a folder. Every `*.yaml` and `*.json` file in it is loaded recursively
(folders whose name starts with `.` are skipped), and which file a resource lives in
does not matter. JSON files have the same content as YAML ones; `osf convert` writes JSON
by default and YAML with `-o yaml`. Each file holds exactly one resource (OSF031 for an empty file, OSF032 for
several YAML documents):

```yaml fragment
apiVersion: dialogue/v1   # <plugin>/<version>
kind: Dialogue            # what this resource is
data:                     # the resource itself
  id: 01_intro_scene
```

| apiVersion    | kinds                            | built in |
|---------------|----------------------------------|----------|
| `story/v1`    | `Story`, `Character`, `Document`, `Variable` | yes |
| `dialogue/v1` | `Dialogue`                       | yes |
| `i18n/v1`     | `Configuration`, `Locale`        | no, list it in `plugins/v1 Configuration` |
| `plugins/v1`  | `Configuration`                  | yes |

An unknown `apiVersion` is OSF102, another version of a known plugin OSF103, a kind its
`apiVersion` does not have OSF123. Inside a resource, an unknown key is OSF013, a missing
required one OSF110, a repeated key OSF014 (YAML parsers silently keep the last one,
which is why `osf` checks it), and a value of the wrong shape OSF122. A file that is not
valid YAML or JSON is OSF030.

**Ids.** `data.id` is unique per kind (OSF109): a Character and a Document may share an
id, two Characters may not. Ids are ASCII letters, digits, `_` and `-`, not starting with
`-` (OSF108); no dots, because dots separate the parts of references and translation
keys. Node and option ids are unique within their parent.

**References.** Other resources are referenced as `Kind.id`, for example
`Dialogue.02_shadows_speak` or `Character.stranger`. Variables are referenced by their
bare id in `set` and `requires`.

Add this first line to any file to get completion and validation in editors that use the
YAML language server (VS Code, JetBrains, Neovim):

```yaml fragment
# yaml-language-server: $schema=<relative path>/schema/osf-v1.schema.json
```

## 2. Kinds

### 2.1 `Story`

Exactly one per project (OSF106 for none, OSF107 for more). `name` is required (OSF110).

```yaml
apiVersion: story/v1
kind: Story
data:
  id: LDOD
  name: "Leviathan: The Last Day of The Decade"   # text
  description: ...                                 # text, optional
  start: Dialogue.01_intro_scene                   # optional: the first dialogue
```

`start` must be `Dialogue.<id>` (OSF021) of an existing dialogue (OSF111). Without it, a
player starts at the first dialogue in file order (files sorted by path).

### 2.2 `Character`

Someone who speaks in dialogues. Any Character can be a dialogue's speaker. `name` is
required.

```yaml
apiVersion: story/v1
kind: Character
data:
  id: stranger
  name: Stranger          # text
  description: ...        # text, optional
```

### 2.3 `Document`

A piece of the story world that dialogues refer to: a place, an item, a faction, a lore
entry. For now a document has only an id and a name (required); its content is a later
extension (§6). A Document is not a speaker.

```yaml
apiVersion: story/v1
kind: Document
data:
  id: old_lighthouse
  name: The Old Lighthouse   # text
```

### 2.4 `Variable`

Story state that options can require and nodes or options can set. `id`, `type` and
`default` are required.

```yaml
apiVersion: story/v1
kind: Variable
data:
  id: is_dreaming
  type: boolean           # boolean | number | unsignedNumber
  default: false          # must match type
  description: True while the player believes they are asleep.   # optional
```

`description` is a note for writers and programmers about what the variable tracks. It
is not shown to players, is not translated and takes no `{{ }}` references.

| type | holds | example `default` |
|------|-------|-------------------|
| `boolean` | `true` or `false` | `false` |
| `number` | a whole number, can be negative | `-3` |
| `unsignedNumber` | a whole number that never goes below 0 | `0` |

An unknown `type` is OSF203; a `default` of another type, or a negative one for an
`unsignedNumber`, is OSF202. Numbers of both types are whole and fit in a 32-bit signed
integer, -2147483648 to 2147483647 (OSF204).

### 2.5 `Dialogue`

```yaml
apiVersion: dialogue/v1
kind: Dialogue
data:
  id: 01_intro_scene
  name: ...                       # text, optional
  startNode: node_01              # required: where the dialogue starts
  nodes:
    node_01:
      next: node_03               # auto-continue
      content:
        speaker: Character.stranger   # optional
        title: ...                    # optional text: the shown speaker name, overrides speaker
        text: Where am I?
    node_03:
      set: { is_dreaming: false }     # set when the node is reached
      content:
        text: Am I dreaming?
      options:                        # player choices
        node_03_option_01:
          requires: { is_dreaming: false }   # shown only when all match
          set: { is_dreaming: true }         # set when chosen
          next: node_04
          content:
            text: Yes, you are.
        node_03_option_02:
          next: Dialogue.02_shadows_speak    # jump to another dialogue's startNode
          content:
            text: No. Fool
    node_04:
      content:
        text: Strange.
```

Rules:

- `id`, `startNode` and `nodes` are required. `startNode` is one of the nodes (OSF115);
  `nodes` is a mapping with at least one node (OSF116).
- Every node and option has `content` with a `text` (OSF110). A node's `content` may
  also have `speaker` and `title`; an option's has only `text`.
- `speaker` is `Character.<id>` (OSF021) of an existing Character (OSF112). `title`
  overrides the speaker's name where the line is shown; `title: null` is the same as no
  title.
- `next` is a node id in the same dialogue (OSF113), `<node>.options` (§2.5.4),
  `Dialogue.<id>` (OSF114), or a list of conditional branches (§2.5.2).
- A node has `next` or `options`, not both (OSF117). A node with neither ends the
  dialogue; so does an option without `next`. `options`, when present, holds at least
  one option.
- `set` and `requires` map Variable ids (OSF200) to values of the variable's type
  (OSF201), or to a number operation (§2.5.1). `requires` belongs to options and to
  branches of `next` (§2.5.2), and can also check what was played (§2.5.3).
- Node and option ids are YAML keys, so a repeated key is an error (OSF118, OSF119).
- A node that no `next` leads to and that is not `startNode` is warning OSF900.

#### 2.5.1 Changing and comparing numbers

In `set` and `requires` a plain value means "set to this value" and "equals this value".
A `number` or `unsignedNumber` variable can instead take a mapping with an operation:

```yaml fragment
set:
  is_dreaming: true                       # set
  gold: { operation: subtract, value: 3 } # add | subtract
requires:
  is_dreaming: false                      # equals
  gold: { operator: ">=", value: 3 }      # != | < | <= | > | >=
```

- `operation` is `add` or `subtract`; its `value` is the amount, 0 or more (OSF206).
- `operator` is `!=`, `<`, `<=`, `>` or `>=`; the condition is met when
  `variable operator value` is true. Equality is the plain form, without a mapping.
- Both are for numbers only: on a `boolean` variable they are OSF205.
- An `unsignedNumber` never goes below 0: subtracting more than it holds leaves it at 0,
  and setting it to a negative value is OSF201. The result of `add` and `subtract` stays
  within the 32-bit range.
- A variable appears at most once in one `set` or `requires` mapping, since it is the key.
  In `requires` it can take a list of conditions instead, all of which must be met; the
  same operator twice in one list is OSF014:

```yaml fragment
requires:
  gold:                                   # 3 to 10 gold
    - { operator: ">=", value: 3 }
    - { operator: "<=", value: 10 }
```

#### 2.5.2 Conditional next

`next` on a node or an option can instead be a list of branches. Each branch has a target
`to` and the same `requires` an option has. The branches are tried in order and the first
whose conditions are all met is followed. The last branch may leave out `requires`: it is
taken when no other matches. When no branch matches and there is no such fallback, the
dialogue ends there, as it does for a node without `next`.

```yaml fragment
next:
  - to: rich                              # tried first
    requires: { gold: { operator: ">=", value: 10 } }
  - to: friend
    requires: { trusted: true }
  - to: away                              # when neither matches
```

- `to` is required and is a node id or `Dialogue.<id>`, as for a plain `next`.
- Only the last branch may leave out `requires`; an earlier one would hide the branches
  after it (OSF125).
- A plain `next: away` is the same as a list holding only `- to: away`.

#### 2.5.3 Played and selected conditions

Besides variables, `requires` can check the play history:

```yaml fragment
requires:
  rumours.played: true                    # node rumours of this dialogue was entered
  hub.room.selected: false                # option room of node hub was never chosen
  Dialogue.cellar.door.played: false      # a node of another dialogue
```

- `<node>.played` is true once the node was entered, at any time in the play.
- `<node>.<option>.selected` is true once that option was chosen.
- Both name a node of the same dialogue; `Dialogue.<id>.` in front names one in another
  dialogue (OSF114 when it does not exist). An unknown node is OSF113, an unknown option
  OSF126.
- They are compared with `true` or `false` only, and belong in `requires`, never in
  `set` (OSF127).

#### 2.5.4 Back to a node's options

`next: <node>.options` offers the options of a node of the same dialogue again, without
showing its line or applying its `set` again. It is how a question hub is written: each
answer goes back to the questions, not to the greeting before them.

```yaml fragment
nodes:
  rumours:
    next: hub.options          # back to the questions of node hub
    content:
      text: They say the cellar is haunted.
```

The node must exist (OSF113) and have options (OSF128). `<node>.options` can be the
target of any `next`, including a branch.

### 2.6 `plugins/v1 Configuration`

```yaml
apiVersion: plugins/v1
kind: Configuration
data:
  plugins: [i18n/v1]
```

It lists the plugins that are not built in; it is needed only when the project uses
one, and there is at most one per project (OSF101). Each entry is `<plugin>/<version>`
that `osf` knows: `i18n/v1` in this version (OSF102 for an unknown plugin, OSF103 for an
unsupported version, OSF124 for one listed twice). Using `i18n/v1` resources without it
is OSF105.

## 3. Translations (`i18n/v1`)

### 3.1 Configuration

At most one per project (OSF300).

```yaml
apiVersion: i18n/v1
kind: Configuration
data:
  source: Eng             # optional: locale the story files are written in
  locales:                # display name -> locale
    Russian: { id: Rus, enforce: true }    # missing translations are errors
    Japanese: { id: Jpn, enforce: false }  # missing translations are one warning
```

Locale ids are three letters, capitalised (`Eng`, `Rus`, `Jpn`; OSF301). `id` is
required, `enforce` defaults to `false`, and a locale listed twice is OSF302.

### 3.2 `Locale`

Translations live in their own files, in one folder per language under `locales/`, so
a translator works inside a single folder and never edits story files. Convention: the
folder is the locale id and mirrors the story folder, one file per source file:

```
story.yaml
arc_1_war/01_intro_scene.yaml
locales/
  Rus/
    story.yaml
    arc_1_war/01_intro_scene.yaml
```

The layout is for people; translations are matched to their source by key, not by path.

```yaml
apiVersion: i18n/v1
kind: Locale
data:
  language: Rus
  values:
    Story.LDOD.name: "Левиафан: Последний День Декады"
    Dialogue.01_intro_scene.nodes.node_01.content.text: Где я?
    Dialogue.01_intro_scene.nodes.node_03.options.node_03_option_01.content.text: Да, спишь.
```

`language` and `values` are required. Several `Locale` files may hold the same language.

### 3.3 Translation keys

A key is `Kind.id` followed by the path to the text field inside `data`:

| Text                     | Key |
|--------------------------|-----|
| Story name               | `Story.<id>.name` |
| Story description        | `Story.<id>.description` |
| Character name           | `Character.<id>.name` |
| Character description    | `Character.<id>.description` |
| Document name            | `Document.<id>.name` |
| Dialogue name            | `Dialogue.<id>.name` |
| Node text                | `Dialogue.<id>.nodes.<node>.content.text` |
| Node title               | `Dialogue.<id>.nodes.<node>.content.title` |
| Option text              | `Dialogue.<id>.nodes.<node>.options.<option>.content.text` |

Text fields are `name`, `description`, `title` and `text`. Ids, references,
variable values and numbers are never translated. A key that is not of the form
`Kind.id.field...` with `Kind` one of `Story`, `Character`, `Document`, `Dialogue` is OSF310.

### 3.4 Rules

- **Error:** a key that is not a text in the story, which catches renamed nodes (OSF304).
- **Error:** the same key translated twice for one locale (OSF305).
- **Error:** a `Locale` whose language is not in the configuration (OSF303).
- **Error:** a translation whose `{{ }}` references differ from the source text (OSF306).
- **Error** for `enforce: true`, one per missing text (OSF307); **warning** otherwise,
  one per locale with the count (OSF901): a text with no translation. A player shows
  the source text instead.

## 4. References inside text

Text can show another resource's text with `{{ Kind.id.field }}`; the field defaults to
`name`:

```yaml fragment
text: "{{ Character.stranger }} steps out of the dark."
text: "{{ Character.stranger.description }}"
text: "The light of {{ Document.old_lighthouse }} is out."
text: "Dreaming: {{ Variable.is_dreaming }}"
```

| Kind       | Fields |
|------------|--------|
| `Story`    | `name`, `description` |
| `Character` | `name`, `description` |
| `Document` | `name` |
| `Dialogue` | `name` (a dialogue without a name shows its id) |
| `Variable` | none: `{{ Variable.<id> }}` shows the variable's current value |

- A reference to something that does not exist is OSF120; a field the kind does not
  have, or any field on a Variable reference, is OSF121.
- References stay as written in built resources. They are replaced when the text is
  shown (by `osf play`, the editor's Play panel or a game): Story, Character, Document and
  Dialogue references by the referenced text **in the same locale**, so a Russian build shows the
  Russian name; Variable references by the variable's current value. The inserted text
  is used as it is: references inside it are not replaced again.
- `\{{` writes literal braces; it is shown as `{{`.
- A translation must contain the same references as its source, fields included, in any
  order, so word order can change but no reference is lost.

`{{ }}` is used because translation tools (Crowdin, Lokalise, Weblate) already protect it
from being edited.

## 5. Playing a dialogue

This is how a player runs a project; `osf play` and the language server's Play panel
implement it (`src/cli/internal/play`).

1. Play starts at a chosen dialogue, else the Story's `start`, else the first dialogue
   in file order. Every Variable is set to its `default`, and no node has been played
   and no option selected yet (§2.5.3).
2. Play enters the dialogue's `startNode`. Entering a node applies its `set` (in the order written), then shows
   its line: the text, and as the speaker its `title` if it has one, else the speaker
   Character's `name`, else nothing (narration).
3. A node with `next` goes on to it without stopping. When `next` is a list of branches
   (§2.5.2), the first branch whose `requires` are all met by the current values is
   followed; when none is, the dialogue ends. `next: <node>.options` goes straight to
   step 4 for that node: its line is not shown again and its `set` is not applied again. `next: Dialogue.<id>` enters that
   dialogue's `startNode`; variables keep their values across dialogues.
4. A node with options stops and offers the options whose `requires` are all met by the
   current values and history (an option with no `requires` is always offered). Choosing
   one marks it selected, applies its `set` and follows its `next`; an option without
   `next` ends the play. A node counts as played from the moment it is entered.
5. A node with neither `next` nor options ends the play. A node whose options are all
   hidden by `requires` is a dead end: nothing can be chosen and play cannot go on.
   `osf play -coverage` reports dead ends, options no path can show, and loops with no
   exit.
6. Texts are shown in the chosen locale, each one falling back to the source text when
   it has no translation, with `{{ }}` references replaced (§4).

A run of more than 10000 nodes without a choice is an infinite `next` loop and stops
with an error.

## 6. Not in v1

These were open questions in earlier drafts. Version 1 leaves them out; a later plugin
version can add them without changing what v1 files mean.

- **`Document` content**: description and other text, parent document, tags, relations
  to other documents, images, and whether a `Character` should point at its `Document`.
- **Grammatical agreement** (gender, plural, case) for references, e.g. Russian
  "сделал/сделала". Write the variants as separate texts chosen with a Variable and
  `requires`.
- **Inline media** (images) in text. Games attach media by node id.
- **Generated ids** (UUIDs). Ids are the stable identity of nodes and translations; a
  rename is a deliberate change, and the language server's rename updates every
  reference and translation key with it.
- **Engine export.** `osf convert` writes these resources as JSON or YAML
  (`-o yaml`); a game engine reads them directly. There is no engine-specific
  output format.
