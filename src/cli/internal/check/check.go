// Package check validates an Open Story project against the rules of
// docs/spec.md: required fields, ids, references, variable values and
// translations. Each problem is reported as a diag.Diagnostic carrying the code
// of its rule.
package check

import (
	"regexp"
	"sort"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/diag"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

type Result struct {
	Errors   []diag.Diagnostic
	Warnings []diag.Diagnostic
}

// All returns errors and warnings together, unsorted.
func (r Result) All() []diag.Diagnostic {
	return append(append([]diag.Diagnostic(nil), r.Errors...), r.Warnings...)
}

func (r *Result) add(d diag.Diagnostic) {
	if d.Severity == diag.Warning {
		r.Warnings = append(r.Warnings, d)
	} else {
		r.Errors = append(r.Errors, d)
	}
}

// at reports rule at the range of s, falling back to pos when s has no
// position of its own (YAML resources without a node for the field).
func (r *Result) at(rule diag.Rule, s *osf.Str, pos diag.Pos, format string, args ...any) {
	if s != nil && s.Pos.File != "" {
		r.add(rule.At(s.Pos, s.End, format, args...))
		return
	}
	r.add(rule.At(pos, diag.Pos{}, format, args...))
}

// suggest is like at, with a "did you mean" candidate when one is close.
func (r *Result) suggest(rule diag.Rule, s *osf.Str, pos diag.Pos, name string, candidates []string, format string, args ...any) {
	r.at(rule, s, pos, format, args...)
	list := &r.Errors
	if rule.Severity == diag.Warning {
		list = &r.Warnings
	}
	(*list)[len(*list)-1].Suggestion = Suggest(name, candidates)
}

// refInText matches {{ Kind.id }} and {{ Kind.id.field }}.
var refInText = regexp.MustCompile(`\{\{\s*([A-Za-z]+)\.([A-Za-z0-9_-]+)(?:\.([A-Za-z0-9_.-]+))?\s*\}\}`)

// TextRefs returns the {{ }} references in text, sorted, skipping \{{.
func TextRefs(text string) []string {
	var refs []string
	for _, m := range refInText.FindAllStringSubmatchIndex(text, -1) {
		if m[0] > 0 && text[m[0]-1] == '\\' {
			continue
		}
		ref := text[m[2]:m[3]] + "." + text[m[4]:m[5]]
		if m[6] >= 0 {
			ref += "." + text[m[6]:m[7]]
		}
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs
}

// textFields lists, per kind, the fields a {{ Kind.id.field }} reference in
// text may name (spec §4); the first one is the default.
var textFields = map[string][]string{
	"Story":     {"name", "description"},
	"Character": {"name", "description"},
	"Document":  {"name"},
	"Dialogue":  {"name"},
}

// span is the header range of a resource, for diagnostics about the whole
// resource rather than one field.
func span(pos, end diag.Pos) *osf.Str { return &osf.Str{Pos: pos, End: end} }

func Project(p *osf.Project) Result {
	var r Result
	defined := map[string]diag.Pos{} // Kind.id -> where
	ids := map[string][]string{}     // Kind -> ids, for suggestions
	define := func(kind, id string, pos, end diag.Pos) {
		if !osf.IDPattern.MatchString(id) {
			r.at(diag.InvalidID, span(pos, end), pos, "%s id %q may only contain letters, digits, _ and -", kind, id)
		}
		key := kind + "." + id
		if prev, ok := defined[key]; ok {
			r.at(diag.DuplicateID, span(pos, end), pos, "%s is already defined at %s", key, prev)
			return
		}
		defined[key] = pos
		ids[kind] = append(ids[kind], id)
	}
	refs := func(kind string) []string {
		var out []string
		for _, id := range ids[kind] {
			out = append(out, kind+"."+id)
		}
		return out
	}

	switch len(p.Stories) {
	case 0:
		r.add(diag.NoStory.At(diag.Pos{}, diag.Pos{}, "the project has no Story; add one Story block"))
	case 1:
	default:
		for _, s := range p.Stories[1:] {
			r.at(diag.DuplicateStory, span(s.Pos, s.End), s.Pos, "a project has exactly one Story; the first is at %s", p.Stories[0].Pos)
		}
	}
	for _, s := range p.Stories {
		define("Story", s.ID, s.Pos, s.End)
		if s.Name == nil {
			r.at(diag.MissingField, span(s.Pos, s.End), s.Pos, "Story %s has no name", s.ID)
		}
	}
	for _, c := range p.Characters {
		define("Character", c.ID, c.Pos, c.End)
		if c.Name == nil {
			r.at(diag.MissingField, span(c.Pos, c.End), c.Pos, "Character %s has no name", c.ID)
		}
	}
	for _, d := range p.Documents {
		define("Document", d.ID, d.Pos, d.End)
		if d.Name == nil {
			r.at(diag.MissingField, span(d.Pos, d.End), d.Pos, "Document %s has no name", d.ID)
		}
	}
	variables := map[string]*osf.Variable{}
	for _, v := range p.Variables {
		define("Variable", v.ID, v.Pos, v.End)
		variables[v.ID] = v
		if !v.Type.Accepts(v.Default) {
			r.at(diag.BadDefault, span(v.Pos, v.End), v.Pos, "Variable %s is %s but its default is %s", v.ID, v.Type, v.Default)
		}
	}
	// graph maps each dialogue to its nodes and each node to its options, for
	// played and selected conditions.
	graph := map[string]map[string]map[string]bool{}
	for _, d := range p.Dialogues {
		define("Dialogue", d.ID, d.Pos, d.End)
		if graph[d.ID] == nil {
			graph[d.ID] = map[string]map[string]bool{}
		}
		for _, n := range d.Nodes {
			graph[d.ID][n.ID] = map[string]bool{}
			for _, o := range n.Options {
				graph[d.ID][n.ID][o.ID] = true
			}
		}
	}
	keysOf := func(m map[string]bool) []string {
		var out []string
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	nodesOf := func(dlg string) []string {
		var out []string
		for k := range graph[dlg] {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	// checkHistory checks a played or selected condition (spec §2.5.3):
	// [Dialogue.<id>.]<node>.played or [Dialogue.<id>.]<node>.<option>.selected,
	// compared with true or false, in requires only.
	checkHistory := func(where string, pos diag.Pos, isSet bool, a osf.Assign, dlg string) {
		at := &osf.Str{Value: a.Name, Pos: a.Pos, End: a.End}
		parts := strings.Split(a.Name, ".")
		if parts[0] == "Dialogue" && len(parts) > 2 {
			dlg, parts = parts[1], parts[2:]
			if graph[dlg] == nil {
				r.suggest(diag.UnknownDialogue, at, pos, "Dialogue."+dlg, refs("Dialogue"), "%s: Dialogue.%s does not exist", where, dlg)
				return
			}
		}
		last := parts[len(parts)-1]
		switch {
		case isSet:
			r.at(diag.BadHistoryCondition, at, pos, "%s: %s is a condition; played and selected belong in requires", where, a.Name)
			return
		case !(last == "played" && len(parts) == 2 || last == "selected" && len(parts) == 3):
			r.at(diag.BadHistoryCondition, at, pos, "%s: %q is neither a variable nor <node>.played or <node>.<option>.selected", where, a.Name)
			return
		case a.Op != "" || a.Value.Kind != osf.Bool:
			r.at(diag.BadHistoryCondition, at, pos, "%s: %s can only be compared with true or false, as %s: true", where, a.Name, a.Name)
			return
		}
		options, ok := graph[dlg][parts[0]]
		if !ok {
			r.suggest(diag.UnknownNode, at, pos, parts[0], nodesOf(dlg), "%s: node %q does not exist in Dialogue %s", where, parts[0], dlg)
		} else if last == "selected" && !options[parts[1]] {
			r.suggest(diag.UnknownOption, at, pos, parts[1], keysOf(options), "%s: Node %s in Dialogue %s has no option %q", where, parts[0], dlg, parts[1])
		}
	}

	plainForm := map[bool]string{true: "set to true or false", false: "compared with true or false"}
	checkValues := func(where string, pos diag.Pos, isSet bool, values []osf.Assign, dlg string) {
		for _, a := range values {
			if strings.Contains(a.Name, ".") {
				checkHistory(where, pos, isSet, a, dlg)
				continue
			}
			at := &osf.Str{Value: a.Name, Pos: a.Pos, End: a.End}
			v, ok := variables[a.Name]
			switch {
			case !ok:
				r.suggest(diag.UnknownVariable, at, pos, a.Name, ids["Variable"], "%s: unknown variable %q", where, a.Name)
			case a.Op != "" && !v.Type.IsNumber():
				r.at(diag.NotANumber, at, pos, "%s: %s is boolean, so it can only be %s", where, a.Name, plainForm[isSet])
			case a.Op != "" && a.Value.Kind != osf.Int:
				r.at(diag.WrongValueType, at, pos, "%s: %s is %s, got %s", where, a.Name, v.Type, a.Value)
			case isSet && a.Op != "" && a.Value.Int < 0:
				r.at(diag.NegativeAmount, at, pos, "%s: cannot %s %d; use a number of 0 or more", where, a.Op, a.Value.Int)
			case isSet && a.Op == "" && !v.Type.Accepts(a.Value), !isSet && (a.Value.Kind == osf.Int) != v.Type.IsNumber():
				r.at(diag.WrongValueType, at, pos, "%s: %s is %s, got %s", where, a.Name, v.Type, a.Value)
			}
		}
	}

	for _, s := range p.Stories {
		if s.Start != nil {
			if _, ok := defined[s.Start.Value]; !ok || !strings.HasPrefix(s.Start.Value, "Dialogue.") {
				r.suggest(diag.UnknownStart, s.Start, s.Pos, s.Start.Value, refs("Dialogue"), "start: %s does not exist", s.Start.Value)
			}
		}
	}

	for _, d := range p.Dialogues {
		if len(d.Nodes) == 0 {
			r.at(diag.NoNodes, span(d.Pos, d.End), d.Pos, "Dialogue %s has no nodes", d.ID)
			continue
		}
		nodes := map[string]bool{}
		var nodeIDs []string
		for _, n := range d.Nodes {
			if !osf.IDPattern.MatchString(n.ID) {
				r.at(diag.InvalidID, span(n.Pos, n.End), n.Pos, "node id %q may only contain letters, digits, _ and -", n.ID)
			}
			nodes[n.ID] = true
			nodeIDs = append(nodeIDs, n.ID)
		}
		start := d.Nodes[0].ID
		if d.StartNode != nil {
			start = d.StartNode.Value
			if !nodes[start] {
				r.suggest(diag.UnknownStartNode, d.StartNode, d.Pos, start, nodeIDs, "start node %q is not a node of Dialogue %s", start, d.ID)
			}
		}
		reached := map[string]bool{}
		checkNext := func(where string, pos diag.Pos, next *osf.Str) {
			if next == nil {
				return
			}
			if strings.HasPrefix(next.Value, "Dialogue.") {
				if _, ok := defined[next.Value]; !ok {
					r.suggest(diag.UnknownDialogue, next, pos, next.Value, refs("Dialogue"), "%s: next: %s does not exist", where, next.Value)
				}
			} else if !nodes[next.Value] {
				r.suggest(diag.UnknownNode, next, pos, next.Value, nodeIDs, "%s: next: node %q does not exist in Dialogue %s", where, next.Value, d.ID)
			} else {
				reached[next.Value] = true
			}
		}
		for _, n := range d.Nodes {
			where := "Node " + n.ID
			if n.Text == nil {
				r.at(diag.MissingField, span(n.Pos, n.End), n.Pos, "%s has no text", where)
			}
			if (n.Next != nil || len(n.Branches) > 0) && len(n.Options) > 0 {
				r.at(diag.NextAndOptions, span(n.Pos, n.End), n.Pos, "%s has both next and options; use one or the other", where)
			}
			if n.Speaker != nil {
				if _, ok := defined[n.Speaker.Value]; !ok || !strings.HasPrefix(n.Speaker.Value, "Character.") {
					id := strings.TrimPrefix(n.Speaker.Value, "Character.")
					r.suggest(diag.UnknownSpeaker, n.Speaker, n.Pos, id, ids["Character"], "%s: speaker: unknown character %q", where, id)
				}
			}
			for _, b := range n.Branches {
				checkNext(where, n.Pos, b.To)
				checkValues(where+": next "+b.To.Value+" requires", n.Pos, false, b.Requires, d.ID)
			}
			checkNext(where, n.Pos, n.Next)
			checkValues(where+": set", n.Pos, true, n.Set, d.ID)
			for _, o := range n.Options {
				ow := where + ", Option " + o.ID
				if !osf.IDPattern.MatchString(o.ID) {
					r.at(diag.InvalidID, span(o.Pos, o.End), o.Pos, "option id %q may only contain letters, digits, _ and -", o.ID)
				}
				if o.Text == nil {
					r.at(diag.MissingField, span(o.Pos, o.End), o.Pos, "%s has no text", ow)
				}
				for _, b := range o.Branches {
					checkNext(ow, o.Pos, b.To)
					checkValues(ow+": next "+b.To.Value+" requires", o.Pos, false, b.Requires, d.ID)
				}
				checkNext(ow, o.Pos, o.Next)
				checkValues(ow+": requires", o.Pos, false, o.Requires, d.ID)
				checkValues(ow+": set", o.Pos, true, o.Set, d.ID)
			}
		}
		for _, n := range d.Nodes {
			if n.ID != start && !reached[n.ID] {
				r.at(diag.UnreachableNode, span(n.Pos, n.End), n.Pos, "Node %s in Dialogue %s is never reached", n.ID, d.ID)
			}
		}
	}

	texts := p.Texts()
	textByKey := map[string]*osf.Str{}
	for _, t := range texts {
		textByKey[t.Key] = t.Text
		for _, ref := range TextRefs(t.Text.Value) {
			parts := strings.SplitN(ref, ".", 3)
			target := parts[0] + "." + parts[1]
			if _, ok := defined[target]; !ok {
				var candidates []string
				for _, c := range refs(parts[0]) {
					if len(parts) == 3 {
						c += "." + parts[2]
					}
					candidates = append(candidates, c)
				}
				r.suggest(diag.UnknownTextReference, t.Text, diag.Pos{}, ref, candidates, "%s: reference {{ %s }} does not exist", t.Key, ref)
			} else if len(parts) == 3 && parts[0] == "Variable" {
				r.at(diag.BadTextReferenceField, t.Text, diag.Pos{}, "%s: reference {{ %s }}: a Variable reference takes no field; write {{ %s }}", t.Key, ref, target)
			} else if len(parts) == 3 && !contains(textFields[parts[0]], parts[2]) {
				r.at(diag.BadTextReferenceField, t.Text, diag.Pos{}, "%s: reference {{ %s }}: %s has no text field %q", t.Key, ref, target, parts[2])
			}
		}
	}

	checkI18n(p, &r, texts, textByKey)
	return r
}

func checkI18n(p *osf.Project, r *Result, texts []osf.TextEntry, textByKey map[string]*osf.Str) {
	usesI18n := len(p.I18n) > 0 || len(p.Locales) > 0
	if usesI18n && p.PluginsSet && !contains(p.Plugins, "i18n/v1") {
		r.add(diag.PluginNotEnabled.At(p.PluginsPos, diag.Pos{}, "i18n/v1 resources are used but i18n/v1 is not listed in the plugins Configuration"))
	} else if usesI18n && !p.PluginsSet {
		r.add(diag.PluginNotEnabled.At(diag.Pos{}, diag.Pos{}, "i18n/v1 resources are used but there is no plugins/v1 Configuration listing i18n/v1"))
	}
	for _, c := range p.I18n[min(1, len(p.I18n)):] {
		r.at(diag.DuplicateI18n, span(c.Pos, c.End), c.Pos, "a project has one I18n configuration; the first is at %s", p.I18n[0].Pos)
	}

	type localeInfo struct {
		enforce bool
		pos     diag.Pos
		end     diag.Pos
	}
	known := map[string]localeInfo{}
	var order []string
	if len(p.I18n) > 0 {
		c := p.I18n[0]
		if c.Source != nil && !osf.LocalePattern.MatchString(c.Source.Value) {
			r.at(diag.InvalidLocaleID, c.Source, c.Pos, "source %q is not a locale id: three letters, capitalised, like Eng", c.Source.Value)
		}
		for _, l := range c.Locales {
			at := span(l.Pos, l.End)
			if !osf.LocalePattern.MatchString(l.ID) {
				r.at(diag.InvalidLocaleID, at, c.Pos, "locale id %q must be three letters, capitalised, like Rus", l.ID)
			}
			if _, dup := known[l.ID]; dup {
				r.at(diag.DuplicateLocale, at, c.Pos, "locale %s is listed twice", l.ID)
				continue
			}
			known[l.ID] = localeInfo{l.Enforce, l.Pos, l.End}
			order = append(order, l.ID)
		}
	}

	var keys []string
	for _, t := range texts {
		keys = append(keys, t.Key)
	}
	translated := map[[2]string]diag.Pos{}
	for _, loc := range p.Locales {
		lang := loc.Language.Value
		if !osf.LocalePattern.MatchString(lang) {
			r.at(diag.InvalidLocaleID, &loc.Language, loc.Pos, "locale id %q must be three letters, capitalised, like Rus", lang)
		} else if _, ok := known[lang]; !ok && len(p.I18n) > 0 {
			r.suggest(diag.UnknownLocale, &loc.Language, loc.Pos, lang, order, "language %s is not listed in the I18n configuration", lang)
		}
		for _, v := range loc.Values {
			src, ok := textByKey[v.Key]
			if !ok {
				r.suggest(diag.UnknownTranslationKey, &v.Text, loc.Pos, v.Key, keys, "%s is not a text in the story (renamed or removed?)", v.Key)
				continue
			}
			k := [2]string{lang, v.Key}
			if prev, dup := translated[k]; dup {
				r.at(diag.DuplicateTranslation, &v.Text, loc.Pos, "%s is already translated at %s", v.Key, prev)
				continue
			}
			translated[k] = firstPos(v.Text.Pos, loc.Pos)
			if strings.Join(TextRefs(v.Text.Value), ",") != strings.Join(TextRefs(src.Value), ",") {
				r.at(diag.ReferencesDiffer, &v.Text, loc.Pos, "%s: {{ }} references differ from the source text", v.Key)
			}
		}
	}

	for _, lang := range order {
		info := known[lang]
		var missing []string
		for _, t := range texts {
			if _, ok := translated[[2]string{lang, t.Key}]; !ok {
				missing = append(missing, t.Key)
			}
		}
		if info.enforce {
			for _, k := range missing {
				r.at(diag.MissingTranslation, textByKey[k], diag.Pos{}, "missing %s translation for %s", lang, k)
			}
		} else if len(missing) > 0 {
			r.at(diag.Untranslated, span(info.pos, info.end), info.pos, "%s: %d of %d texts untranslated (not enforced)", lang, len(missing), len(texts))
		}
	}
}

func firstPos(a, b diag.Pos) diag.Pos {
	if a.File != "" {
		return a
	}
	return b
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
