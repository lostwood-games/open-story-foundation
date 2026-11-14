package yamlio

import (
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// Comments travel between .osf and YAML as YAML comments: a `// text` line is
// `# text`, each line of a `/* */` comment is `# ` plus the line, and a blank
// line between comments stays blank. The
// lines before a block that stand apart from it (a blank line between) are
// the document's head comment; the lines directly above it are the head
// comment of the apiVersion key.

// yamlComment renders .osf comment lines as one YAML comment block.
func yamlComment(lines []string) string {
	var out []string
	inBlock := false
	for _, l := range lines {
		switch {
		case inBlock:
			out = append(out, hashLine(l))
			inBlock = !strings.HasSuffix(strings.TrimRight(l, " \t"), "*/")
		case l == "":
			out = append(out, "")
		case strings.HasPrefix(l, "//"):
			out = append(out, "#"+l[2:])
		default: // first line of a /* */ comment
			out = append(out, hashLine(l))
			inBlock = !strings.HasSuffix(strings.TrimRight(l, " \t"), "*/")
		}
	}
	return strings.Join(out, "\n")
}

func hashLine(l string) string {
	if l == "" {
		return "#"
	}
	return "# " + l
}

// lineComment renders a trailing .osf comment as a YAML line comment.
func lineComment(trail string) string {
	if trail == "" {
		return ""
	}
	return yamlComment([]string{trail})
}

// osfComment turns a YAML comment block back into .osf comment lines. A run
// of lines from `/*` to `*/` is a block comment and keeps its lines as they
// were; every other line becomes a `//` comment. Editor directives and the
// head comment osf build writes are not comments of the resource.
func osfComment(text string) []string {
	if text == "" {
		return nil
	}
	var bodies []string // the text after #, or "\n" for a blank line
	for _, l := range strings.Split(text, "\n") {
		switch {
		case generatedFrom.MatchString(l) || strings.HasPrefix(strings.TrimSpace(l), "# yaml-language-server:"):
		case strings.HasPrefix(l, "#"):
			bodies = append(bodies, l[1:])
		default:
			bodies = append(bodies, "\n")
		}
	}
	var out []string
	for i := 0; i < len(bodies); {
		b := bodies[i]
		if strings.HasPrefix(strings.TrimLeft(b, " \t"), "/*") {
			if end := blockEnd(bodies, i); end >= 0 {
				for _, r := range bodies[i : end+1] {
					out = append(out, strings.TrimPrefix(strings.ReplaceAll(r, "\n", ""), " "))
				}
				i = end + 1
				continue
			}
		}
		if b == "\n" {
			out = append(out, "")
		} else {
			out = append(out, "//"+b)
		}
		i++
	}
	return out
}

// blockEnd finds the line closing the /* */ comment that starts at line i,
// or -1 when it is not closed cleanly.
func blockEnd(bodies []string, i int) int {
	for j := i; j < len(bodies); j++ {
		if k := strings.Index(bodies[j], "*/"); k >= 0 {
			if strings.TrimRight(bodies[j], " \t") == bodies[j][:k+2] {
				return j
			}
			return -1
		}
	}
	return -1
}

// osfLine turns a YAML line comment into a trailing .osf comment.
func osfLine(comment string) string {
	lines := osfComment(comment)
	if len(lines) != 1 {
		return ""
	}
	return lines[0]
}

// splitHead separates the comment lines that stand apart from a block (up to
// the last blank line) from those directly above it.
func splitHead(lines []string) (apart, above []string) {
	last := -1
	for i, l := range lines {
		if l == "" {
			last = i
		}
	}
	if last < 0 {
		return nil, lines
	}
	return trimBlank(lines[:last]), lines[last+1:]
}

func trimBlank(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// comment attaches c to a key and its value: the lines above as the key's
// head comment, the trailing one on the value, or on the key when the value
// is a mapping or a sequence (that is where yaml.v3 prints it).
func comment(key, value *yaml.Node, c osf.Comments) {
	key.HeadComment = yamlComment(trimBlank(c.Head))
	if value.Kind == yaml.ScalarNode {
		value.LineComment = lineComment(c.Trail)
	} else {
		key.LineComment = lineComment(c.Trail)
	}
}

// comments reads the comments of the entry key: value back. prev is the
// previous key of the mapping, whose foot comment is the part of this
// entry's head comment that a blank line separated (yaml.v3 splits it so).
func comments(prevKey, prevValue, key, value *yaml.Node) osf.Comments {
	var c osf.Comments
	for _, n := range []*yaml.Node{prevKey, prevValue} {
		if n != nil && n.FootComment != "" {
			c.Head = append(c.Head, osfComment(n.FootComment)...)
			c.Head = append(c.Head, "")
		}
	}
	c.Head = append(c.Head, osfComment(key.HeadComment)...)
	if value != nil {
		c.Head = append(c.Head, osfComment(value.HeadComment)...)
		c.Trail = osfLine(value.LineComment)
	}
	if c.Trail == "" {
		c.Trail = osfLine(key.LineComment)
	}
	return c
}
