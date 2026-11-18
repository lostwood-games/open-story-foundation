package play

import "github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"

type Node struct {
	model    *osf.NodeModel
	dialogue *osf.Dialogue
	session  *Session
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

func (n *Node) Enter() []Change {
	n.played = true
	return n.session.apply(n.model.Set)
}

func (n *Node) Next(now world) (target string, ok bool) {
	now.dialogue = n.dialogue.ID
	return now.next(n.model.Branches, n.model.Next)
}

func (n *Node) Speaker() string {
	ix := n.session.ix
	if n.model.Title != nil {
		return ix.resolve(ix.text(n.textKey()+"title", n.model.Title), n.session.vars)
	}
	if n.model.Speaker != nil {
		id := n.model.Speaker.Value[len("Character."):]
		if c, ok := ix.characters[id]; ok {
			return ix.resolve(ix.text("Character."+id+".name", c.Name), n.session.vars)
		}
	}
	return ""
}

func (n *Node) Text() string {
	ix := n.session.ix
	return ix.resolve(ix.text(n.textKey()+"text", n.model.Text), n.session.vars)
}

func (n *Node) textKey() string {
	return "Dialogue." + n.dialogue.ID + ".nodes." + n.model.ID + ".content."
}

// key is the node's played-history key, as played conditions name it.
func (n *Node) key() string { return playedKey(n.dialogue.ID, n.model.ID) }
