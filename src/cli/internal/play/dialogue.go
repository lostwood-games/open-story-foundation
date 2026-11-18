package play

import (
	"fmt"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// Dialogue is a dialogue during a play. It holds its nodes.
type Dialogue struct {
	model *osf.DialogueModel
	nodes []*Node
	byID  map[string]*Node
}

func (d *Dialogue) ID() string { return d.model.ID }

// Nodes lists the nodes in the order they are written.
func (d *Dialogue) Nodes() []*Node { return d.nodes }

// Node is the node with the given id, nil when there is none.
func (d *Dialogue) Node(id string) *Node { return d.byID[id] }

// Start is the node the dialogue starts at: its start node when that names a
// node, else the first node; nil for a dialogue without nodes.
func (d *Dialogue) Start() *Node {
	if d.model.StartNode != nil {
		if n := d.Node(d.model.StartNode.Value); n != nil {
			return n
		}
	}
	if len(d.nodes) > 0 {
		return d.nodes[0]
	}
	return nil
}

// Target is where a next leads: Node is entered, or with Back (next:
// <node>.options) its options are offered again without entering it. Err is
// set when the next names nothing; validation reports that before a play.
type Target struct {
	Node *Node
	Back bool
	Err  error
}

// targetIn resolves a next written in dialogue d: a node id, <node>.options or
// Dialogue.<id>, which leads to that dialogue's start node.
func targetIn(d *Dialogue, value string, dialogues map[string]*Dialogue) *Target {
	if id, ok := strings.CutPrefix(value, "Dialogue."); ok {
		nd, ok := dialogues[id]
		if !ok {
			return &Target{Err: fmt.Errorf("next: %s does not exist", value)}
		}
		if nd.Start() == nil {
			return &Target{Err: fmt.Errorf("next: Dialogue %s has no nodes", nd.ID())}
		}
		return &Target{Node: nd.Start()}
	}
	id, back := strings.CutSuffix(value, ".options")
	n := d.Node(id)
	if n == nil {
		return &Target{Err: fmt.Errorf("next: node %q does not exist in Dialogue %s", id, d.ID())}
	}
	return &Target{Node: n, Back: back}
}
