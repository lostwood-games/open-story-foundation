package play

import "github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"

// Branch is a conditional next, as in `next: dark requires { lantern: false }`.
type Branch struct {
	to         string
	conditions []Condition
}

func newBranch(b *osf.Branch, dialogue string) *Branch {
	return &Branch{to: b.To.Value, conditions: conditionsOf(b.Requires, b.RequiresOr, dialogue)}
}

// To is where the branch leads: a node id, <node>.options or Dialogue.<id>.
func (b *Branch) To() string { return b.to }

// CanBeTaken reports whether the branch can be followed in the given state.
// If cannot will return reasons.
func (b *Branch) CanBeTaken(now world) (bool, []string) {
	var notMet []string
	for _, condition := range b.conditions {
		if met, reason := condition.Check(now); !met {
			notMet = append(notMet, reason)
		}
	}
	return len(notMet) == 0, notMet
}

// branchesOf turns the conditional nexts of a node or an option written in
// dialogue into Branch objects. One without a target is left out.
func branchesOf(branches []*osf.Branch, dialogue string) []*Branch {
	var out []*Branch
	for _, b := range branches {
		if b.To != nil {
			out = append(out, newBranch(b, dialogue))
		}
	}
	return out
}

// leadsTo is where a node or an option goes in the given state: the first
// branch that can be taken, else the plain next. ok is false when it leads
// nowhere, which ends the play.
func leadsTo(branches []*Branch, plain *osf.Str, now world) (target string, ok bool) {
	for _, b := range branches {
		if ok, _ := b.CanBeTaken(now); ok {
			return b.To(), true
		}
	}
	if plain != nil {
		return plain.Value, true
	}
	return "", false
}
