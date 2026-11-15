package lang

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/diag"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// Versions is a parsed `Versions { ... }` block.
type Versions struct {
	Entries  []VersionEntry
	Pos      diag.Pos
	Comments osf.Comments
}

type VersionEntry struct {
	Name  string
	Value osf.Str
}

// Unit collects what the files of one project define.
type Unit struct {
	Project  *osf.Project
	Versions []*Versions
}

func NewUnit() *Unit { return &Unit{Project: &osf.Project{}} }

var reserved = map[string]bool{
	"Versions": true, "Story": true, "I18n": true, "Locale": true, "Character": true, "Document": true,
	"Variable": true, "Dialogue": true, "Node": true, "Option": true,
	"Boolean": true, "Number": true, "UnsignedNumber": true,
	"start": true, "true": true, "false": true,
}

// Reserved reports whether word is a reserved word that cannot be an id.
func Reserved(word string) bool { return reserved[word] }

// ReservedWords lists the reserved words, sorted.
func ReservedWords() []string {
	var out []string
	for w := range reserved {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// blockKeywords open a block (or, for Variable, a one-line definition).
var blockKeywords = map[string]bool{
	"Versions": true, "Story": true, "I18n": true, "Locale": true, "Character": true, "Document": true,
	"Variable": true, "Dialogue": true, "Node": true, "Option": true,
}

// ParseFile parses one .osf file into u and returns every problem found.
// display is the path shown in messages; rel is the path relative to the
// project root, with / separators. A broken statement is skipped and parsing
// resumes at the next field or block at the same depth, so what parsed is
// kept and later checks still run.
func ParseFile(u *Unit, display, rel string, src []byte) []diag.Diagnostic {
	toks, diags := Lex(display, src)
	p := &parser{u: u, toks: toks, rel: rel, folderLocale: localeFolder(rel), diags: diags}
	p.parseFile()
	return p.diags
}

// localeFolder returns X when rel is inside locales/X/, else "".
func localeFolder(rel string) string {
	parts := strings.Split(rel, "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "locales" {
			return parts[i+1]
		}
	}
	return ""
}

type parser struct {
	u            *Unit
	toks         []Token
	i            int
	depth        int // braces opened and not yet closed
	rel          string
	folderLocale string
	diags        []diag.Diagnostic

	// Comments are kept for conversions (osf.Comments): skip collects the
	// comment tokens passed over and boundary hands them to the statements.
	pending []pending
	lastEnd int           // line where the last code token ended
	head    []string      // comment lines above the statement being parsed
	last    *osf.Comments // statement a comment on the current line belongs to
}

type pending struct {
	tok      Token
	trailing bool // on the line where the previous code token ended
}

func isComment(k Kind) bool { return k == LineComment || k == BlockComment }

// skip moves past the comments at the current position, remembering them.
func (p *parser) skip() {
	for isComment(p.toks[p.i].Kind) {
		t := p.toks[p.i]
		p.pending = append(p.pending, pending{t, t.Pos.Line == p.lastEnd})
		p.i++
	}
}

// boundary attributes the comments skipped before t, the first token of a
// statement: those on the line where the previous statement ended trail it,
// the rest head the new one, with "" standing for a blank line.
func (p *parser) boundary(t Token) {
	p.head = nil
	prevEnd := 0
	for _, c := range p.pending {
		text := strings.TrimRight(c.tok.Text, " \t")
		switch {
		case c.trailing:
			if p.last != nil && p.last.Trail == "" && !strings.Contains(text, "\n") {
				p.last.Trail = text
			}
			continue
		case prevEnd > 0 && c.tok.Pos.Line > prevEnd+1:
			p.head = append(p.head, "")
		}
		p.head = append(p.head, strings.Split(text, "\n")...)
		prevEnd = c.tok.End.Line
	}
	if prevEnd > 0 && t.Pos.Line > prevEnd+1 {
		p.head = append(p.head, "")
	}
	p.pending = nil
}

// done records the statement just parsed as the owner of p.head and of a
// comment that may follow on its line.
func (p *parser) done(c *osf.Comments) {
	c.Head = p.head
	p.last = c
}

// bail is the panic value that aborts the current statement; block and
// parseFile recover it and skip to the next statement.
type bail struct{}

func (p *parser) peek() Token { p.skip(); return p.toks[p.i] }

func (p *parser) peek2() Token {
	p.skip()
	for j := p.i + 1; j < len(p.toks); j++ {
		if !isComment(p.toks[j].Kind) {
			return p.toks[j]
		}
	}
	return p.toks[len(p.toks)-1]
}

func (p *parser) next() Token {
	p.skip()
	t := p.toks[p.i]
	switch t.Kind {
	case EOF:
		return t
	case LBrace:
		p.depth++
	case RBrace:
		p.depth = max(p.depth-1, 0)
	}
	p.i++
	p.lastEnd = t.End.Line
	return t
}

func (p *parser) report(r diag.Rule, t Token, format string, args ...any) {
	p.diags = append(p.diags, r.At(t.Pos, t.End, format, args...))
}

// fail reports and abandons the current statement. The offending token must
// already be consumed, so that recovery does not stop on it.
func (p *parser) fail(r diag.Rule, t Token, format string, args ...any) {
	p.report(r, t, format, args...)
	panic(bail{})
}

// found is fail for "expected X, found t": when t opens a block or closes
// one it is given back, so the block after a missing value is still parsed.
func (p *parser) found(r diag.Rule, t Token, format string, args ...any) {
	if p.i > 0 && p.toks[p.i-1].Pos == t.Pos && (t.Kind == RBrace || t.Kind == Ident && blockKeywords[t.Text]) {
		p.i--
		if t.Kind == RBrace {
			p.depth++
		}
	}
	p.fail(r, t, format, args...)
}

// attempt runs stmt; if it fails, tokens are skipped until the next statement
// start at depth base (or the brace closing the block at that depth).
func (p *parser) attempt(base int, stmt func()) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bail); !ok {
				panic(r)
			}
			p.sync(base)
		}
	}()
	stmt()
}

func (p *parser) sync(base int) {
	for {
		t := p.peek()
		switch {
		case t.Kind == EOF:
			return
		case p.depth > base:
			p.next()
		case t.Kind == RBrace:
			if base == 0 {
				p.next() // a stray } at the top level
				continue
			}
			return
		case t.Kind == Semi:
			p.next()
			return
		case t.Kind == Ident && (blockKeywords[t.Text] || base > 0 && p.isField(t)):
			return
		default:
			p.next()
		}
	}
}

// isField reports whether t starts a field statement: `name =`, `name :` or
// `start <id>`.
func (p *parser) isField(t Token) bool {
	if t.Text == "start" {
		return true
	}
	k := p.peek2().Kind
	return k == Equals || k == Colon
}

func describe(t Token) string {
	switch t.Kind {
	case Ident, Int:
		return fmt.Sprintf("%q", t.Text)
	case String, LongString:
		return "a string"
	}
	return t.Kind.String()
}

func (p *parser) expect(k Kind, context string) Token {
	t := p.next()
	if t.Kind != k {
		p.found(diag.UnexpectedToken, t, "expected %s %s, found %s", k, context, describe(t))
	}
	return t
}

func (p *parser) skipSeparators(kinds ...Kind) {
	for {
		k := p.peek().Kind
		found := false
		for _, s := range kinds {
			found = found || k == s
		}
		if !found {
			return
		}
		p.next()
	}
}

// block parses `{ statement* }`, calling stmt with the first token of each
// statement. Statements always start with a name. A statement that fails is
// skipped; a block that is never closed ends at the end of the file. c is
// the block's own comments, which a comment on the `{` or `}` line joins.
func (p *parser) block(owner Token, c *osf.Comments, stmt func(t Token)) {
	p.expect(LBrace, "to open "+owner.Text)
	p.last = c
	base := p.depth
	for {
		p.skipSeparators(Semi)
		t := p.next()
		p.boundary(t)
		switch t.Kind {
		case RBrace:
			p.last = c
			return
		case EOF:
			p.report(diag.UnclosedBlock, owner, "%s block is not closed: missing }", owner.Text)
			p.last = c
			return
		case Ident:
			p.attempt(base, func() { stmt(t) })
		default:
			p.attempt(base, func() {
				p.fail(diag.UnexpectedToken, t, "expected a field or a block inside %s, found %s", owner.Text, describe(t))
			})
		}
	}
}

// fields catches a field written twice in one block.
type fields map[string]diag.Pos

func (f fields) once(p *parser, t Token) {
	if prev, ok := f[t.Text]; ok {
		p.fail(diag.DuplicateField, t, "%s is already set on line %d", t.Text, prev.Line)
	}
	f[t.Text] = t.Pos
}

func (p *parser) unknown(t Token, block, allowed string) {
	if blockKeywords[t.Text] {
		p.fail(diag.MisplacedBlock, t, "%s blocks do not belong inside %s; expected %s", t.Text, block, allowed)
	}
	p.fail(diag.UnknownField, t, "unknown field %q in %s; expected %s", t.Text, block, allowed)
}

func (p *parser) id(what string) osf.Str {
	t := p.next()
	if t.Kind == Ident && reserved[t.Text] {
		p.fail(diag.ReservedWord, t, "%q is a reserved word and cannot be used as %s", t.Text, what)
	}
	if t.Kind != Ident && (t.Kind != Int || t.Text[0] == '-') {
		p.found(diag.ExpectedID, t, "expected %s, found %s", what, describe(t))
	}
	return osf.Str{Value: t.Text, Pos: t.Pos, End: t.End}
}

func (p *parser) str(field Token) *osf.Str {
	t := p.next()
	if t.Kind != String && t.Kind != LongString {
		p.found(diag.ExpectedString, t, "expected a string for %s, found %s", field.Text, describe(t))
	}
	return &osf.Str{Value: t.Value, Pos: t.Pos, End: t.End}
}

// op expects the `=` or `:` that follows a field name.
func (p *parser) op(field Token, want Kind) {
	if t := p.next(); t.Kind != want {
		p.found(diag.WrongSeparator, t, "expected %s after %s, found %s", want, field.Text, describe(t))
	}
}

func (p *parser) boolean(field Token) bool {
	t := p.next()
	if t.Kind != Ident || t.Text != "true" && t.Text != "false" {
		p.found(diag.ExpectedValue, t, "expected true or false for %s, found %s", field.Text, describe(t))
	}
	return t.Text == "true"
}

func (p *parser) value(what string) osf.Value {
	t := p.next()
	switch {
	case t.Kind == Int:
		n, err := strconv.ParseInt(t.Text, 10, 64)
		if err != nil || n < osf.MinNumber || n > osf.MaxNumber {
			p.fail(diag.NumberTooLarge, t, "number %s is too large", t.Text)
		}
		return osf.Value{Kind: osf.Int, Int: n}
	case t.Kind == Ident && (t.Text == "true" || t.Text == "false"):
		return osf.Value{Kind: osf.Bool, Bool: t.Text == "true"}
	}
	p.found(diag.ExpectedValue, t, "expected a value for %s (true, false or a number), found %s", what, describe(t))
	return osf.Value{}
}

// mapOps are the operators each value map takes besides `:`.
var mapOps = map[string]map[string]osf.Op{
	"set":      {"+=": osf.OpAdd, "-=": osf.OpSubtract},
	"requires": {"!=": osf.OpNotEqual, "<": osf.OpLess, "<=": osf.OpLessEqual, ">": osf.OpGreater, ">=": osf.OpGreaterEqual},
}

// valueMap parses `{ name: value, ... }`, where a `set` entry may also be
// `name += n` or `name -= n` and a `requires` entry a comparison such as
// `name >= n`. A bad entry is skipped up to the next comma or the closing brace.
func (p *parser) valueMap(field Token) []osf.Assign {
	p.expect(LBrace, "after "+field.Text+":")
	base := p.depth
	var out []osf.Assign
	seen := fields{}
	for {
		p.skipSeparators(Comma, Semi)
		if p.peek().Kind == RBrace {
			p.next()
			return out
		}
		if p.peek().Kind == EOF {
			p.report(diag.UnclosedBlock, field, "%s map is not closed: missing }", field.Text)
			return out
		}
		p.attemptEntry(base, func() {
			name := p.entryName(field)
			var op osf.Op
			if t := p.peek(); t.Kind == Op {
				p.next()
				known := false
				if op, known = mapOps[field.Text][t.Text]; !known {
					if field.Text == "set" {
						p.fail(diag.UnexpectedToken, t, "set takes name: value, name += number or name -= number, found '%s'", t.Text)
					}
					p.fail(diag.UnexpectedToken, t, "requires takes name: value or a comparison (!=, <, <=, >, >=), found '%s'", t.Text)
				}
			} else {
				p.expect(Colon, "after "+name.Value)
			}
			// set names a variable once; requires may compare one several
			// times, as in gold >= 3, gold <= 10, but not twice the same way.
			if key := name.Value; field.Text != "requires" {
				seen.once(p, Token{Text: key, Pos: name.Pos})
			} else if prev, ok := seen[key+" "+string(op)]; ok {
				if op == osf.OpEqual {
					p.fail(diag.DuplicateField, Token{Text: key, Pos: name.Pos, End: name.End}, "%s is already required on line %d", key, prev.Line)
				}
				p.fail(diag.DuplicateField, Token{Text: key, Pos: name.Pos, End: name.End}, "%s %s is already required on line %d", key, op, prev.Line)
			} else {
				seen[key+" "+string(op)] = name.Pos
			}
			out = append(out, osf.Assign{Name: name.Value, Op: op, Value: p.value(name.Value), Pos: name.Pos, End: name.End})
		})
	}
}

// entryName parses the name of a value map entry: a variable, or in requires
// a played or selected condition, a dotted path such as node_02.played,
// node_02.ask.selected or Dialogue.intro.greet.played. The checker validates
// the path.
func (p *parser) entryName(field Token) osf.Str {
	var name osf.Str
	if t := p.peek(); field.Text == "requires" && t.Kind == Ident && t.Text == "Dialogue" && p.peek2().Kind == Dot {
		p.next()
		name = osf.Str{Value: t.Text, Pos: t.Pos, End: t.End}
	} else {
		name = p.id("a variable name")
	}
	for p.peek().Kind == Dot {
		dot := p.next()
		if field.Text != "requires" {
			p.fail(diag.UnexpectedToken, dot, "%s takes variable names; played and selected conditions belong in requires", field.Text)
		}
		part := p.id("a node or option id, played or selected")
		name.Value += "." + part.Value
		name.End = part.End
	}
	return name
}

// attemptEntry is attempt for value map entries: recovery stops at a comma,
// at the closing brace, or at the next `name :`.
func (p *parser) attemptEntry(base int, entry func()) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bail); !ok {
				panic(r)
			}
			for {
				t := p.peek()
				switch {
				case t.Kind == EOF:
					return
				case p.depth > base:
					p.next()
				case t.Kind == RBrace || t.Kind == Comma:
					return
				case t.Kind == Ident && (p.peek2().Kind == Colon || p.peek2().Kind == Op):
					return
				default:
					p.next()
				}
			}
		}
	}()
	entry()
}

// ref parses `Kind.id`.
func (p *parser) ref(kind string) *osf.Str {
	t := p.next()
	if t.Kind != Ident || t.Text != kind {
		p.found(diag.BadReference, t, "expected %s.<id>, found %s", kind, describe(t))
	}
	p.expect(Dot, "after "+kind)
	id := p.id("a " + strings.ToLower(kind) + " id")
	return &osf.Str{Value: kind + "." + id.Value, Pos: t.Pos, End: id.End}
}

// notInLocale rejects fields that cannot be translated.
func (p *parser) notInLocale(t Token, inLocale bool) {
	if inLocale {
		p.fail(diag.NotTranslatable, t, "%s cannot be translated; a Locale block holds only name, description, body, title and text", t.Text)
	}
}

func (p *parser) parseFile() {
	for {
		p.skipSeparators(Semi)
		t := p.next()
		p.boundary(t)
		if t.Kind == EOF {
			return
		}
		p.attempt(0, func() { p.topLevel(t) })
	}
}

func (p *parser) topLevel(t Token) {
	if t.Kind != Ident {
		p.fail(diag.UnexpectedToken, t, "expected a block such as Dialogue or Character, found %s", describe(t))
	}
	proj := p.u.Project
	switch t.Text {
	case "Versions":
		p.versions(t)
	case "Story":
		proj.Stories = append(proj.Stories, p.story(t, false))
	case "I18n":
		proj.I18n = append(proj.I18n, p.i18n(t))
	case "Character":
		proj.Characters = append(proj.Characters, p.character(t, false))
	case "Document":
		proj.Documents = append(proj.Documents, p.document(t))
	case "Variable":
		proj.Variables = append(proj.Variables, p.variable(t))
	case "Dialogue":
		proj.Dialogues = append(proj.Dialogues, p.dialogue(t, false))
	case "Locale":
		proj.Locales = append(proj.Locales, p.locale(t))
	case "Node", "Option":
		p.fail(diag.MisplacedBlock, t, "%s blocks belong inside a Dialogue", t.Text)
	default:
		p.fail(diag.UnknownBlock, t, "unknown block %q; expected Versions, Story, I18n, Character, Document, Variable, Dialogue or Locale", t.Text)
	}
}

func (p *parser) versions(kw Token) {
	v := &Versions{Pos: kw.Pos, Comments: osf.Comments{Head: p.head}}
	seen := fields{}
	p.block(kw, &v.Comments, func(t Token) {
		seen.once(p, t)
		p.op(t, Equals)
		v.Entries = append(v.Entries, VersionEntry{Name: t.Text, Value: *p.str(t)})
		p.done(&v.Entries[len(v.Entries)-1].Value.Comments)
	})
	p.u.Versions = append(p.u.Versions, v)
}

func (p *parser) story(kw Token, inLocale bool) *osf.Story {
	id := p.id("a story id")
	s := &osf.Story{ID: id.Value, Pos: kw.Pos, End: id.End, IDPos: id.Pos, File: p.rel, Comments: osf.Comments{Head: p.head}}
	seen := fields{}
	p.block(kw, &s.Comments, func(t Token) {
		switch t.Text {
		case "name", "description":
			seen.once(p, t)
			p.op(t, Equals)
			if t.Text == "name" {
				s.Name = p.str(t)
				p.done(&s.Name.Comments)
			} else {
				s.Description = p.str(t)
				p.done(&s.Description.Comments)
			}
		case "start":
			p.notInLocale(t, inLocale)
			seen.once(p, t)
			p.op(t, Equals)
			s.Start = p.ref("Dialogue")
			p.done(&s.Start.Comments)
		default:
			p.unknown(t, "Story", "name, description or start")
		}
	})
	return s
}

func (p *parser) i18n(kw Token) *osf.I18n {
	c := &osf.I18n{Pos: kw.Pos, End: kw.End, File: p.rel, Comments: osf.Comments{Head: p.head}}
	seen := fields{}
	p.block(kw, &c.Comments, func(t Token) {
		switch t.Text {
		case "source":
			seen.once(p, t)
			p.op(t, Equals)
			id := p.id("a locale id")
			c.Source = &id
			p.done(&c.Source.Comments)
		case "Locale":
			id := p.id("a locale id")
			def := osf.LocaleDef{ID: id.Value, Pos: id.Pos, End: id.End, Comments: osf.Comments{Head: p.head}}
			def.Name = p.str(t).Value
			if p.peek().Kind == LBrace {
				opts := fields{}
				p.block(t, &def.Comments, func(o Token) {
					if o.Text != "enforce" {
						p.unknown(o, "Locale", "enforce")
					}
					opts.once(p, o)
					p.op(o, Equals)
					def.Enforce = p.boolean(o)
				})
			}
			c.Locales = append(c.Locales, def)
			p.last = &c.Locales[len(c.Locales)-1].Comments
		default:
			p.unknown(t, "I18n", "source or Locale")
		}
	})
	return c
}

func (p *parser) character(kw Token, inLocale bool) *osf.Character {
	id := p.id("a character id")
	c := &osf.Character{ID: id.Value, Pos: kw.Pos, End: id.End, IDPos: id.Pos, File: p.rel, Comments: osf.Comments{Head: p.head}}
	seen := fields{}
	p.block(kw, &c.Comments, func(t Token) {
		switch t.Text {
		case "name":
			seen.once(p, t)
			p.op(t, Equals)
			c.Name = p.str(t)
			p.done(&c.Name.Comments)
		case "description":
			seen.once(p, t)
			p.op(t, Equals)
			c.Description = p.str(t)
			p.done(&c.Description.Comments)
		default:
			p.unknown(t, "Character", "name or description")
		}
	})
	return c
}

func (p *parser) document(kw Token) *osf.Document {
	id := p.id("a document id")
	d := &osf.Document{ID: id.Value, Pos: kw.Pos, End: id.End, IDPos: id.Pos, File: p.rel, Comments: osf.Comments{Head: p.head}}
	seen := fields{}
	p.block(kw, &d.Comments, func(t Token) {
		switch t.Text {
		case "name":
			seen.once(p, t)
			p.op(t, Equals)
			d.Name = p.str(t)
			p.done(&d.Name.Comments)
		default:
			p.unknown(t, "Document", "name")
		}
	})
	return d
}

func (p *parser) variable(kw Token) *osf.Variable {
	id := p.id("a variable id")
	v := &osf.Variable{ID: id.Value, Pos: kw.Pos, End: id.End, IDPos: id.Pos, File: p.rel}
	p.done(&v.Comments)
	p.expect(Colon, "after the variable id")
	t := p.expect(Ident, "as the variable type")
	switch t.Text {
	case "Boolean":
		v.Type = osf.Boolean
	case "Number":
		v.Type = osf.Number
	case "UnsignedNumber":
		v.Type = osf.UnsignedNumber
	default:
		p.fail(diag.UnknownVariableType, t, "unknown variable type %q; expected Boolean, Number or UnsignedNumber", t.Text)
	}
	p.expect(Equals, "after the variable type")
	v.Default = p.value(v.ID)
	if p.peek().Kind == LBrace {
		seen := fields{}
		p.block(kw, &v.Comments, func(t Token) {
			if t.Text != "description" {
				p.unknown(t, "Variable", "description")
			}
			seen.once(p, t)
			p.op(t, Equals)
			v.Description = p.str(t)
			p.done(&v.Description.Comments)
		})
		p.last = &v.Comments
	}
	return v
}

func (p *parser) dialogue(kw Token, inLocale bool) *osf.Dialogue {
	id := p.id("a dialogue id")
	d := &osf.Dialogue{ID: id.Value, Pos: kw.Pos, End: id.End, IDPos: id.Pos, File: p.rel, Comments: osf.Comments{Head: p.head}}
	seen := fields{}
	nodes := fields{}
	p.block(kw, &d.Comments, func(t Token) {
		switch t.Text {
		case "name":
			seen.once(p, t)
			p.op(t, Equals)
			d.Name = p.str(t)
			p.done(&d.Name.Comments)
		case "start":
			p.notInLocale(t, inLocale)
			seen.once(p, t)
			if k := p.peek().Kind; k == Equals || k == Colon {
				p.fail(diag.WrongSeparator, p.next(), "write start followed by the node id, with no %s", k)
			}
			id := p.id("the start node id")
			d.StartNode = &id
			p.done(&d.StartNode.Comments)
		case "Node":
			n := p.node(t, inLocale)
			if prev, ok := nodes[n.ID]; ok {
				p.diags = append(p.diags, diag.DuplicateNode.At(n.Pos, n.End, "Node %s is already defined on line %d", n.ID, prev.Line))
				return
			}
			nodes[n.ID] = n.Pos
			d.Nodes = append(d.Nodes, n)
		default:
			p.unknown(t, "Dialogue", "name, start or Node")
		}
	})
	return d
}

// target parses the value of `next`: a node id, <node>.options or
// Dialogue.<id>.
func (p *parser) target() *osf.Str {
	if t := p.peek(); t.Kind == Ident && t.Text == "Dialogue" {
		return p.ref("Dialogue")
	}
	id := p.id("a node id or Dialogue.<id>")
	if p.peek().Kind == Dot {
		p.next()
		t := p.next()
		if t.Kind != Ident || t.Text != "options" {
			p.found(diag.BadReference, t, "expected options after %s., to go back to its options; found %s", id.Value, describe(t))
		}
		id.Value += ".options"
		id.End = t.End
	}
	return &id
}

// nextField parses `next: target`, which may be written once, or
// `next: target requires { ... }`, a branch, which may be written any number
// of times and is tried in the order written.
func (p *parser) nextField(t Token, seen fields, next **osf.Str, branches *[]*osf.Branch) {
	p.op(t, Colon)
	to := p.target()
	if r := p.peek(); r.Kind == Ident && r.Text == "requires" {
		p.next()
		b := &osf.Branch{To: to, Requires: p.valueMap(r)}
		*branches = append(*branches, b)
		p.done(&b.Comments)
		return
	}
	seen.once(p, t)
	*next = to
	p.done(&to.Comments)
}

func (p *parser) node(kw Token, inLocale bool) *osf.Node {
	id := p.id("a node id")
	n := &osf.Node{ID: id.Value, Pos: kw.Pos, End: id.End, IDPos: id.Pos, Comments: osf.Comments{Head: p.head}}
	seen := fields{}
	options := fields{}
	p.block(kw, &n.Comments, func(t Token) {
		switch t.Text {
		case "text", "title":
			seen.once(p, t)
			p.op(t, Colon)
			if t.Text == "text" {
				n.Text = p.str(t)
				p.done(&n.Text.Comments)
			} else {
				n.Title = p.str(t)
				p.done(&n.Title.Comments)
			}
		case "speaker":
			p.notInLocale(t, inLocale)
			seen.once(p, t)
			p.op(t, Colon)
			if pk := p.peek(); pk.Kind == Ident && pk.Text == "Character" && p.peek2().Kind == Dot {
				p.fail(diag.BadReference, p.next(), "write the document id alone: speaker: <id>")
			}
			id := p.id("a document id")
			n.Speaker = &osf.Str{Value: "Character." + id.Value, Pos: id.Pos, End: id.End}
			p.done(&n.Speaker.Comments)
		case "next":
			p.notInLocale(t, inLocale)
			p.nextField(t, seen, &n.Next, &n.Branches)
		case "set":
			p.notInLocale(t, inLocale)
			seen.once(p, t)
			p.op(t, Colon)
			n.Set = p.valueMap(t)
			p.done(&n.SetComments)
		case "requires":
			p.fail(diag.MisplacedBlock, t, "requires is only allowed on an Option")
		case "Option":
			o := p.option(t, inLocale)
			if prev, ok := options[o.ID]; ok {
				p.diags = append(p.diags, diag.DuplicateOption.At(o.Pos, o.End, "Option %s is already defined on line %d", o.ID, prev.Line))
				return
			}
			options[o.ID] = o.Pos
			n.Options = append(n.Options, o)
			if inLocale && o.Text != nil {
				// A translated option is one statement: its comments are the text's.
				o.Text.Comments.Head = append(o.Comments.Head, o.Text.Comments.Head...)
				p.last = &o.Text.Comments
			}
		default:
			p.unknown(t, "Node", "text, title, speaker, next, set or Option")
		}
	})
	return n
}

func (p *parser) option(kw Token, inLocale bool) *osf.Option {
	id := p.id("an option id")
	o := &osf.Option{ID: id.Value, Pos: kw.Pos, End: id.End, IDPos: id.Pos, Comments: osf.Comments{Head: p.head}}
	seen := fields{}
	p.block(kw, &o.Comments, func(t Token) {
		switch t.Text {
		case "text":
			seen.once(p, t)
			p.op(t, Colon)
			o.Text = p.str(t)
			p.done(&o.Text.Comments)
		case "next":
			p.notInLocale(t, inLocale)
			p.nextField(t, seen, &o.Next, &o.Branches)
		case "requires", "set":
			p.notInLocale(t, inLocale)
			seen.once(p, t)
			p.op(t, Colon)
			if t.Text == "set" {
				o.Set = p.valueMap(t)
				p.done(&o.SetComments)
			} else {
				o.Requires = p.valueMap(t)
				p.done(&o.RequiresComments)
			}
		default:
			p.unknown(t, "Option", "text, next, requires or set")
		}
	})
	return o
}

func (p *parser) locale(kw Token) *osf.Locale {
	lang := p.id("a locale id")
	if p.folderLocale != "" && lang.Value != p.folderLocale {
		p.diags = append(p.diags, diag.LocaleFolderMismatch.At(lang.Pos, lang.End,
			"Locale %s is in the locales/%s folder; use Locale %s or move the file to locales/%s/", lang.Value, p.folderLocale, p.folderLocale, lang.Value))
	}
	loc := &osf.Locale{Language: lang, Pos: kw.Pos, End: lang.End, File: p.rel, Comments: osf.Comments{Head: p.head}}
	add := func(key string, s *osf.Str, ids ...osf.Str) {
		if s != nil {
			loc.Values = append(loc.Values, osf.LocaleValue{Key: key, Text: *s, IDs: ids})
		}
	}
	p.block(kw, &loc.Comments, func(t Token) {
		switch t.Text {
		case "Story":
			s := p.story(t, true)
			id := osf.IDSpan(s.ID, s.IDPos)
			add("Story."+s.ID+".name", s.Name, id)
			add("Story."+s.ID+".description", s.Description, id)
		case "Character":
			c := p.character(t, true)
			id := osf.IDSpan(c.ID, c.IDPos)
			add("Character."+c.ID+".name", c.Name, id)
			add("Character."+c.ID+".description", c.Description, id)
		case "Document":
			d := p.document(t)
			add("Document."+d.ID+".name", d.Name, osf.IDSpan(d.ID, d.IDPos))
		case "Dialogue":
			d := p.dialogue(t, true)
			did := osf.IDSpan(d.ID, d.IDPos)
			add("Dialogue."+d.ID+".name", d.Name, did)
			for _, n := range d.Nodes {
				nk, nid := "Dialogue."+d.ID+".nodes."+n.ID, osf.IDSpan(n.ID, n.IDPos)
				add(nk+".content.title", n.Title, did, nid)
				add(nk+".content.text", n.Text, did, nid)
				for _, o := range n.Options {
					add(nk+".options."+o.ID+".content.text", o.Text, did, nid, osf.IDSpan(o.ID, o.IDPos))
				}
			}
		default:
			p.fail(diag.NotTranslatable, t, "only Story, Character, Document and Dialogue blocks can be translated, found %s", describe(t))
		}
	})
	return loc
}
