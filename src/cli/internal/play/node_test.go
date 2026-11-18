package play

import (
	"reflect"
	"testing"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// These tests use one Node on its own, like the Option tests: the state it is
// checked against is built by hand and passed in.

// gateNode is node `gate` of Dialogue `market`, in a started play that
// stops at the start node `intro`, so gate itself is not played yet.
func gateNode(t *testing.T, gate *osf.NodeModel, v variables) (*Session, *Node) {
	t.Helper()
	d := &osf.Dialogue{ID: "market", Nodes: []*osf.NodeModel{
		{ID: "intro", Text: str("Hello")}, gate,
		{ID: "inside", Text: str("Inside")}, {ID: "outside", Text: str("Outside")},
	}}
	p := project(v, d)
	p.Characters = []*osf.Character{{ID: "guard", Name: str("Guard")}}
	s, _ := start(t, p)
	return s, s.nodes[gate]
}

func TestNodeEnter(t *testing.T) {
	_, n := gateNode(t, &osf.NodeModel{ID: "gate", Text: str("A gate"), Set: []osf.Assign{add("gold", 2), is("brave", boolean(true))}}, variables{})
	if n.IsPlayed() {
		t.Fatal("IsPlayed = true before Enter")
	}
	vars := map[string]osf.Value{"gold": number(1), "brave": boolean(false)}
	changes := n.Enter(stateWith(vars))
	if !n.IsPlayed() {
		t.Error("IsPlayed = false after Enter")
	}
	want := []Change{{Name: "gold", Value: int64(3)}, {Name: "brave", Value: true}}
	if !reflect.DeepEqual(changes, want) {
		t.Errorf("Enter changes = %v, want %v", changes, want)
	}
	if vars["gold"] != number(3) {
		t.Errorf("gold after Enter = %v, want 3", vars["gold"])
	}
}

func TestNodeNext(t *testing.T) {
	model := &osf.NodeModel{ID: "gate", Text: str("A gate"), Next: str("outside"),
		Branches: []*osf.Branch{branch("inside", []osf.Assign{compare("gold", osf.OpGreaterEqual, 3)})}}
	_, n := gateNode(t, model, variables{})
	for _, tc := range []struct {
		gold int64
		want string
	}{{5, "inside"}, {1, "outside"}} {
		got, ok := n.Next(stateWith(map[string]osf.Value{"gold": number(tc.gold)}))
		if !ok || got != tc.want {
			t.Errorf("gold %d: Next = %q, %v; want %q, true", tc.gold, got, ok, tc.want)
		}
	}
}

func TestNodeNextNowhere(t *testing.T) {
	_, n := gateNode(t, &osf.NodeModel{ID: "gate", Text: str("A gate")}, variables{})
	if got, ok := n.Next(stateWith(nil)); ok {
		t.Errorf("Next = %q, true; want no target", got)
	}
}

// A relative name like intro.stateData.played in a branch means the node's
// own dialogue: market.intro.
func TestNodeNextReadsItsOwnDialogue(t *testing.T) {
	model := &osf.NodeModel{ID: "gate", Text: str("A gate"), Next: str("outside"),
		Branches: []*osf.Branch{branch("inside", []osf.Assign{is("intro.stateData.played", boolean(true))})}}
	_, n := gateNode(t, model, variables{})
	now := stateWith(nil, "market.intro")
	if got, _ := n.Next(now); got != "inside" {
		t.Errorf("Next = %q, want inside: market.intro was played", got)
	}
}

func TestNodeOptions(t *testing.T) {
	model := &osf.NodeModel{ID: "gate", Text: str("A gate"), Options: []*osf.OptionModel{
		{ID: "pay", Text: str("Pay"), Requires: []osf.Assign{compare("gold", osf.OpGreaterEqual, 3)}},
		{ID: "leave", Text: str("Leave")},
	}}
	_, n := gateNode(t, model, variables{})
	if !n.HasOptions() || len(n.Options()) != 2 {
		t.Fatalf("HasOptions = %v, %d options; want true, 2", n.HasOptions(), len(n.Options()))
	}
	ids := func(opts []*Option) []string {
		var out []string
		for _, o := range opts {
			out = append(out, o.ID())
		}
		return out
	}
	rich := n.AvailableOptions(stateWith(map[string]osf.Value{"gold": number(5)}))
	poor := n.AvailableOptions(stateWith(map[string]osf.Value{"gold": number(1)}))
	if got := ids(rich); !reflect.DeepEqual(got, []string{"pay", "leave"}) {
		t.Errorf("AvailableOptions(gold 5) = %v, want [pay leave]", got)
	}
	if got := ids(poor); !reflect.DeepEqual(got, []string{"leave"}) {
		t.Errorf("AvailableOptions(gold 1) = %v, want [leave]", got)
	}
	for _, o := range n.Options() {
		if o.node != n {
			t.Errorf("option %s does not point back at its node", o.ID())
		}
	}
}

func TestNodeWithoutOptions(t *testing.T) {
	_, n := gateNode(t, &osf.NodeModel{ID: "gate", Text: str("A gate")}, variables{})
	if n.HasOptions() || len(n.AvailableOptions(stateWith(nil))) != 0 {
		t.Error("a node without options says it has some")
	}
}

// An entered node answers played conditions of options.
func TestNodePlayedAnswersConditions(t *testing.T) {
	ask := &osf.OptionModel{ID: "ask", Text: str("Ask"), Requires: []osf.Assign{is("gate.stateData.played", boolean(true))}}
	model := &osf.NodeModel{ID: "gate", Text: str("A gate"), Options: []*osf.OptionModel{ask}}
	s, n := gateNode(t, model, variables{})
	if ok, _ := s.options[ask].IsAvailable(s.world()); ok {
		t.Fatal("ask is available before gate was played")
	}
	n.Enter(s.world())
	if ok, reasons := s.options[ask].IsAvailable(s.world()); !ok {
		t.Errorf("ask is not available after gate was played: %v", reasons)
	}
}

func TestNodeSpeaker(t *testing.T) {
	for _, tc := range []struct {
		name string
		node *osf.NodeModel
		want string
	}{
		{"narration", &osf.NodeModel{ID: "gate", Text: str("A gate")}, ""},
		{"character", &osf.NodeModel{ID: "gate", Text: str("Halt"), Speaker: str("Character.guard")}, "Guard"},
		{"title wins", &osf.NodeModel{ID: "gate", Text: str("Halt"), Speaker: str("Character.guard"), Title: str("Old guard")}, "Old guard"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, n := gateNode(t, tc.node, variables{})
			if got := n.Speaker(stateWith(nil)); got != tc.want {
				t.Errorf("Speaker = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNodeText(t *testing.T) {
	_, n := gateNode(t, &osf.NodeModel{ID: "gate", Text: str("The toll is {{ Variable.gold }} gold")}, variables{})
	if got, want := n.Text(stateWith(map[string]osf.Value{"gold": number(4)})), "The toll is 4 gold"; got != want {
		t.Errorf("Text = %q, want %q", got, want)
	}
}
