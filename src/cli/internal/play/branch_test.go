package play

import (
	"reflect"
	"testing"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

func TestBranchCanBeTaken(t *testing.T) {
	for _, tc := range []struct {
		name    string
		branch  *osf.Branch
		vars    map[string]osf.Value
		hist    []string
		want    bool
		reasons []string
	}{
		{"no requires", branch("inside", nil), nil, nil, true, nil},
		{"met", branch("inside", []osf.Assign{compare("gold", osf.OpGreaterEqual, 3)}), map[string]osf.Value{"gold": number(3)}, nil, true, nil},
		{"not met", branch("inside", []osf.Assign{compare("gold", osf.OpGreaterEqual, 3)}), map[string]osf.Value{"gold": number(1)}, nil,
			false, []string{"requires gold >= 3, but gold is 1"}},
		{"two not met", branch("inside", []osf.Assign{compare("gold", osf.OpGreaterEqual, 3), is("brave", boolean(true))}),
			map[string]osf.Value{"gold": number(1), "brave": boolean(false)}, nil,
			false, []string{"requires gold >= 3, but gold is 1", "requires brave: true, but brave is false"}},
		{"or", branch("inside", []osf.Assign{compare("gold", osf.OpGreaterEqual, 3)}, []osf.Assign{is("brave", boolean(true))}),
			map[string]osf.Value{"gold": number(0), "brave": boolean(true)}, nil, true, nil},
		{"played in its own dialogue", branch("inside", []osf.Assign{is("intro.stateData.played", boolean(true))}), nil, []string{"market.intro"}, true, nil},
		{"played elsewhere", branch("inside", []osf.Assign{is("Dialogue.cellar.door.stateData.played", boolean(true))}), nil, []string{"market.door"},
			false, []string{"requires Dialogue.cellar.door.stateData.played: true, but it is false"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, all := market("inside")
			b := newBranch(tc.branch, d, all)
			if b.To().Node != d.Node("inside") {
				t.Errorf("To = %v, want node inside", b.To())
			}
			got, reasons := b.CanBeTaken(stateWith(tc.vars, tc.hist...))
			if got != tc.want || !reflect.DeepEqual(reasons, tc.reasons) {
				t.Errorf("CanBeTaken = %v, %q; want %v, %q", got, reasons, tc.want, tc.reasons)
			}
		})
	}
}

// leadsTo takes the first branch that can be taken, else the plain next.
func TestLeadsTo(t *testing.T) {
	d, all := market("rich", "ok", "poor")
	branches := branchesOf([]*osf.Branch{
		branch("rich", []osf.Assign{compare("gold", osf.OpGreaterEqual, 10)}),
		branch("ok", []osf.Assign{compare("gold", osf.OpGreaterEqual, 3)}),
		{Requires: []osf.Assign{is("brave", boolean(true))}}, // no target: left out
	}, d, all)
	if len(branches) != 2 {
		t.Fatalf("branchesOf kept %d branches, want 2", len(branches))
	}
	for _, tc := range []struct {
		gold  int64
		plain *Target
		want  string
		ok    bool
	}{
		{20, &Target{Node: d.Node("poor")}, "rich", true},
		{5, &Target{Node: d.Node("poor")}, "ok", true},
		{1, &Target{Node: d.Node("poor")}, "poor", true},
		{1, nil, "", false},
	} {
		to, ok := leadsTo(branches, tc.plain, stateWith(map[string]osf.Value{"gold": number(tc.gold)}))
		got := nodeID(to)
		if got != tc.want || ok != tc.ok {
			t.Errorf("gold %d: leadsTo = %q, %v; want %q, %v", tc.gold, got, ok, tc.want, tc.ok)
		}
	}
}

// market is a Dialogue market with nodes of the given ids, and the
// dialogues by id as targets are resolved against.
func market(ids ...string) (*Dialogue, map[string]*Dialogue) {
	d := &Dialogue{model: &osf.DialogueModel{ID: "market"}, byID: map[string]*Node{}}
	for _, id := range ids {
		n := &Node{model: &osf.NodeModel{ID: id}, dialogue: d}
		d.nodes = append(d.nodes, n)
		d.byID[id] = n
	}
	return d, map[string]*Dialogue{"market": d}
}

// nodeID is the id of the node a target leads to, "" for none.
func nodeID(t *Target) string {
	if t == nil || t.Node == nil {
		return ""
	}
	return t.Node.ID()
}
