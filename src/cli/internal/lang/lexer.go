// Package lang reads the .osf language (language.md) into the osf model.
package lang

import (
	"strings"
	"unicode/utf8"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/diag"
)

type Kind int

const (
	EOF Kind = iota
	Ident
	Int
	String     // "..."
	LongString // """..."""
	LBrace
	RBrace
	Colon
	Equals
	Dot
	Comma
	Semi
	Op           // += -= != < <= > >=
	LineComment  // // ...
	BlockComment // /* ... */
)

var kindNames = [...]string{"end of file", "name", "number", "string", "long text", "'{'", "'}'", "':'", "'='", "'.'", "','", "';'", "operator", "comment", "comment"}

func (k Kind) String() string { return kindNames[k] }

type Token struct {
	Kind  Kind
	Text  string // exactly as written
	Value string // decoded value of strings; same as Text otherwise
	Pos   diag.Pos
	End   diag.Pos // just after the token's last character
}

// Lex splits src into tokens, comments included. Line endings are normalised
// to \n first. Every problem is reported and lexing goes on: a bad character
// is skipped, an unclosed string ends at its line, an unclosed long text or
// comment ends at the end of the file.
func Lex(file string, src []byte) ([]Token, []diag.Diagnostic) {
	s := strings.ReplaceAll(string(src), "\r\n", "\n")
	s = strings.TrimPrefix(s, "\uFEFF")
	lx := &lexer{file: file, src: s, line: 1, col: 1}
	if !utf8.ValidString(s) {
		lx.report(diag.InvalidUTF8, lx.pos(), lx.pos(), "file is not valid UTF-8")
		lx.src = strings.ToValidUTF8(s, "\uFFFD")
	}
	var toks []Token
	for {
		t := lx.next()
		toks = append(toks, t)
		if t.Kind == EOF {
			return toks, lx.diags
		}
	}
}

type lexer struct {
	file      string
	src       string
	i         int
	line, col int
	diags     []diag.Diagnostic
}

func (lx *lexer) pos() diag.Pos { return diag.Pos{File: lx.file, Line: lx.line, Col: lx.col} }

func (lx *lexer) report(r diag.Rule, pos, end diag.Pos, format string, args ...any) {
	lx.diags = append(lx.diags, r.At(pos, end, format, args...))
}

// advance moves past n bytes, keeping line and column up to date.
func (lx *lexer) advance(n int) {
	for _, r := range lx.src[lx.i : lx.i+n] {
		if r == '\n' {
			lx.line++
			lx.col = 1
		} else {
			lx.col++
		}
	}
	lx.i += n
}

func isWordByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func (lx *lexer) next() Token {
	for {
		for lx.i < len(lx.src) && strings.IndexByte(" \t\n\r", lx.src[lx.i]) >= 0 {
			lx.advance(1)
		}
		start := lx.pos()
		rest := lx.src[lx.i:]
		tok := func(k Kind, n int) Token {
			text := rest[:n]
			lx.advance(n)
			return Token{Kind: k, Text: text, Value: text, Pos: start, End: lx.pos()}
		}
		if rest == "" {
			return Token{Kind: EOF, Pos: start, End: start}
		}
		switch c := rest[0]; {
		case strings.HasPrefix(rest, "//"):
			n := strings.IndexByte(rest, '\n')
			if n < 0 {
				n = len(rest)
			}
			return tok(LineComment, n)
		case strings.HasPrefix(rest, "/*"):
			n := strings.Index(rest[2:], "*/")
			if n < 0 {
				lx.report(diag.UnclosedComment, start, lx.after(2), "comment is not closed: missing */")
				return tok(BlockComment, len(rest))
			}
			return tok(BlockComment, n+4)
		case strings.HasPrefix(rest, `"""`):
			n := strings.Index(rest[3:], `"""`)
			if n < 0 {
				lx.report(diag.UnclosedLongText, start, lx.after(3), `long text is not closed: missing """`)
				t := tok(LongString, len(rest))
				t.Value = LongTextValue(t.Text[3:])
				return t
			}
			t := tok(LongString, n+6)
			t.Value = LongTextValue(t.Text[3 : len(t.Text)-3])
			return t
		case c == '"':
			return lx.str(start, rest)
		case c == '{':
			return tok(LBrace, 1)
		case c == '}':
			return tok(RBrace, 1)
		case c == ':':
			return tok(Colon, 1)
		case c == '=':
			return tok(Equals, 1)
		case c == '.':
			return tok(Dot, 1)
		case c == ',':
			return tok(Comma, 1)
		case c == ';':
			return tok(Semi, 1)
		case opLen(rest) > 0:
			return tok(Op, opLen(rest))
		case isWordByte(c) || c == '-' && len(rest) > 1 && rest[1] >= '0' && rest[1] <= '9':
			n := 1
			for n < len(rest) {
				if isWordByte(rest[n]) {
					n++
				} else if rest[n] == '-' && rest[n-1] != '-' && n+1 < len(rest) && isWordByte(rest[n+1]) {
					n++
				} else {
					break
				}
			}
			word := rest[:n]
			if isInt(word) {
				return tok(Int, n)
			}
			if word[0] == '-' {
				lx.report(diag.BadNumber, start, lx.after(n), "%q is neither a number nor a name", word)
				lx.advance(n)
				continue
			}
			return tok(Ident, n)
		default:
			r, size := utf8.DecodeRuneInString(rest)
			lx.report(diag.UnexpectedCharacter, start, lx.after(size), "unexpected character %q", r)
			lx.advance(size)
		}
	}
}

// after returns the position n bytes past the current one, without moving.
func (lx *lexer) after(n int) diag.Pos {
	p := lx.pos()
	for _, r := range lx.src[lx.i : lx.i+n] {
		if r == '\n' {
			p.Line++
			p.Col = 1
		} else {
			p.Col++
		}
	}
	return p
}

// str lexes a "..." string starting at rest. An unclosed string ends at the
// end of its line; a bad escape is kept as written.
func (lx *lexer) str(start diag.Pos, rest string) Token {
	var b strings.Builder
	end := func(n int) Token {
		text := rest[:n]
		lx.advance(n)
		return Token{Kind: String, Text: text, Value: b.String(), Pos: start, End: lx.pos()}
	}
	for j := 1; j < len(rest); j++ {
		switch rest[j] {
		case '\n':
			lx.report(diag.UnclosedString, start, lx.after(j), `string is not closed on its line; use """ for text over several lines`)
			return end(j)
		case '"':
			return end(j + 1)
		case '\\':
			j++
			if j == len(rest) {
				b.WriteByte('\\')
				break
			}
			switch rest[j] {
			case '"', '\\':
				b.WriteByte(rest[j])
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '{':
				b.WriteString(`\{`) // \{{ is kept for OSF: literal braces in text
			case '\n':
				lx.report(diag.UnclosedString, start, lx.after(j), `string is not closed on its line; use """ for text over several lines`)
				return end(j)
			default:
				r, size := utf8.DecodeRuneInString(rest[j:])
				lx.report(diag.BadEscape, lx.after(j-1), lx.after(j+size), `unknown escape \%c; use \" \\ \n \t or \{{`, r)
				b.WriteByte('\\')
				b.WriteString(rest[j : j+size])
				j += size - 1
			}
		default:
			b.WriteByte(rest[j])
		}
	}
	lx.report(diag.UnclosedString, start, lx.after(len(rest)), "string is not closed: missing \"")
	return end(len(rest))
}

// opLen is the length of the operator rest starts with, or 0.
func opLen(rest string) int {
	for _, op := range [...]string{"+=", "-=", "!=", "<=", ">=", "<", ">"} {
		if strings.HasPrefix(rest, op) {
			return len(op)
		}
	}
	return 0
}

func isInt(s string) bool {
	s = strings.TrimPrefix(s, "-")
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// LongTextLines returns the lines of a """ block body (raw is the text between
// the quotes): the line break after the opening quotes and the whitespace-only
// line before the closing quotes are dropped. ok is false for a block written
// on one line.
func LongTextLines(raw string) (lines []string, ok bool) {
	nl := strings.IndexByte(raw, '\n')
	if nl < 0 {
		return nil, false
	}
	if strings.TrimSpace(raw[:nl]) == "" {
		raw = raw[nl+1:]
	}
	lines = strings.Split(raw, "\n")
	if strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, true
}

// Dedent removes the leading whitespace common to every non-blank line.
func Dedent(lines []string) []string {
	prefix, first := "", true
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		ws := l[:len(l)-len(strings.TrimLeft(l, " \t"))]
		if first {
			prefix, first = ws, false
			continue
		}
		for !strings.HasPrefix(ws, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			out[i] = ""
		} else {
			out[i] = l[len(prefix):]
		}
	}
	return out
}

func isNote(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, " \t"), "//")
}

// LongTextValue is the text a """ block stands for (language.md §2): writer's
// notes removed, common indentation removed, ending with a newline like a YAML
// | block.
func LongTextValue(raw string) string {
	lines, ok := LongTextLines(raw)
	if !ok {
		return raw
	}
	var kept []string
	for _, l := range lines {
		if !isNote(l) {
			kept = append(kept, strings.TrimRight(l, " \t"))
		}
	}
	kept = Dedent(kept)
	for i, l := range kept {
		if t := strings.TrimLeft(l, " \t"); strings.HasPrefix(t, `\//`) {
			kept[i] = l[:len(l)-len(t)] + t[1:]
		}
	}
	return strings.Join(kept, "\n") + "\n"
}
