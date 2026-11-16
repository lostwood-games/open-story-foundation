// Package format rewrites .osf source in the canonical style, the way
// `terraform fmt` does for .tf files. Line breaks the writer chose are kept;
// indentation, spacing, blank lines and alignment are normalised:
//
//   - two spaces per block level;
//   - one space between tokens, none before : , ; or around .;
//   - at most one blank line in a row, none at the start of the file;
//   - `=` aligned in runs of consecutive `name = value` lines;
//   - trailing comments aligned in runs of consecutive lines;
//   - the maps of a requires written under it, `{ ... }` then `or { ... }`
//     lines, one level deeper than the requires;
//   - """ text re-indented one level deeper than the line that opens it.
package format

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/diag"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/lang"
)

const indent = "  "

type line struct {
	toks        []lang.Token
	depth       int
	blankBefore bool

	code      string // rendered tokens, without the trailing comment
	comment   string // trailing // comment, if any
	multiline bool   // code spans several lines (""" text or a /* */ comment)
}

// Source formats one .osf file. It returns the first diagnostic as the error
// if the file does not parse.
func Source(file string, src []byte) ([]byte, error) {
	if diags := lang.ParseFile(lang.NewUnit(), file, "", src); len(diags) > 0 {
		diag.Sort(diags)
		return nil, diags[0]
	}
	toks, _ := lang.Lex(file, src)
	lines := split(toks)
	for _, l := range lines {
		render(l)
	}
	alignEquals(lines)
	alignComments(lines)

	var b bytes.Buffer
	for i, l := range lines {
		if l.blankBefore && i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat(indent, l.depth))
		b.WriteString(l.code)
		if l.comment != "" {
			if l.code != "" {
				b.WriteByte(' ')
			}
			b.WriteString(l.comment)
		}
		b.WriteByte('\n')
	}
	out := b.Bytes()
	if err := sameTokens(file, toks, out); err != nil {
		return nil, err
	}
	return out, nil
}

// split groups tokens into the lines they were written on and works out each
// line's block depth.
func split(toks []lang.Token) []*line {
	var lines []*line
	depth, prevEnd := 0, 0
	for _, t := range toks {
		if t.Kind == lang.EOF {
			break
		}
		if len(lines) == 0 || t.Pos.Line > prevEnd {
			d := depth
			if t.Kind == lang.RBrace {
				d--
			}
			nl := &line{depth: max(d, 0), blankBefore: prevEnd > 0 && t.Pos.Line > prevEnd+1}
			if t.Kind == lang.LBrace && len(lines) > 0 && endsWithRequires(lines[len(lines)-1]) {
				// The first map of a requires written under `requires:`.
				nl.depth++
				nl.blankBefore = false
			}
			lines = append(lines, nl)
		}
		l := lines[len(lines)-1]
		if len(l.toks) == 1 && l.toks[0].Kind == lang.Ident && l.toks[0].Text == "or" && t.Kind == lang.LBrace {
			// `or { ... }` on its own line is another map of the requires above.
			l.depth++
			l.blankBefore = false
		}
		l.toks = append(l.toks, t)
		switch t.Kind {
		case lang.LBrace:
			depth++
		case lang.RBrace:
			depth--
		}
		prevEnd = t.End.Line
	}
	return lines
}

// endsWithRequires reports whether a line stops after `requires:` or, on a
// conditional next, after `requires`, so that its maps follow on the next lines.
func endsWithRequires(l *line) bool {
	toks := l.toks
	if n := len(toks); n > 0 && toks[n-1].Kind == lang.LineComment {
		toks = toks[:n-1]
	}
	if n := len(toks); n > 0 && toks[n-1].Kind == lang.Colon {
		toks = toks[:n-1]
	}
	n := len(toks)
	return n > 0 && toks[n-1].Kind == lang.Ident && toks[n-1].Text == "requires"
}

func render(l *line) {
	toks := l.toks
	if n := len(toks); n > 1 && toks[n-1].Kind == lang.LineComment {
		l.comment = strings.TrimRight(toks[n-1].Text, " \t")
		toks = toks[:n-1]
	}
	var b strings.Builder
	for i, t := range toks {
		if i > 0 && space(toks[i-1], t) {
			b.WriteByte(' ')
		}
		switch t.Kind {
		case lang.LongString:
			s := longText(t.Text, l.depth)
			l.multiline = l.multiline || strings.Contains(s, "\n")
			b.WriteString(s)
		case lang.LineComment:
			b.WriteString(strings.TrimRight(t.Text, " \t"))
		case lang.BlockComment:
			l.multiline = l.multiline || strings.Contains(t.Text, "\n")
			b.WriteString(t.Text)
		default:
			b.WriteString(t.Text)
		}
	}
	l.code = b.String()
}

func space(a, b lang.Token) bool {
	switch {
	case b.Kind == lang.Colon || b.Kind == lang.Comma || b.Kind == lang.Semi || b.Kind == lang.Dot:
		return false
	case a.Kind == lang.Dot:
		return false
	case a.Kind == lang.LBrace && b.Kind == lang.RBrace:
		return false
	}
	return true
}

// longText re-indents a """ block: body one level deeper than the line,
// closing quotes at the line's level.
func longText(text string, depth int) string {
	raw := text[3 : len(text)-3]
	lines, ok := lang.LongTextLines(raw)
	if !ok {
		return text
	}
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	var b strings.Builder
	b.WriteString(`"""` + "\n")
	body := strings.Repeat(indent, depth+1)
	for _, l := range lang.Dedent(lines) {
		if l != "" {
			b.WriteString(body + l)
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat(indent, depth) + `"""`)
	return b.String()
}

func width(s string) int { return utf8.RuneCountInString(s) }

// alignEquals pads `name = value` lines so their = line up within each run of
// consecutive such lines at the same depth.
func alignEquals(lines []*line) {
	isAssign := func(l *line) bool {
		return len(l.toks) >= 2 && l.toks[0].Kind == lang.Ident && l.toks[1].Kind == lang.Equals && !l.multiline
	}
	runs(lines, isAssign, func(run []*line) {
		w := 0
		for _, l := range run {
			w = max(w, width(l.toks[0].Text))
		}
		for _, l := range run {
			name := l.toks[0].Text
			l.code = name + strings.Repeat(" ", w-width(name)) + l.code[len(name):]
		}
	})
}

// alignComments lines up trailing comments within each run of consecutive
// lines that have one, two spaces after the longest code.
func alignComments(lines []*line) {
	hasTrailing := func(l *line) bool { return l.comment != "" && l.code != "" && !l.multiline }
	runs(lines, hasTrailing, func(run []*line) {
		w := 0
		for _, l := range run {
			w = max(w, width(l.code)+len(indent)*l.depth)
		}
		for _, l := range run {
			l.code += strings.Repeat(" ", w-width(l.code)-len(indent)*l.depth+1)
		}
	})
}

// runs calls fn with each maximal run of consecutive lines that match ok, are
// not separated by a blank line and share a depth.
func runs(lines []*line, ok func(*line) bool, fn func([]*line)) {
	for i := 0; i < len(lines); {
		if !ok(lines[i]) {
			i++
			continue
		}
		j := i + 1
		for j < len(lines) && ok(lines[j]) && !lines[j].blankBefore && lines[j].depth == lines[i].depth {
			j++
		}
		fn(lines[i:j])
		i = j
	}
}

// sameTokens guards against the formatter changing meaning: the formatted
// file must lex to the same tokens with the same values.
func sameTokens(file string, before []lang.Token, out []byte) error {
	after, diags := lang.Lex(file, out)
	if len(diags) > 0 {
		return fmt.Errorf("%s: formatting broke the file (please report this): %v", file, diags[0])
	}
	if len(after) != len(before) {
		return fmt.Errorf("%s: formatting changed the file's tokens (please report this)", file)
	}
	for i := range before {
		a, b := before[i], after[i]
		same := a.Kind == b.Kind && a.Value == b.Value
		if a.Kind == lang.LineComment {
			same = a.Kind == b.Kind && strings.TrimRight(a.Text, " \t") == b.Text
		}
		if !same {
			return fmt.Errorf("%s:%d: formatting changed %s (please report this)", file, a.Pos.Line, a.Kind)
		}
	}
	return nil
}
