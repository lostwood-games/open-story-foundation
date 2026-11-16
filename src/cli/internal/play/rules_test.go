package play

import (
	"reflect"
	"testing"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// These tests build the project as model values, so they test the play rules
// alone, without the parser.

func str(s string) *osf.Str { return &osf.Str{Value: s} }

func boolean(b bool) osf.Value { return osf.Value{Kind: osf.Bool, Bool: b} }

func number(n int64) osf.Value { return osf.Value{Kind: osf.Int, Int: n} }

// is is the condition or assignment `name: value`.
func is(name string, v osf.Value) osf.Assign { return osf.Assign{Name: name, Value: v} }

// compare is the condition `name op n`; add is the assignment `name += n`.
func compare(name string, op osf.Op, n int64) osf.Assign {
	return osf.Assign{Name: name, Op: op, Value: number(n)}
}

func add(name string, n int64) osf.Assign {
	return osf.Assign{Name: name, Op: osf.OpAdd, Value: number(n)}
}

func branch(to string, requires []osf.Assign, or ...[]osf.Assign) *osf.Branch {
	return &osf.Branch{To: str(to), Requires: requires, RequiresOr: or}
}

type variables struct {
	gold  int64
	mood  int64
	brave bool
}

// project is a project with the variables gold, mood and brave and the given
// dialogues; play starts at the first one.
func project(v variables, dialogues ...*osf.Dialogue) *osf.Project {
	return &osf.Project{
		Variables: []*osf.Variable{
			{ID: "gold", Type: osf.UnsignedNumber, Default: number(v.gold)},
			{ID: "mood", Type: osf.Number, Default: number(v.mood)},
			{ID: "brave", Type: osf.Boolean, Default: boolean(v.brave)},
		},
		Dialogues: dialogues,
	}
}

func start(t *testing.T, p *osf.Project) (*Session, State) {
	t.Helper()
	s, err := New(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.Start()
	if err != nil {
		t.Fatal(err)
	}
	return s, st
}

func choose(t *testing.T, s *Session, option string) State {
	t.Helper()
	st, err := s.ChooseID(option)
	if err != nil {
		t.Fatalf("choose %s: %v", option, err)
	}
	return st
}

func optionIDs(st State) []string {
	var out []string
	for _, o := range st.Options {
		out = append(out, o.ID)
	}
	return out
}

// A conditional next follows the first branch whose requires are met, then
// the plain next.
func TestConditionalNext(t *testing.T) {
	gate := &osf.Dialogue{ID: "d", Nodes: []*osf.Node{
		{
			ID:   "gate",
			Text: str("Gate"),
			Branches: []*osf.Branch{
				branch("rich", []osf.Assign{compare("gold", osf.OpGreaterEqual, 10)}),
				branch("brave", []osf.Assign{is("brave", boolean(true))}, []osf.Assign{is("mood", number(5))}),
			},
			Next: str("poor"),
		},
		{ID: "rich", Text: str("Rich")},
		{ID: "brave", Text: str("Brave")},
		{ID: "poor", Text: str("Poor")},
	}}
	tests := []struct {
		name string
		vars variables
		want string
	}{
		{"first branch", variables{gold: 10}, "rich"},
		{"first branch wins over the second", variables{gold: 10, brave: true}, "rich"},
		{"second branch", variables{brave: true}, "brave"},
		{"second branch, other group", variables{mood: 5}, "brave"},
		{"plain next", variables{gold: 9, mood: 4}, "poor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, st := start(t, project(tt.vars, gate))
			if st.Node != tt.want || !st.Finished {
				t.Errorf("reached %s (finished %v), want %s", st.Node, st.Finished, tt.want)
			}
		})
	}
}

// On an option the branch is checked after the option's set, and an option
// whose branches are not met and that has no plain next ends the play.
func TestConditionalNextOnOption(t *testing.T) {
	d := &osf.Dialogue{ID: "d", Nodes: []*osf.Node{
		{
			ID:   "shop",
			Text: str("Shop"),
			Options: []*osf.Option{
				{
					ID:       "work",
					Text:     str("Work"),
					Set:      []osf.Assign{add("gold", 10)},
					Branches: []*osf.Branch{branch("rich", []osf.Assign{compare("gold", osf.OpGreaterEqual, 13)})},
					Next:     str("shop"),
				},
				{
					ID:       "stop",
					Text:     str("Stop"),
					Branches: []*osf.Branch{branch("rich", []osf.Assign{compare("gold", osf.OpGreaterEqual, 100)})},
				},
			},
		},
		{ID: "rich", Text: str("Rich")},
	}}
	tests := []struct {
		name      string
		gold      int64
		option    string
		wantNode  string
		wantEnded bool
		wantLines int
	}{
		{"branch met after set", 3, "work", "rich", true, 1},
		{"branch not met, plain next", 0, "work", "shop", false, 1},
		{"no branch met, no plain next", 3, "stop", "shop", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := start(t, project(variables{gold: tt.gold}, d))
			st := choose(t, s, tt.option)
			if st.Node != tt.wantNode || st.Finished != tt.wantEnded || len(st.Lines) != tt.wantLines {
				t.Errorf("node %s, finished %v, %d lines; want %s, %v, %d", st.Node, st.Finished, len(st.Lines), tt.wantNode, tt.wantEnded, tt.wantLines)
			}
		})
	}
}

// requires can check what was played: a node entered, an option chosen, in
// this dialogue or another.
func TestPlayedAndSelected(t *testing.T) {
	played := func(name string, want bool) []osf.Assign { return []osf.Assign{is(name, boolean(want))} }
	d := &osf.Dialogue{ID: "d", Nodes: []*osf.Node{
		{
			ID:   "hub",
			Text: str("Hub"),
			Options: []*osf.Option{
				{ID: "rumours", Text: str("Rumours"), Next: str("rumours")},
				{ID: "room", Text: str("Room"), Requires: played("rumours.stateData.played", true), Next: str("hub")},
				{ID: "once", Text: str("Once"), Requires: played("hub.options.once.selected", false), Next: str("hub")},
				{ID: "leave", Text: str("Leave"), Requires: played("Dialogue.e.far.stateData.played", false), Next: str("Dialogue.e")},
				{ID: "back", Text: str("Back"), Requires: played("Dialogue.e.far.options.home.selected", true)},
			},
		},
		{ID: "rumours", Text: str("They say..."), Next: str("hub")},
	}}
	e := &osf.Dialogue{ID: "e", Nodes: []*osf.Node{
		{ID: "far", Text: str("Far"), Options: []*osf.Option{{ID: "home", Text: str("Home"), Next: str("Dialogue.d")}}},
	}}
	s, st := start(t, project(variables{}, d, e))

	steps := []struct {
		choose []string // options chosen in order; none for the start
		want   []string // options offered afterwards
	}{
		{nil, []string{"rumours", "once", "leave"}},
		{[]string{"once"}, []string{"rumours", "leave"}},
		{[]string{"rumours"}, []string{"rumours", "room", "leave"}},
		{[]string{"leave", "home"}, []string{"rumours", "room", "back"}},
	}
	for _, step := range steps {
		for _, o := range step.choose {
			st = choose(t, s, o)
		}
		if got := optionIDs(st); !reflect.DeepEqual(got, step.want) {
			t.Fatalf("after %v: options %v, want %v", step.choose, got, step.want)
		}
	}

	// A new play forgets the history.
	st, _ = s.Restart()
	if got, want := optionIDs(st), steps[0].want; !reflect.DeepEqual(got, want) {
		t.Errorf("after restart: options %v, want %v", got, want)
	}
}

// next: <node>.options offers the node's options again without its line or
// its set; a plain next to the node enters it again.
func TestBackToOptions(t *testing.T) {
	d := &osf.Dialogue{ID: "d", Nodes: []*osf.Node{
		{
			ID:   "hub",
			Text: str("Welcome"),
			Set:  []osf.Assign{add("gold", 1)},
			Options: []*osf.Option{
				{ID: "ask", Text: str("Ask"), Next: str("answer")},
				{ID: "again", Text: str("Again"), Next: str("hub")},
				{ID: "leave", Text: str("Leave")},
			},
		},
		{ID: "answer", Text: str("An answer."), Next: str("hub.options")},
	}}
	tests := []struct {
		name      string
		option    string
		wantLines []string
		wantGold  int64
		wantPath  []Visit
	}{
		{
			name:      "back to the options",
			option:    "ask",
			wantLines: []string{"An answer."},
			wantGold:  4, // 3 + 1 from entering hub once
			wantPath:  []Visit{{"d", "hub", "ask"}, {"d", "answer", ""}, {"d", "hub", ""}},
		},
		{
			name:      "plain next enters the node",
			option:    "again",
			wantLines: []string{"Welcome"},
			wantGold:  5,
			wantPath:  []Visit{{"d", "hub", "again"}, {"d", "hub", ""}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := start(t, project(variables{gold: 3}, d))
			st := choose(t, s, tt.option)
			if got := texts(st.Lines); !reflect.DeepEqual(got, tt.wantLines) {
				t.Errorf("lines %v, want %v", got, tt.wantLines)
			}
			if got := value(st, "gold"); got != tt.wantGold {
				t.Errorf("gold %v, want %d", got, tt.wantGold)
			}
			if !reflect.DeepEqual(st.Path, tt.wantPath) {
				t.Errorf("path %+v, want %+v", st.Path, tt.wantPath)
			}
			if st.Node != "hub" || st.Finished || !reflect.DeepEqual(optionIDs(st), []string{"ask", "again", "leave"}) {
				t.Errorf("state: node %s, finished %v, options %v", st.Node, st.Finished, optionIDs(st))
			}
		})
	}
}

// Coverage follows branches, history and <node>.options like a play does.
func TestCoverageNewFeatures(t *testing.T) {
	d := &osf.Dialogue{ID: "d", Nodes: []*osf.Node{
		{
			ID:   "hub",
			Text: str("Hub"),
			Options: []*osf.Option{
				{
					ID:       "ask",
					Text:     str("Ask"),
					Requires: []osf.Assign{is("hub.options.ask.selected", boolean(false))},
					Next:     str("answer"),
				},
				{
					ID:       "secret",
					Text:     str("Secret"),
					Requires: []osf.Assign{is("answer.stateData.played", boolean(true))},
					Branches: []*osf.Branch{branch("vault", []osf.Assign{compare("gold", osf.OpGreaterEqual, 100)})},
					Next:     str("hall"),
				},
				{
					ID:       "never",
					Text:     str("Never"),
					Requires: []osf.Assign{is("vault.stateData.played", boolean(true))},
				},
				{ID: "leave", Text: str("Leave")},
			},
		},
		{ID: "answer", Text: str("An answer."), Next: str("hub.options")},
		{ID: "hall", Text: str("Hall")},
		{ID: "vault", Text: str("Vault")},
	}}
	r, err := Coverage(project(variables{gold: 3}, d), "")
	if err != nil {
		t.Fatal(err)
	}
	wantFindings := []string{"unreachable option: d.hub.never (requires vault.stateData.played = true)"}
	if got := findings(r); !reflect.DeepEqual(got, wantFindings) {
		t.Errorf("findings %v, want %v", got, wantFindings)
	}
	want := DialogueCoverage{ID: "d", Reached: 3, Total: 4, Unreached: []string{"vault"}}
	if got := r.Dialogues[0]; !reflect.DeepEqual(got, want) {
		t.Errorf("coverage %+v, want %+v", got, want)
	}
}
