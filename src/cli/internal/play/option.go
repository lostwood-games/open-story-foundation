package play

import "github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"

type Option struct {
	model      *osf.OptionModel
	node       *Node
	dialogue   *Dialogue
	conditions []Condition // its requires; all must be met
	branches   []*Branch   // conditional nexts, tried in order before the plain next
	next       *Target     // the plain next; nil when there is none
	selected   bool
}

func (o *Option) ID() string { return o.model.ID }

// will also return reasons
func (o *Option) IsAvailable(now world) (bool, []string) {
	if len(o.conditions) == 0 {
		return true, nil
	}
	var notMet []string
	for _, condition := range o.conditions {
		if met, reason := condition.Check(now); !met {
			notMet = append(notMet, reason)
		}
	}
	return len(notMet) == 0, notMet
}

// (<node>.options.<option>.selected).
func (o *Option) IsSelected() bool { return o.selected }

func (o *Option) Select(now world) []Change {
	o.selected = true
	return now.apply(o.model.Set)
}

func (o *Option) Next(now world) (target *Target, ok bool) {
	return leadsTo(o.branches, o.next, now)
}

// Text is the option's text in the play's locale, with references resolved.
func (o *Option) Text(now world) string {
	key := "Dialogue." + o.dialogue.ID() + ".nodes." + o.node.ID() + ".options." + o.model.ID + ".content.text"
	return now.text(key, o.model.Text)
}

// key is the option's played-history key, as selected conditions name it.
func (o *Option) key() string { return selectedKey(o.dialogue.ID(), o.node.ID(), o.model.ID) }
