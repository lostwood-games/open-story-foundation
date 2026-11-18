package play

import (
	"sort"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// history records what was played: "<dialogue>.<node>" once a node was
// entered and "<dialogue>.<node>.<option>" once an option was chosen.
type history map[string]bool

func (h history) has(key string) bool { return h[key] }

// historyReader answers whether a history key is set: a history map, or a
// session that asks its Option objects.
type historyReader interface {
	has(key string) bool
}

func playedKey(dialogue, node string) string { return dialogue + "." + node }

func selectedKey(dialogue, node, option string) string {
	return dialogue + "." + node + "." + option
}

// historyKey reads a played or selected condition written in dialogue dlg:
// [Dialogue.<id>.]<node>.stateData.played or
// [Dialogue.<id>.]<node>.options.<option>.selected. ok is false for a
// variable name.
func historyKey(name, dlg string) (key string, ok bool) {
	if !strings.Contains(name, ".") {
		return "", false
	}
	parts := strings.Split(name, ".")
	if parts[0] == "Dialogue" && len(parts) > 2 {
		dlg, parts = parts[1], parts[2:]
	}
	switch {
	case len(parts) == 3 && parts[1] == "stateData" && parts[2] == "played":
		return playedKey(dlg, parts[0]), true
	case len(parts) == 4 && parts[1] == "options" && parts[3] == "selected":
		return selectedKey(dlg, parts[0], parts[2]), true
	}
	return "", false
}

// world is what conditions are checked against: the variable values and the
// play history, seen from one dialogue.
type world struct {
	dialogue string
	vars     map[string]osf.Value
	hist     historyReader
}

// holds reports whether one requires entry is met.
func (w world) holds(a osf.Assign) bool {
	if key, ok := historyKey(a.Name, w.dialogue); ok {
		return a.Holds(osf.Value{Kind: osf.Bool, Bool: w.hist.has(key)})
	}
	v, ok := w.vars[a.Name]
	return ok && a.Holds(v)
}

// met reports whether a requires is met: every entry of one of its groups.
// A requires without conditions is met.
func (w world) met(first []osf.Assign, or [][]osf.Assign) bool {
	groups := osf.Groups(first, or)
	for _, g := range groups {
		all := true
		for _, a := range g {
			if !w.holds(a) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return len(groups) == 0
}

// allowed reports whether an option is offered.
func (w world) allowed(o *osf.OptionModel) bool { return w.met(o.Requires, o.RequiresOr) }

// next picks where a node or an option leads: the first branch whose
// requires are met, else the plain next. ok is false when it leads nowhere,
// which ends the play.
func (w world) next(branches []*osf.Branch, plain *osf.Str) (target string, ok bool) {
	for _, b := range branches {
		if b.To != nil && w.met(b.Requires, b.RequiresOr) {
			return b.To.Value, true
		}
	}
	if plain != nil {
		return plain.Value, true
	}
	return "", false
}

// backToOptions reports whether a next target is <node>.options, which offers
// that node's options again without entering it, and returns the node id.
func backToOptions(target string) (node string, ok bool) {
	if strings.HasPrefix(target, "Dialogue.") {
		return "", false
	}
	return strings.CutSuffix(target, ".options")
}

// trackedHistory lists the history keys some condition of the project reads,
// sorted. Only those can change how a play goes on.
func trackedHistory(p *osf.Project) []string {
	seen := map[string]bool{}
	add := func(dlg string, first []osf.Assign, or [][]osf.Assign) {
		for _, g := range osf.Groups(first, or) {
			for _, a := range g {
				if key, ok := historyKey(a.Name, dlg); ok {
					seen[key] = true
				}
			}
		}
	}
	for _, d := range p.Dialogues {
		for _, n := range d.Nodes {
			for _, b := range n.Branches {
				add(d.ID, b.Requires, b.RequiresOr)
			}
			for _, o := range n.Options {
				add(d.ID, o.Requires, o.RequiresOr)
				for _, b := range o.Branches {
					add(d.ID, b.Requires, b.RequiresOr)
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
