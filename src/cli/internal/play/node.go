package play

import "github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"

type Node struct {
	model    *osf.NodeModel
	dialogue *osf.Dialogue
	options  []*Option
	played   bool
}

func (n *Node) ID() string { return n.model.ID }

func (n *Node) Options() []*Option { return n.options }

func (n *Node) HasOptions() bool { return len(n.options) > 0 }

func (n *Node) AvailableOptions(now world) []*Option {
	var out []*Option
	for _, o := range n.options {
		if ok, _ := o.IsAvailable(now); ok {
			out = append(out, o)
		}
	}
	return out
}

// (<node>.stateData.played).
func (n *Node) IsPlayed() bool { return n.played }

func (n *Node) Enter(now world) []Change {
	n.played = true
	return now.apply(n.model.Set)
}

func (n *Node) Next(now world) (target string, ok bool) {
	now.dialogue = n.dialogue.ID
	return now.next(n.model.Branches, n.model.Next)
}

func (n *Node) Speaker(now world) string {
	if n.model.Title != nil {
		return now.text(n.textKey()+"title", n.model.Title)
	}
	if n.model.Speaker != nil {
		id := n.model.Speaker.Value[len("Character."):]
		if c, ok := now.ix.characters[id]; ok {
			return now.text("Character."+id+".name", c.Name)
		}
	}
	return ""
}

func (n *Node) Text(now world) string {
	return now.text(n.textKey()+"text", n.model.Text)
}

func (n *Node) textKey() string {
	return "Dialogue." + n.dialogue.ID + ".nodes." + n.model.ID + ".content."
}

// key is the node's played-history key, as played conditions name it.
func (n *Node) key() string { return playedKey(n.dialogue.ID, n.model.ID) }
