// Package osf holds the in-memory model of an Open Story project. Both the .osf
// language parser and the YAML loader produce it, the checker validates it, and
// the YAML writer turns it into OSF resource files.
package osf

import (
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/diag"
)

// Comments are the comments written around one statement, kept so that a
// conversion can write them back. Head holds the lines above the statement
// as written: "// ..." lines, the lines of a "/* */" comment, and "" for a
// blank line, so a trailing "" means the comment stood apart from the
// statement. Trail is the comment at the end of the statement's last line,
// as written.
type Comments struct {
	Head  []string
	Trail string
}

// Str is a string value with the place it was written: Pos is its first
// character, End the position after its last one. Comments are those of the
// statement the string is the value of.
type Str struct {
	Value    string
	Pos      diag.Pos
	End      diag.Pos
	Comments Comments
}

type VarType int

const (
	Boolean        VarType = iota
	Number                 // a whole number, can be negative
	UnsignedNumber         // a whole number that never goes below 0
)

// String is the type as written in YAML: boolean, number, unsignedNumber.
func (t VarType) String() string {
	return [...]string{"boolean", "number", "unsignedNumber"}[t]
}

// Name is the type as written in .osf: Boolean, Number, UnsignedNumber.
func (t VarType) Name() string {
	return [...]string{"Boolean", "Number", "UnsignedNumber"}[t]
}

// VarTypes lists every variable type, in the order the docs show them.
var VarTypes = []VarType{Boolean, Number, UnsignedNumber}

// IsNumber is true for both number types.
func (t VarType) IsNumber() bool { return t != Boolean }

func (t VarType) Accepts(v Value) bool {
	switch t {
	case Boolean:
		return v.Kind == Bool
	case UnsignedNumber:
		return v.Kind == Int && v.Int >= 0
	}
	return v.Kind == Int
}

// Numbers of both types fit in a 32-bit signed integer.
const (
	MinNumber = -1 << 31
	MaxNumber = 1<<31 - 1
)

type ValueKind int

const (
	Bool ValueKind = iota
	Int
)

func (k ValueKind) String() string {
	return [...]string{"boolean", "number"}[k]
}

// Value is a variable value: a boolean or a whole number.
type Value struct {
	Kind ValueKind
	Bool bool
	Int  int64
}

func (v Value) String() string {
	if v.Kind == Bool {
		return fmt.Sprint(v.Bool)
	}
	return fmt.Sprint(v.Int)
}

// Op is what a `set` entry does to its variable or how a `requires` entry
// compares it. The zero value is the plain form: set the value, or equal to it.
type Op string

const (
	OpSet      Op = ""
	OpAdd      Op = "add"
	OpSubtract Op = "subtract"

	OpEqual        Op = ""
	OpNotEqual     Op = "!="
	OpLess         Op = "<"
	OpLessEqual    Op = "<="
	OpGreater      Op = ">"
	OpGreaterEqual Op = ">="
)

// SetOps and CompareOps are the operations with a name in YAML, in the order
// the docs show them.
var (
	SetOps     = []Op{OpAdd, OpSubtract}
	CompareOps = []Op{OpNotEqual, OpLess, OpLessEqual, OpGreater, OpGreaterEqual}
)

// Assign is one entry of a `set` or `requires` map.
type Assign struct {
	Name  string
	Op    Op
	Value Value
	Pos   diag.Pos
	End   diag.Pos
}

// Apply returns the value of a variable of type t holding cur after this
// `set` entry. Results stay within 32 bits, and an unsigned number stops at 0.
func (a Assign) Apply(t VarType, cur Value) Value {
	n := a.Value.Int
	switch a.Op {
	case OpAdd:
		n = cur.Int + n
	case OpSubtract:
		n = cur.Int - n
	default:
		return a.Value
	}
	if t == UnsignedNumber && n < 0 {
		n = 0
	}
	return Value{Kind: Int, Int: min(max(n, MinNumber), MaxNumber)}
}

// Holds reports whether cur meets this `requires` entry.
func (a Assign) Holds(cur Value) bool {
	switch a.Op {
	case OpNotEqual:
		return cur != a.Value
	case OpLess:
		return cur.Int < a.Value.Int
	case OpLessEqual:
		return cur.Int <= a.Value.Int
	case OpGreater:
		return cur.Int > a.Value.Int
	case OpGreaterEqual:
		return cur.Int >= a.Value.Int
	}
	return cur == a.Value
}

// SetString is a `set` entry as written in .osf: `gold: 5`, `gold += 5`.
func (a Assign) SetString() string {
	switch a.Op {
	case OpAdd:
		return a.Name + " += " + a.Value.String()
	case OpSubtract:
		return a.Name + " -= " + a.Value.String()
	}
	return a.Name + ": " + a.Value.String()
}

// RequiresString is a `requires` entry as written in .osf: `gold: 5`, `gold >= 5`.
func (a Assign) RequiresString() string {
	if a.Op == OpEqual {
		return a.Name + ": " + a.Value.String()
	}
	return a.Name + " " + string(a.Op) + " " + a.Value.String()
}

type Project struct {
	// APIVersions maps a plugin name (story, dialogue, i18n) to its version (v1).
	APIVersions map[string]string
	// Plugins lists the non-built-in plugins, e.g. [i18n/v1].
	Plugins    []string
	PluginsPos diag.Pos
	PluginsSet bool // a plugins/v1 Configuration exists
	// PluginsComments are those of the Versions block (or the plugins
	// Configuration); PluginComments those of each entry of Plugins.
	PluginsComments Comments
	PluginComments  map[string]Comments

	Stories    []*Story
	Characters []*Character
	Documents  []*Document
	Variables  []*Variable
	Dialogues  []*Dialogue
	I18n       []*I18n
	Locales    []*Locale
}

// Resource fields shared by every kind: Pos is where it is defined (the
// block keyword), End is the end of its header (after the id), File is the
// project-relative source file it came from, Comments those around the
// block's header. IDPos is the first character of the id itself (zero when
// the loader had no id); see IDSpan.
type Story struct {
	ID          string
	Name        *Str
	Description *Str
	Start       *Str // Dialogue.<id>
	Pos         diag.Pos
	End         diag.Pos
	IDPos       diag.Pos
	File        string
	Comments    Comments
}

// Character is someone who speaks in dialogues. Name and Description are
// translatable.
type Character struct {
	ID          string
	Name        *Str
	Description *Str
	Pos         diag.Pos
	End         diag.Pos
	IDPos       diag.Pos
	File        string
	Comments    Comments
}

// Document is a piece of the story world that dialogues refer to (a place, an
// item, a faction, a lore entry). For now it has only an id and a name.
type Document struct {
	ID       string
	Name     *Str
	Pos      diag.Pos
	End      diag.Pos
	IDPos    diag.Pos
	File     string
	Comments Comments
}

// Variable is story state. Description is a note for writers and
// programmers; it is not shown to players and is not translated.
type Variable struct {
	ID          string
	Type        VarType
	Default     Value
	Description *Str
	Pos         diag.Pos
	End         diag.Pos
	IDPos       diag.Pos
	File        string
	Comments    Comments
}

type Dialogue struct {
	ID        string
	Name      *Str
	StartNode *Str
	Nodes     []*NodeModel
	Pos       diag.Pos
	End       diag.Pos
	IDPos     diag.Pos
	File      string
	Comments  Comments
}

type NodeModel struct {
	ID          string
	Speaker     *Str // Character.<id>
	Title       *Str
	Text        *Str
	Next        *Str      // followed when no branch matches
	Branches    []*Branch // conditional next, tried in order before Next
	Set         []Assign
	Options     []*OptionModel
	Pos         diag.Pos
	End         diag.Pos
	IDPos       diag.Pos
	Comments    Comments
	SetComments Comments // around the set statement
}

type OptionModel struct {
	ID               string
	Text             *Str
	Next             *Str       // followed when no branch matches
	Branches         []*Branch  // conditional next, tried in order before Next
	Requires         []Assign   // all must be met
	RequiresOr       [][]Assign // alternatives: the option also shows when one of these groups is fully met
	Set              []Assign
	Pos              diag.Pos
	End              diag.Pos
	IDPos            diag.Pos
	Comments         Comments
	RequiresComments Comments
	SetComments      Comments
}

// Groups returns the alternative groups of a requires: the first group, then
// the ones written after `or`. It is met when every condition of one group is.
func Groups(first []Assign, or [][]Assign) [][]Assign {
	if len(first) == 0 && len(or) == 0 {
		return nil
	}
	return append([][]Assign{first}, or...)
}

// Branch is a conditional next: To is followed when every Requires entry is
// met. The branches of a node or option are tried in order; the first that
// matches wins, and Next is followed when none does.
type Branch struct {
	To         *Str
	Requires   []Assign
	RequiresOr [][]Assign // alternatives, as for an option
	Comments   Comments
}

// Targets returns where next can lead: each branch's target, then Next.
func Targets(branches []*Branch, next *Str) []*Str {
	var out []*Str
	for _, b := range branches {
		if b.To != nil {
			out = append(out, b.To)
		}
	}
	if next != nil {
		out = append(out, next)
	}
	return out
}

// IDSpan returns id as a Str written at pos. An id is one token on one line,
// so it ends utf8.RuneCountInString(id) columns after it starts.
func IDSpan(id string, pos diag.Pos) Str {
	end := pos
	end.Col += utf8.RuneCountInString(id)
	return Str{Value: id, Pos: pos, End: end}
}

type I18n struct {
	Source   *Str
	Locales  []LocaleDef
	Pos      diag.Pos
	End      diag.Pos
	File     string
	Comments Comments
}

type LocaleDef struct {
	Name     string // display name, e.g. Russian
	ID       string // locale id, e.g. Rus
	Enforce  bool
	Pos      diag.Pos
	End      diag.Pos
	Comments Comments
}

type Locale struct {
	Language Str
	Values   []LocaleValue
	Pos      diag.Pos
	End      diag.Pos
	File     string
	Comments Comments
}

// LocaleValue is one translation: Key is a translation key such as
// Dialogue.01_intro_scene.nodes.node_01.content.text. IDs are the resource
// ids the key names, in key order (dialogue, node, option; or the story or
// document), each where it is written: in a YAML key they sit inside the
// key scalar, in an .osf Locale block they are the ids of the nested blocks.
type LocaleValue struct {
	Key  string
	Text Str
	IDs  []Str
}

var (
	IDPattern     = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*$`)
	LocalePattern = regexp.MustCompile(`^[A-Z][a-z]{2}$`)
)

type TextEntry struct {
	Key  string
	Text *Str
}

// Texts lists every translatable text of the project's story resources, in
// definition order, keyed by translation key (spec §3.3).
func (p *Project) Texts() []TextEntry {
	var out []TextEntry
	add := func(key string, s *Str) {
		if s != nil {
			out = append(out, TextEntry{key, s})
		}
	}
	for _, s := range p.Stories {
		add("Story."+s.ID+".name", s.Name)
		add("Story."+s.ID+".description", s.Description)
	}
	for _, c := range p.Characters {
		add("Character."+c.ID+".name", c.Name)
		add("Character."+c.ID+".description", c.Description)
	}
	for _, d := range p.Documents {
		add("Document."+d.ID+".name", d.Name)
	}
	for _, d := range p.Dialogues {
		DialogueTexts(d, add)
	}
	return out
}

// DialogueTexts calls add for every text in d with its translation key.
func DialogueTexts(d *Dialogue, add func(key string, s *Str)) {
	k := "Dialogue." + d.ID
	add(k+".name", d.Name)
	for _, n := range d.Nodes {
		nk := k + ".nodes." + n.ID
		add(nk+".content.title", n.Title)
		add(nk+".content.text", n.Text)
		for _, o := range n.Options {
			add(nk+".options."+o.ID+".content.text", o.Text)
		}
	}
}

// Supported versions of the language and of each plugin, newest last.
var Supported = map[string][]string{
	"language": {"1"},
	"story":    {"v1"},
	"dialogue": {"v1"},
	"i18n":     {"v1"},
}
