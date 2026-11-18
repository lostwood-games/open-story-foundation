package play

import (
	"reflect"
	"testing"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

func shopOption(t *testing.T, o *osf.OptionModel, v variables) *Option {
	t.Helper()
	shop := &osf.NodeModel{ID: "shop", Text: str("Buy?"), Options: []*osf.OptionModel{o}}
	d := &osf.Dialogue{ID: "market", Nodes: []*osf.NodeModel{shop, {ID: "thanks", Text: str("Thanks")}, {ID: "poor", Text: str("Too poor")}}}
	s, _ := start(t, project(v, d))
	return s.options[o]
}

// stateWith is a world with the given values and the history keys that are set.
func stateWith(vars map[string]osf.Value, hist ...string) world {
	h := history{}
	for _, k := range hist {
		h[k] = true
	}
	return world{dialogue: "market", vars: vars, hist: h}
}

func TestOptionIsAvailable(t *testing.T) {
	for _, tc := range []struct {
		name     string
		requires []osf.Assign
		or       [][]osf.Assign
		vars     map[string]osf.Value
		hist     []string
		want     bool
		reasons  []string
	}{
		{"no requires", nil, nil, nil, nil, true, nil},
		{"met", []osf.Assign{compare("gold", osf.OpGreaterEqual, 3)}, nil, map[string]osf.Value{"gold": number(3)}, nil, true, nil},
		{"not met", []osf.Assign{compare("gold", osf.OpGreaterEqual, 3)}, nil, map[string]osf.Value{"gold": number(2)}, nil,
			false, []string{"requires gold >= 3, but gold is 2"}},
		{"unknown variable", []osf.Assign{is("luck", boolean(true))}, nil, nil, nil,
			false, []string{"requires luck: true, but luck has no value"}},
		{"one entry of two not met", []osf.Assign{compare("gold", osf.OpGreaterEqual, 3), is("brave", boolean(true))},
			nil, map[string]osf.Value{"gold": number(5), "brave": boolean(false)}, nil,
			false, []string{"requires brave: true, but brave is false"}},
		{"two entries not met", []osf.Assign{compare("gold", osf.OpGreaterEqual, 3), is("brave", boolean(true))},
			nil, map[string]osf.Value{"gold": number(1), "brave": boolean(false)}, nil,
			false, []string{"requires gold >= 3, but gold is 1", "requires brave: true, but brave is false"}},
		{"second alternative", []osf.Assign{compare("gold", osf.OpGreaterEqual, 3)}, [][]osf.Assign{{is("brave", boolean(true))}},
			map[string]osf.Value{"gold": number(0), "brave": boolean(true)}, nil, true, nil},
		{"no alternative", []osf.Assign{compare("gold", osf.OpGreaterEqual, 3)}, [][]osf.Assign{{is("brave", boolean(true))}},
			map[string]osf.Value{"gold": number(0), "brave": boolean(false)}, nil,
			false, []string{"requires gold >= 3, but gold is 0 or requires brave: true, but brave is false"}},
		{"played", []osf.Assign{is("thanks.stateData.played", boolean(true))}, nil, nil, []string{"market.thanks"}, true, nil},
		{"not played", []osf.Assign{is("thanks.stateData.played", boolean(true))}, nil, nil, nil,
			false, []string{"requires thanks.stateData.played: true, but it is false"}},
		{"selected", []osf.Assign{is("shop.options.buy.selected", boolean(false))}, nil, nil, []string{"market.shop.buy"},
			false, []string{"requires shop.options.buy.selected: false, but it is true"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := shopOption(t, &osf.OptionModel{ID: "buy", Text: str("Buy"), Requires: tc.requires, RequiresOr: tc.or}, variables{})
			got, reasons := o.IsAvailable(stateWith(tc.vars, tc.hist...))
			if got != tc.want || !reflect.DeepEqual(reasons, tc.reasons) {
				t.Errorf("IsAvailable = %v, %q; want %v, %q", got, reasons, tc.want, tc.reasons)
			}
		})
	}
}

// A relative name like thanks.stateData.played means the option's own
// dialogue, whichever dialogue the passed state is seen from.
func TestOptionIsAvailableReadsItsOwnDialogue(t *testing.T) {
	o := shopOption(t, &osf.OptionModel{ID: "buy", Text: str("Buy"), Requires: []osf.Assign{is("thanks.stateData.played", boolean(true))}}, variables{})
	now := stateWith(nil, "market.thanks")
	now.dialogue = "elsewhere"
	if ok, reasons := o.IsAvailable(now); !ok {
		t.Errorf("IsAvailable = false (%v), want true: market.thanks was played", reasons)
	}
}

// The state passed in decides the answer, not the session: the same option
// is available in one state and not in another.
func TestOptionIsAvailableDependsOnlyOnTheState(t *testing.T) {
	o := shopOption(t, &osf.OptionModel{ID: "buy", Text: str("Buy"), Requires: []osf.Assign{compare("gold", osf.OpGreaterEqual, 3)}}, variables{gold: 0})
	rich, _ := o.IsAvailable(stateWith(map[string]osf.Value{"gold": number(9)}))
	poor, _ := o.IsAvailable(stateWith(map[string]osf.Value{"gold": number(1)}))
	if !rich || poor {
		t.Errorf("IsAvailable(rich) = %v, IsAvailable(poor) = %v, want true, false", rich, poor)
	}
}

func TestOptionNext(t *testing.T) {
	model := &osf.OptionModel{ID: "buy", Text: str("Buy"), Next: str("thanks"),
		Branches: []*osf.Branch{branch("poor", []osf.Assign{compare("gold", osf.OpLess, 3)})}}
	o := shopOption(t, model, variables{})
	for _, tc := range []struct {
		gold int64
		want string
	}{{1, "poor"}, {3, "thanks"}} {
		got, ok := o.Next(stateWith(map[string]osf.Value{"gold": number(tc.gold)}))
		if !ok || got != tc.want {
			t.Errorf("gold %d: Next = %q, %v; want %q, true", tc.gold, got, ok, tc.want)
		}
	}
}

func TestOptionNextNowhere(t *testing.T) {
	o := shopOption(t, &osf.OptionModel{ID: "buy", Text: str("Buy")}, variables{})
	if got, ok := o.Next(stateWith(nil)); ok {
		t.Errorf("Next = %q, true; want no target", got)
	}
}

func TestOptionSelect(t *testing.T) {
	model := &osf.OptionModel{ID: "buy", Text: str("Buy"), Set: []osf.Assign{add("gold", -5), is("brave", boolean(true))}}
	o := shopOption(t, model, variables{gold: 3})
	if o.IsSelected() {
		t.Fatal("IsSelected = true before Select")
	}
	changes := o.Select()
	if !o.IsSelected() {
		t.Error("IsSelected = false after Select")
	}
	// gold is unsigned, so 3 - 5 stops at 0.
	want := []Change{{Name: "gold", Value: int64(0)}, {Name: "brave", Value: true}}
	if !reflect.DeepEqual(changes, want) {
		t.Errorf("Select changes = %v, want %v", changes, want)
	}
}

// A selected option answers selected conditions of other options.
func TestOptionSelectedAnswersConditions(t *testing.T) {
	buy := &osf.OptionModel{ID: "buy", Text: str("Buy")}
	again := &osf.OptionModel{ID: "again", Text: str("Again"), Requires: []osf.Assign{is("shop.options.buy.selected", boolean(true))}}
	shop := &osf.NodeModel{ID: "shop", Text: str("Buy?"), Options: []*osf.OptionModel{buy, again}}
	s, err := New(project(variables{}, &osf.Dialogue{ID: "market", Nodes: []*osf.NodeModel{shop}}), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.options[again].IsAvailable(s.world()); ok {
		t.Fatal("again is available before buy was selected")
	}
	s.options[buy].Select()
	if ok, reasons := s.options[again].IsAvailable(s.world()); !ok {
		t.Errorf("again is not available after buy was selected: %v", reasons)
	}
}

func TestOptionText(t *testing.T) {
	o := shopOption(t, &osf.OptionModel{ID: "buy", Text: str("Pay with {{ Variable.gold }} gold")}, variables{gold: 7})
	if got, want := o.Text(), "Pay with 7 gold"; got != want {
		t.Errorf("Text = %q, want %q", got, want)
	}
}
