package osfwrite

import (
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// writer collects the lines of one block, indenting by depth.
type writer struct {
	depth int
	lines []string
}

// add writes one statement. A value spanning several lines (""" text) is
// already indented from its second line on.
func (w *writer) add(s string) {
	first, rest, multi := strings.Cut(s, "\n")
	w.lines = append(w.lines, strings.Repeat(indent, w.depth)+first)
	if multi {
		w.lines = append(w.lines, strings.Split(rest, "\n")...)
	}
}

// addc writes a statement with its comments: the head lines above it, the
// trailing comment after it (osf fmt aligns those).
func (w *writer) addc(s string, c osf.Comments) {
	w.comments(c.Head)
	if c.Trail != "" {
		s += "  " + c.Trail
	}
	w.add(s)
}

// comments writes head comment lines (osf.Comments): comment lines at the
// current depth, the inner lines of a /* */ comment as they were, "" as a
// blank line.
func (w *writer) comments(lines []string) {
	inBlock := false
	for _, l := range lines {
		switch {
		case inBlock:
			w.lines = append(w.lines, l)
			inBlock = !strings.HasSuffix(l, "*/")
		case l == "":
			w.blank()
		default:
			w.add(l)
			inBlock = strings.HasPrefix(l, "/*") && !strings.HasSuffix(l, "*/")
		}
	}
}

// blank separates the next statement from the previous one; nothing is added
// right after an opening brace (language.md §11).
func (w *writer) blank() {
	if n := len(w.lines); n > 0 && w.lines[n-1] != "" && !strings.HasSuffix(w.lines[n-1], "{") {
		w.lines = append(w.lines, "")
	}
}

// block writes `header {`, the body at one level deeper, and `}`, with the
// block's comments above and after the header. An empty body gives
// `header {}`.
func (w *writer) block(header string, c osf.Comments, body func()) {
	w.comments(c.Head)
	start := len(w.lines)
	trail := ""
	if c.Trail != "" {
		trail = "  " + c.Trail
	}
	w.add(header + " {" + trail)
	w.depth++
	body()
	w.depth--
	if len(w.lines) == start+1 {
		w.lines[start] = strings.Repeat(indent, w.depth) + header + " {}" + trail
		return
	}
	w.add("}")
}

type assign struct {
	name     string
	value    string // rendered
	comments osf.Comments
}

// assigns writes `name = value` lines with = aligned within each run of
// single-line values, as osf fmt aligns them: a head comment starts a new run.
func (w *writer) assigns(as []assign) {
	for i := 0; i < len(as); {
		if strings.Contains(as[i].value, "\n") {
			w.addc(as[i].name+" = "+as[i].value, as[i].comments)
			i++
			continue
		}
		j, pad := i, 0
		for j < len(as) && !strings.Contains(as[j].value, "\n") && (j == i || len(as[j].comments.Head) == 0) {
			pad = max(pad, width(as[j].name))
			j++
		}
		for _, a := range as[i:j] {
			w.addc(a.name+strings.Repeat(" ", pad-width(a.name))+" = "+a.value, a.comments)
		}
		i = j
	}
}

func (w *writer) textAssign(as *[]assign, name string, s *osf.Str) {
	if s != nil {
		*as = append(*as, assign{name, text(s.Value, w.depth), s.Comments})
	}
}

func (w *writer) textField(name string, s *osf.Str) {
	if s != nil {
		w.addc(name+": "+text(s.Value, w.depth), s.Comments)
	}
}

func value(v osf.Value) string { return v.String() }

func valueMap(name string, as []osf.Assign) string {
	if len(as) == 0 {
		return ""
	}
	parts := make([]string, len(as))
	for i, a := range as {
		parts[i] = a.SetString()
		if name == "requires" {
			parts[i] = a.RequiresString()
		}
	}
	return name + ": { " + strings.Join(parts, ", ") + " }"
}

func typeName(t osf.VarType) string { return t.Name() }
