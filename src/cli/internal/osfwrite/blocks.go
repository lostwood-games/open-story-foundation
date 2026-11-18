package osfwrite

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// Block kinds in the order language.md §11 puts them in a file.
const (
	kindVersions = iota
	kindStory
	kindI18n
	kindCharacter
	kindDocument
	kindVariable
	kindDialogue
	kindLocale
)

type block struct {
	kind  int
	lines []string
}

func render(kind int, body func(w *writer)) block {
	w := &writer{}
	body(w)
	return block{kind, w.lines}
}

// versions writes the Versions block: language first, then the built-in
// plugins and the others in name order (language.md §3.1).
func versions(p *osf.Project) block {
	return render(kindVersions, func(w *writer) {
		w.block("Versions", p.PluginsComments, func() {
			w.assigns([]assign{{name: "language", value: `"1"`}})
			w.blank()
			api := func(name string) string {
				if v := p.APIVersions[name]; v != "" {
					return v
				}
				return "v1"
			}
			as := []assign{{name: "story", value: quote(api("story"))}, {name: "dialogue", value: quote(api("dialogue"))}}
			others := map[string]string{}
			for name, v := range p.APIVersions {
				if name != "story" && name != "dialogue" && name != "plugins" {
					others[name] = v
				}
			}
			for _, pl := range p.Plugins {
				if name, v, ok := strings.Cut(pl, "/"); ok && others[name] == "" {
					others[name] = v
				}
			}
			names := make([]string, 0, len(others))
			for name := range others {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				as = append(as, assign{name, quote(others[name]), p.PluginComments[name+"/"+others[name]]})
			}
			w.assigns(as)
		})
	})
}

func story(w *writer, s *osf.Story) {
	w.block("Story "+s.ID, s.Comments, func() {
		var as []assign
		w.textAssign(&as, "name", s.Name)
		w.textAssign(&as, "description", s.Description)
		if s.Start != nil {
			as = append(as, assign{"start", s.Start.Value, s.Start.Comments})
		}
		w.assigns(as)
	})
}

func i18n(w *writer, c *osf.I18n) {
	w.block("I18n", c.Comments, func() {
		if c.Source != nil {
			w.assigns([]assign{{"source", c.Source.Value, c.Source.Comments}})
		}
		for _, l := range c.Locales {
			line := "Locale " + l.ID + " " + quote(l.Name)
			if l.Enforce {
				line += " { enforce = true }"
			}
			w.addc(line, l.Comments)
		}
	})
}

func character(w *writer, c *osf.Character) {
	w.block("Character "+c.ID, c.Comments, func() {
		var as []assign
		w.textAssign(&as, "name", c.Name)
		w.textAssign(&as, "description", c.Description)
		w.assigns(as)
	})
}

func document(w *writer, d *osf.Document) {
	w.block("Document "+d.ID, d.Comments, func() {
		var as []assign
		w.textAssign(&as, "name", d.Name)
		w.assigns(as)
	})
}

func variable(w *writer, v *osf.Variable) {
	line := "Variable " + v.ID + ": " + typeName(v.Type) + " = " + value(v.Default)
	if v.Description != nil {
		line += " { description = " + quote(v.Description.Value) + " }"
	}
	w.addc(line, v.Comments)
}

// dialogue writes a Dialogue block; inLocale leaves out everything but text
// and writes text-only options on one line.
func dialogue(w *writer, d *osf.Dialogue, inLocale bool) {
	w.block("Dialogue "+d.ID, d.Comments, func() {
		var as []assign
		w.textAssign(&as, "name", d.Name)
		w.assigns(as)
		if d.StartNode != nil && (len(d.Nodes) == 0 || d.StartNode.Value != d.Nodes[0].ID) {
			w.addc("start "+d.StartNode.Value, d.StartNode.Comments)
		}
		for _, n := range d.Nodes {
			w.blank()
			node(w, n, inLocale)
		}
	})
}

func node(w *writer, n *osf.Node, inLocale bool) {
	w.block("Node "+n.ID, n.Comments, func() {
		if n.Speaker != nil {
			w.addc("speaker: "+strings.TrimPrefix(n.Speaker.Value, "Character."), n.Speaker.Comments)
		}
		w.textField("title", n.Title)
		w.textField("text", n.Text)
		if s := valueMap("set", n.Set); s != "" {
			w.addc(s, n.SetComments)
		}
		nexts(w, n.Branches, n.Next)
		for _, o := range n.Options {
			oneLine := inLocale && o.Text != nil && !strings.Contains(text(o.Text.Value, w.depth), "\n")
			if !oneLine || len(w.lines) == 0 || !strings.HasPrefix(strings.TrimSpace(w.lines[len(w.lines)-1]), "Option ") {
				w.blank()
			}
			if oneLine {
				// One statement, so the text's comments are the option's.
				w.addc("Option "+o.ID+" { text: "+text(o.Text.Value, w.depth)+" }", o.Text.Comments)
				continue
			}
			w.block("Option "+o.ID, o.Comments, func() {
				w.textField("text", o.Text)
				if s := requires(o.Requires, o.RequiresOr); s != "" {
					w.addc("requires:"+s, o.RequiresComments)
				}
				if s := valueMap("set", o.Set); s != "" {
					w.addc(s, o.SetComments)
				}
				nexts(w, o.Branches, o.Next)
			})
		}
	})
}

// nexts writes the branches in order, each `next: to requires { ... }`, then
// the plain `next:`.
func nexts(w *writer, branches []*osf.Branch, next *osf.Str) {
	for _, b := range branches {
		w.addc("next: "+b.To.Value+" requires"+requires(b.Requires, b.RequiresOr), b.Comments)
	}
	if next != nil {
		w.addc("next: "+next.Value, next.Comments)
	}
}

// locale writes a Locale block, turning translation keys back into Story,
// Character, Document and Dialogue blocks that hold only text.
func locale(l *osf.Locale) (block, error) {
	var stories []*osf.Story
	var characters []*osf.Character
	var documents []*osf.Document
	var dialogues []*osf.Dialogue
	for _, v := range l.Values {
		s := v.Text
		parts := strings.Split(v.Key, ".")
		bad := fmt.Errorf("%s: translation key %s cannot be written as a Locale block", l.Language.Pos, v.Key)
		if len(parts) < 3 {
			return block{}, bad
		}
		kind, id := parts[0], parts[1]
		switch {
		case kind == "Story" && len(parts) == 3 && (parts[2] == "name" || parts[2] == "description"):
			st := find(&stories, id, func(s *osf.Story) string { return s.ID }, func() *osf.Story { return &osf.Story{ID: id} })
			if parts[2] == "name" {
				st.Name = &s
			} else {
				st.Description = &s
			}
		case kind == "Character" && len(parts) == 3 && (parts[2] == "name" || parts[2] == "description"):
			c := find(&characters, id, func(c *osf.Character) string { return c.ID }, func() *osf.Character { return &osf.Character{ID: id} })
			if parts[2] == "name" {
				c.Name = &s
			} else {
				c.Description = &s
			}
		case kind == "Document" && len(parts) == 3 && parts[2] == "name":
			d := find(&documents, id, func(d *osf.Document) string { return d.ID }, func() *osf.Document { return &osf.Document{ID: id} })
			d.Name = &s
		case kind == "Dialogue" && len(parts) == 3 && parts[2] == "name":
			d := find(&dialogues, id, func(d *osf.Dialogue) string { return d.ID }, func() *osf.Dialogue { return &osf.Dialogue{ID: id} })
			d.Name = &s
		case kind == "Dialogue" && len(parts) >= 6 && parts[2] == "nodes":
			d := find(&dialogues, id, func(d *osf.Dialogue) string { return d.ID }, func() *osf.Dialogue { return &osf.Dialogue{ID: id} })
			n := find(&d.Nodes, parts[3], func(n *osf.Node) string { return n.ID }, func() *osf.Node { return &osf.Node{ID: parts[3]} })
			switch {
			case len(parts) == 6 && parts[4] == "content" && parts[5] == "title":
				n.Title = &s
			case len(parts) == 6 && parts[4] == "content" && parts[5] == "text":
				n.Text = &s
			case len(parts) == 8 && parts[4] == "options" && parts[6] == "content" && parts[7] == "text":
				o := find(&n.Options, parts[5], func(o *osf.OptionModel) string { return o.ID }, func() *osf.OptionModel { return &osf.OptionModel{ID: parts[5]} })
				o.Text = &s
			default:
				return block{}, bad
			}
		default:
			return block{}, bad
		}
	}
	return render(kindLocale, func(w *writer) {
		w.block("Locale "+l.Language.Value, l.Comments, func() {
			for _, s := range stories {
				w.blank()
				story(w, s)
			}
			for _, c := range characters {
				w.blank()
				character(w, c)
			}
			for _, d := range documents {
				w.blank()
				document(w, d)
			}
			for _, d := range dialogues {
				w.blank()
				dialogue(w, d, true)
			}
		})
	}), nil
}

// find returns the item of list with the given id, appending a new one when
// it is missing, so blocks keep the order the keys were first seen in.
func find[T any](list *[]T, id string, idOf func(T) string, make func() T) T {
	for _, x := range *list {
		if idOf(x) == id {
			return x
		}
	}
	x := make()
	*list = append(*list, x)
	return x
}
