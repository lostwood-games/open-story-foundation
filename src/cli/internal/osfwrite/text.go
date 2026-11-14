package osfwrite

import (
	"strings"
	"unicode/utf8"
)

const indent = "  "

// quote writes s as a "..." string with the escapes the lexer knows. A \{ in
// the stored text is the literal-brace escape and is written back as \{.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			if i+1 < len(s) && s[i+1] == '{' {
				b.WriteString(`\{`)
				i++
			} else {
				b.WriteString(`\\`)
			}
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// canLong reports whether s survives a """ block unchanged: it ends with a
// line break, no line has trailing whitespace (the lexer trims it), the lines
// share no leading whitespace (the lexer removes it), and nothing in it would
// be read as the closing quotes or as the \// escape.
func canLong(s string) bool {
	if !strings.HasSuffix(s, "\n") {
		return false
	}
	flush := false
	for _, l := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		if strings.TrimRight(l, " \t") != l || strings.Contains(l, `"""`) || strings.Contains(l, "\r") {
			return false
		}
		t := strings.TrimLeft(l, " \t")
		if strings.HasPrefix(t, `\//`) {
			return false
		}
		if t != "" && t == l {
			flush = true
		}
	}
	return flush || strings.TrimSpace(s) == ""
}

// long writes s as a """ block whose body is indented one level deeper than
// depth, the way osf fmt lays it out. Lines starting with // are escaped so
// they are not dropped as writer's notes.
func long(s string, depth int) string {
	var b strings.Builder
	b.WriteString(`"""` + "\n")
	for _, l := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		if l != "" {
			if t := strings.TrimLeft(l, " \t"); strings.HasPrefix(t, "//") {
				l = l[:len(l)-len(t)] + `\` + t
			}
			b.WriteString(strings.Repeat(indent, depth+1) + l)
		}
		b.WriteByte('\n')
	}
	b.WriteString(strings.Repeat(indent, depth) + `"""`)
	return b.String()
}

// text writes a text value for a field at depth: a """ block when the text
// ends with a line break and can be written that way, else a "..." string.
func text(s string, depth int) string {
	if canLong(s) {
		return long(s, depth)
	}
	return quote(s)
}

func width(s string) int { return utf8.RuneCountInString(s) }
