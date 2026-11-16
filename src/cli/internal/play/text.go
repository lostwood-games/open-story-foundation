package play

import (
	"regexp"
	"sort"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// refInText matches {{ Kind.id }} and {{ Kind.id.field }}, like check.TextRefs.
var refInText = regexp.MustCompile(`\{\{\s*([A-Za-z]+)\.([A-Za-z0-9_-]+)(?:\.([A-Za-z0-9_.-]+))?\s*\}\}`)

// index resolves ids and translation keys for one project and locale.
type index struct {
	p          *osf.Project
	locale     string
	texts      map[string]string // translation key -> text in the locale
	characters map[string]*osf.Character
	documents  map[string]*osf.Document
	stories    map[string]*osf.Story
	dialogues  map[string]*osf.Dialogue
	variables  map[string]*osf.Variable
}

func newIndex(p *osf.Project, locale string) *index {
	ix := &index{
		p:          p,
		locale:     locale,
		texts:      map[string]string{},
		characters: map[string]*osf.Character{},
		documents:  map[string]*osf.Document{},
		stories:    map[string]*osf.Story{},
		dialogues:  map[string]*osf.Dialogue{},
		variables:  map[string]*osf.Variable{},
	}
	for _, c := range p.Characters {
		ix.characters[c.ID] = c
	}
	for _, d := range p.Documents {
		ix.documents[d.ID] = d
	}
	for _, s := range p.Stories {
		ix.stories[s.ID] = s
	}
	for _, d := range p.Dialogues {
		ix.dialogues[d.ID] = d
	}
	for _, v := range p.Variables {
		ix.variables[v.ID] = v
	}
	if locale != "" {
		for _, l := range p.Locales {
			if l.Language.Value != locale {
				continue
			}
			for _, v := range l.Values {
				ix.texts[v.Key] = v.Text.Value
			}
		}
	}
	return ix
}

// Locales lists the locale ids a project can be played in, sorted: the ids
// declared in I18n and those with Locale resources, plus the source language.
func Locales(p *osf.Project) []string {
	seen := map[string]bool{}
	for _, i := range p.I18n {
		if i.Source != nil {
			seen[i.Source.Value] = true
		}
		for _, l := range i.Locales {
			seen[l.ID] = true
		}
	}
	for _, l := range p.Locales {
		seen[l.Language.Value] = true
	}
	var out []string
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// text returns the translation of key in the locale, or the source text.
// A nil source gives "".
func (ix *index) text(key string, src *osf.Str) string {
	if t, ok := ix.texts[key]; ok {
		return t
	}
	if src == nil {
		return ""
	}
	return src.Value
}

// resolve replaces the {{ }} references in text (spec §4) and turns \{{ into
// {{. Variables read their current value from vars; unknown references are
// left as written.
func (ix *index) resolve(text string, vars map[string]osf.Value) string {
	var b strings.Builder
	last := 0
	for _, m := range refInText.FindAllStringSubmatchIndex(text, -1) {
		if m[0] > 0 && text[m[0]-1] == '\\' {
			continue
		}
		kind, id := text[m[2]:m[3]], text[m[4]:m[5]]
		field := "name"
		if m[6] >= 0 {
			field = text[m[6]:m[7]]
		}
		value, ok := ix.lookup(kind, id, field, vars)
		if !ok {
			continue
		}
		b.WriteString(text[last:m[0]])
		b.WriteString(value)
		last = m[1]
	}
	b.WriteString(text[last:])
	return strings.ReplaceAll(b.String(), `\{{`, "{{")
}

func (ix *index) lookup(kind, id, field string, vars map[string]osf.Value) (string, bool) {
	key := kind + "." + id + "." + field
	switch kind {
	case "Character":
		c, ok := ix.characters[id]
		if !ok {
			return "", false
		}
		switch field {
		case "name":
			return ix.text(key, c.Name), true
		case "description":
			return ix.text(key, c.Description), true
		}
	case "Document":
		if d, ok := ix.documents[id]; ok && field == "name" {
			return ix.text(key, d.Name), true
		}
	case "Story":
		s, ok := ix.stories[id]
		if !ok {
			return "", false
		}
		switch field {
		case "name":
			return ix.text(key, s.Name), true
		case "description":
			return ix.text(key, s.Description), true
		}
	case "Dialogue":
		d, ok := ix.dialogues[id]
		if !ok || field != "name" {
			return "", false
		}
		if d.Name == nil {
			return d.ID, true
		}
		return ix.text(key, d.Name), true
	case "Variable":
		if v, ok := vars[id]; ok && field == "name" {
			return plain(v), true
		}
	}
	return "", false
}

// plain is a variable value as shown inside text.
func plain(v osf.Value) string { return v.String() }

// startNode is the node a dialogue starts at: startNode when set, else the
// first node. It is nil for an empty dialogue.
func startNode(d *osf.Dialogue) *osf.Node {
	if d.StartNode != nil {
		for _, n := range d.Nodes {
			if n.ID == d.StartNode.Value {
				return n
			}
		}
	}
	if len(d.Nodes) > 0 {
		return d.Nodes[0]
	}
	return nil
}

func findNode(d *osf.Dialogue, id string) *osf.Node {
	for _, n := range d.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// typeOf is the type of variable name; an unknown one counts as a number.
func (ix *index) typeOf(name string) osf.VarType {
	if v := ix.variables[name]; v != nil {
		return v.Type
	}
	return osf.Number
}
