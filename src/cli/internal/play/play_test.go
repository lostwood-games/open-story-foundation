package play

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/lang"
	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// Accepted so that `go test ./... -update` (the golden flag of package main)
// does not fail in this package.
var _ = flag.Bool("update", false, "accepted for package main's golden files")

const base = `
Versions { language = "1" story = "v1" dialogue = "v1" i18n = "v1" }
Story s { name = "S" description = "A story" start = Dialogue.d }
Character hero { name = "Hero" description = "Brave" }
Variable brave: Boolean = false
Variable gold: UnsignedNumber = 3
Variable mood: Number = 1
`

func load(t *testing.T, files map[string]string) *osf.Project {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, diags := lang.LoadDir(dir)
	if len(diags) > 0 {
		t.Fatalf("load: %v", diags)
	}
	return p
}

func one(t *testing.T, src string) *osf.Project {
	return load(t, map[string]string{"s.osf": base + src})
}

// texts flattens the lines of a state as "Speaker: text" for easy comparison.
func texts(lines []Line) []string {
	var out []string
	for _, l := range lines {
		if l.Speaker != "" {
			out = append(out, l.Speaker+": "+l.Text)
		} else {
			out = append(out, l.Text)
		}
	}
	return out
}

func optionTexts(st State) []string {
	var out []string
	for _, o := range st.Options {
		out = append(out, o.Text)
	}
	return out
}

func value(st State, name string) any {
	for _, v := range st.Vars {
		if v.Name == name {
			return v.Value
		}
	}
	return nil
}

func TestNextChainAndSetOnStart(t *testing.T) {
	p := one(t, `Dialogue d {
  Node a { speaker: hero text: "One" set: { gold: 5 } next: b }
  Node b { text: "Two" next: c }
  Node c { text: "Three {{ Variable.gold }}" }
}`)
	s, err := New(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.Start()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Hero: One", "Two", "Three 5"}
	if got := texts(st.Lines); !reflect.DeepEqual(got, want) {
		t.Errorf("lines %v, want %v", got, want)
	}
	if !st.Finished || st.Node != "c" || st.Dialogue != "d" {
		t.Errorf("state %+v", st)
	}
	if !reflect.DeepEqual(st.Lines[0].Set, []Change{{"gold", int64(5)}}) {
		t.Errorf("set %+v", st.Lines[0].Set)
	}
	if value(st, "gold") != int64(5) || value(st, "brave") != false || value(st, "mood") != int64(1) {
		t.Errorf("vars %+v", st.Vars)
	}
	wantPath := []Visit{{"d", "a", ""}, {"d", "b", ""}, {"d", "c", ""}}
	if !reflect.DeepEqual(st.Path, wantPath) {
		t.Errorf("path %+v", st.Path)
	}
}

func TestRequiresAndOptionSet(t *testing.T) {
	p := one(t, `Dialogue d {
  Node a { text: "Pick"
    Option hidden { text: "Hidden" requires: { brave: true } next: b }
    Option shown { text: "Shown" requires: { gold: 3, mood: 1 } set: { brave: true, mood: 2 } next: a }
    Option end { text: "End" }
  }
  Node b { text: "Brave one" }
}`)
	s, _ := New(p, Options{})
	st, _ := s.Start()
	if got := optionTexts(st); !reflect.DeepEqual(got, []string{"Shown", "End"}) {
		t.Fatalf("options %v", got)
	}
	st, err := s.Choose(1)
	if err != nil {
		t.Fatal(err)
	}
	if st.Chosen == nil || st.Chosen.ID != "shown" || !reflect.DeepEqual(st.Chosen.Set, []Change{{"brave", true}, {"mood", int64(2)}}) {
		t.Errorf("chosen %+v", st.Chosen)
	}
	// brave is now true, so hidden shows; shown needs mood = 1 and is gone.
	if got := optionTexts(st); !reflect.DeepEqual(got, []string{"Hidden", "End"}) {
		t.Fatalf("options after set %v", got)
	}
	if st.Path[0].Option != "shown" || len(st.Path) != 2 {
		t.Errorf("path %+v", st.Path)
	}
	st, _ = s.ChooseID("hidden")
	if !st.Finished || st.Text != "Brave one" {
		t.Errorf("state %+v", st)
	}
	if _, err := s.Choose(1); err != ErrFinished {
		t.Errorf("Choose after end: %v", err)
	}
}

func TestEndings(t *testing.T) {
	p := one(t, `Dialogue d {
  Node a { text: "Pick" Option o { text: "Bye" set: { gold: 0 } } }
}
Dialogue e { Node x { text: "Alone" } }`)
	s, _ := New(p, Options{})
	st, _ := s.Start()
	st, err := s.Choose(1)
	if err != nil || !st.Finished || len(st.Lines) != 0 || st.Chosen.Set[0].Name != "gold" {
		t.Errorf("ending by option: %+v, %v", st, err)
	}
	if got := len(s.Transcript()); got != 1 {
		t.Errorf("transcript has %d lines, want 1", got)
	}
	s, _ = New(p, Options{Dialogue: "e"})
	st, _ = s.Start()
	if !st.Finished || st.Text != "Alone" || len(st.Options) != 0 {
		t.Errorf("ending by node: %+v", st)
	}
	st, _ = s.Restart()
	if len(st.Path) != 1 || !st.Finished {
		t.Errorf("restart: %+v", st)
	}
}

func TestCrossDialogueJump(t *testing.T) {
	p := one(t, `Dialogue d {
  Node a { text: "Pick" Option o { text: "Jump" next: Dialogue.e } }
}
Dialogue e {
  start second
  Node first { text: "Not me" }
  Node second { text: "Start here" next: Dialogue.f }
}
Dialogue f { Node z { text: "Last" } }`)
	s, _ := New(p, Options{})
	s.Start()
	st, err := s.Choose(1)
	if err != nil {
		t.Fatal(err)
	}
	if got := texts(st.Lines); !reflect.DeepEqual(got, []string{"Start here", "Last"}) {
		t.Errorf("lines %v", got)
	}
	if st.Dialogue != "f" || st.Node != "z" || !st.Finished {
		t.Errorf("state %+v", st)
	}
}

func TestStartDialogueDefaults(t *testing.T) {
	p := one(t, `Dialogue z { Node a { text: "Z" } }
Dialogue d { Node a { text: "D" } }`)
	s, _ := New(p, Options{})
	if s.Dialogue() != "d" {
		t.Errorf("start = %s, want the Story's start", s.Dialogue())
	}
	if _, err := New(p, Options{Dialogue: "nope"}); err == nil {
		t.Error("unknown dialogue accepted")
	}
	// Without a Story start the first dialogue wins.
	p = load(t, map[string]string{"s.osf": `Versions { language = "1" story = "v1" dialogue = "v1" }
Story s { name = "S" }
Dialogue z { Node a { text: "Z" } }
Dialogue d { Node a { text: "D" } }`})
	s, _ = New(p, Options{})
	if s.Dialogue() != "z" {
		t.Errorf("start = %s, want z", s.Dialogue())
	}
}

func TestReferencesAndEscapes(t *testing.T) {
	p := one(t, `Dialogue d {
  Node a { speaker: hero text: """
    {{ Character.hero }} / {{ Character.hero.name }} / {{ Character.hero.description }}
    {{ Story.s }} / {{ Story.s.name }} / {{ Story.s.description }}
    {{ Dialogue.d }} / {{Variable.brave}} / {{ Variable.gold }} / {{ Variable.mood }}
    \{{ not.a.ref }} and {{ Unknown.x }}
  """ }
}`)
	s, _ := New(p, Options{})
	st, _ := s.Start()
	want := "Hero / Hero / Brave\nS / S / A story\nd / false / 3 / 1\n{{ not.a.ref }} and {{ Unknown.x }}\n"
	if st.Text != want {
		t.Errorf("text:\n%s\nwant:\n%s", st.Text, want)
	}
	if got := s.Resolve("{{ Character.hero }}!"); got != "Hero!" {
		t.Errorf("Resolve = %q", got)
	}
}

func TestTitleAndSpeaker(t *testing.T) {
	p := one(t, `Dialogue d {
  Node a { speaker: hero title: "???" text: "Who" next: b }
  Node b { speaker: hero text: "Me" next: c }
  Node c { title: "{{ Character.hero }} (whispering)" text: "Psst" next: e }
  Node e { text: "Silence" }
}`)
	s, _ := New(p, Options{})
	st, _ := s.Start()
	want := []string{"???: Who", "Hero: Me", "Hero (whispering): Psst", "Silence"}
	if got := texts(st.Lines); !reflect.DeepEqual(got, want) {
		t.Errorf("lines %v, want %v", got, want)
	}
}

const localized = `
I18n { source = Eng Locale Rus "Russian" Locale Jpn "Japanese" }
Dialogue d {
  Node a { speaker: hero title: "{{ Character.hero }}" text: "Hi, {{ Story.s }}" next: b }
  Node b { speaker: hero text: "Untranslated" Option o { text: "Go" } Option p { text: "Stay" } }
}`

const localizedRus = `Locale Rus {
  Story s { name = "История" }
  Character hero { name = "Герой" }
  Dialogue d {
    Node a { text: "Привет, {{ Story.s }}" }
    Node b { Option o { text: "Иди" } }
  }
}`

func TestLocaleFallback(t *testing.T) {
	p := load(t, map[string]string{"s.osf": base + localized, "locales/Rus/s.osf": localizedRus})
	s, err := New(p, Options{Locale: "Rus"})
	if err != nil {
		t.Fatal(err)
	}
	st, _ := s.Start()
	want := []string{"Герой: Привет, История", "Герой: Untranslated"}
	if got := texts(st.Lines); !reflect.DeepEqual(got, want) {
		t.Errorf("lines %v, want %v", got, want)
	}
	if got := optionTexts(st); !reflect.DeepEqual(got, []string{"Иди", "Stay"}) {
		t.Errorf("options %v", got)
	}
	// A declared locale without translations, and the source language, show the source.
	for _, loc := range []string{"Jpn", "Eng", ""} {
		s, err := New(p, Options{Locale: loc})
		if err != nil {
			t.Fatal(err)
		}
		st, _ := s.Start()
		if got := texts(st.Lines)[0]; got != "Hero: Hi, S" {
			t.Errorf("locale %q: %q", loc, got)
		}
	}
	if _, err := New(p, Options{Locale: "Xyz"}); err == nil || !strings.Contains(err.Error(), "unknown locale") {
		t.Errorf("unknown locale: %v", err)
	}
	if got := Locales(p); !reflect.DeepEqual(got, []string{"Eng", "Jpn", "Rus"}) {
		t.Errorf("Locales = %v", got)
	}
}

func TestInfiniteLoopGuard(t *testing.T) {
	p := one(t, `Dialogue d {
  Node a { text: "A" next: b }
  Node b { text: "B" next: Dialogue.d }
}`)
	s, _ := New(p, Options{})
	st, err := s.Start()
	if err != ErrLoop {
		t.Fatalf("err = %v, want ErrLoop", err)
	}
	if !st.Finished || len(st.Lines) != MaxChain {
		t.Errorf("finished %v, %d lines", st.Finished, len(st.Lines))
	}
}

func TestStateIsJSON(t *testing.T) {
	p := one(t, `Dialogue d { Node a { text: "Pick" Option o { text: "Go" set: { gold: 1 } } } }`)
	s, _ := New(p, Options{})
	st, _ := s.Start()
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"dialogue", "node", "speaker", "text", "options", "vars", "finished", "lines", "path"} {
		if _, ok := back[key]; !ok {
			t.Errorf("state JSON lacks %q: %s", key, raw)
		}
	}
	if back["vars"].([]any)[1].(map[string]any)["value"] != float64(3) {
		t.Errorf("vars JSON: %s", raw)
	}
}

func findings(r *Report) []string {
	var out []string
	for _, f := range r.Findings {
		out = append(out, f.Msg)
	}
	return out
}

func TestCoverageDeadEnd(t *testing.T) {
	p := one(t, `Dialogue d {
  Node a { text: "Pick"
    Option o1 { text: "Set" set: { brave: true } next: b }
    Option o2 { text: "Keep" next: b }
  }
  Node b { text: "Gate"
    Option g { text: "Through" requires: { brave: false } }
  }
}`)
	r, err := Coverage(p, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`dead end: d.b has no available option when brave = true, gold = 3, mood = 1`}
	if got := findings(r); !reflect.DeepEqual(got, want) {
		t.Errorf("findings %v, want %v", got, want)
	}
	// Every state stuck: "in every reachable state".
	p = one(t, `Dialogue d {
  Node a { text: "Gate" Option g { text: "Never" requires: { brave: true } } }
}`)
	r, _ = Coverage(p, "")
	want = []string{
		"dead end: d.a has no available option in every reachable state",
		"unreachable option: d.a.g (requires brave = true)",
	}
	if got := findings(r); !reflect.DeepEqual(got, want) {
		t.Errorf("findings %v, want %v", got, want)
	}
}

func TestCoverageUnreachableOption(t *testing.T) {
	p := one(t, `Dialogue d {
  Node a { text: "Pick"
    Option rich { text: "Rich" requires: { gold: 100 } }
    Option poor { text: "Poor" }
  }
}`)
	r, _ := Coverage(p, "")
	want := []string{"unreachable option: d.a.rich (requires gold = 100)"}
	if got := findings(r); !reflect.DeepEqual(got, want) {
		t.Errorf("findings %v, want %v", got, want)
	}
}

func TestCoverageLoopWithNoExit(t *testing.T) {
	p := one(t, `Dialogue d {
  Node a { text: "Pick"
    Option out { text: "Out" }
    Option in { text: "In" next: Dialogue.e }
  }
}
Dialogue e {
  Node x { text: "X" next: y }
  Node y { text: "Y" Option back { text: "Again" next: x } }
}
Dialogue unused { Node z { text: "Z" } }`)
	r, _ := Coverage(p, "")
	want := []string{"loop with no exit: e.x, e.y"}
	if got := findings(r); !reflect.DeepEqual(got, want) {
		t.Errorf("findings %v, want %v", got, want)
	}
	var b strings.Builder
	r.Write(&b)
	wantText := `== Coverage from Dialogue d
d: 1/1 nodes reached
e: 2/2 nodes reached
unused: not reached
3 states, 1 ending
loop with no exit: e.x, e.y
== 1 finding
`
	if b.String() != wantText {
		t.Errorf("report:\n%s\nwant:\n%s", b.String(), wantText)
	}
	// A loop the player can leave is fine, and a self-loop with no exit is not.
	p = one(t, `Dialogue d {
  Node a { text: "Pick" Option again { text: "Again" next: a } Option out { text: "Out" } }
  Node b { text: "Lonely" next: b }
}`)
	r, _ = Coverage(p, "")
	if got := findings(r); len(got) != 0 {
		t.Errorf("findings %v, want none", got)
	}
	if !reflect.DeepEqual(r.Dialogues[0].Unreached, []string{"b"}) {
		t.Errorf("unreached %v", r.Dialogues[0].Unreached)
	}
	r, _ = Coverage(p, "")
	p = one(t, `Dialogue d { Node b { text: "Lonely" next: b } }`)
	r, _ = Coverage(p, "")
	if got := findings(r); !reflect.DeepEqual(got, []string{"loop with no exit: d.b"}) {
		t.Errorf("self loop findings %v", got)
	}
}

func TestOperations(t *testing.T) {
	p := one(t, `
Dialogue d {
  Node a { text: "Pick" set: { gold += 2, mood -= 3 }
    Option spend { text: "Spend" requires: { gold >= 5, mood != 1 } set: { gold -= 9 } next: b }
    Option poor  { text: "Poor" requires: { gold < 5 } next: b }
    Option same  { text: "Same" requires: { gold <= 5, mood > -3 } next: b }
  }
  Node b { text: "Done {{ Variable.gold }}" }
}`)
	s, _ := New(p, Options{})
	st, err := s.Start()
	if err != nil {
		t.Fatal(err)
	}
	// gold 3 + 2 = 5, mood 1 - 3 = -2: spend and same are offered, poor is not.
	if value(st, "gold") != int64(5) || value(st, "mood") != int64(-2) {
		t.Fatalf("vars: %+v", st.Vars)
	}
	if got := optionTexts(st); !reflect.DeepEqual(got, []string{"Spend", "Same"}) {
		t.Fatalf("options: %v", got)
	}
	// An UnsignedNumber stops at 0: 5 - 9 leaves 0.
	st, err = s.ChooseID("spend")
	if err != nil {
		t.Fatal(err)
	}
	if value(st, "gold") != int64(0) || !reflect.DeepEqual(st.Chosen.Set, []Change{{"gold", int64(0)}}) {
		t.Fatalf("after spend: %+v, chosen %+v", st.Vars, st.Chosen)
	}
}
