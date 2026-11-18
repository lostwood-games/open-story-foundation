package play

import (
	"testing"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

func TestDialogueStart(t *testing.T) {
	d, _ := market("a", "b")
	if got := nodeID(&Target{Node: d.Start()}); got != "a" {
		t.Errorf("Start = %q, want the first node a", got)
	}
	d.model.StartNode = str("b")
	if got := nodeID(&Target{Node: d.Start()}); got != "b" {
		t.Errorf("Start = %q, want the start node b", got)
	}
	d.model.StartNode = str("missing")
	if got := nodeID(&Target{Node: d.Start()}); got != "a" {
		t.Errorf("Start = %q, want the first node a when startNode names nothing", got)
	}
	empty, _ := market()
	if empty.Start() != nil {
		t.Error("Start of a dialogue without nodes is not nil")
	}
}

func TestDialogueNode(t *testing.T) {
	d, _ := market("a", "b")
	if d.Node("b") == nil || d.Node("b").ID() != "b" || d.Node("c") != nil {
		t.Errorf("Node(b) = %v, Node(c) = %v; want b, nil", d.Node("b"), d.Node("c"))
	}
	if len(d.Nodes()) != 2 {
		t.Errorf("Nodes has %d nodes, want 2", len(d.Nodes()))
	}
}

func TestTargetIn(t *testing.T) {
	d, all := market("hub", "end")
	cellar := &Dialogue{model: &osf.DialogueModel{ID: "cellar"}, byID: map[string]*Node{}}
	door := &Node{model: &osf.NodeModel{ID: "door"}, dialogue: cellar}
	cellar.nodes, cellar.byID["door"] = []*Node{door}, door
	all["cellar"] = cellar
	all["empty"] = &Dialogue{model: &osf.DialogueModel{ID: "empty"}, byID: map[string]*Node{}}

	for _, tc := range []struct {
		next string
		node string
		back bool
		err  string
	}{
		{next: "end", node: "end"},
		{next: "hub.options", node: "hub", back: true},
		{next: "Dialogue.cellar", node: "door"},
		{next: "nowhere", err: `next: node "nowhere" does not exist in Dialogue market`},
		{next: "nowhere.options", err: `next: node "nowhere" does not exist in Dialogue market`},
		{next: "Dialogue.attic", err: "next: Dialogue.attic does not exist"},
		{next: "Dialogue.empty", err: "next: Dialogue empty has no nodes"},
	} {
		got := targetIn(d, tc.next, all)
		gotErr := ""
		if got.Err != nil {
			gotErr = got.Err.Error()
		}
		if nodeID(got) != tc.node || got.Back != tc.back || gotErr != tc.err {
			t.Errorf("%s: target = %q back %v err %q; want %q back %v err %q", tc.next, nodeID(got), got.Back, gotErr, tc.node, tc.back, tc.err)
		}
	}
}
