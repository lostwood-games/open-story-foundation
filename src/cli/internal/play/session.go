// Package play runs a dialogue of an Open Story project as spec §5 defines:
// it follows next chains and their branches, applies set, remembers what was
// played, filters options by requires and resolves {{ }} in text.
// Session is the engine; Coverage explores every reachable path.
package play

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// MaxChain is the longest run of next links allowed without a choice; a
// longer one is an infinite loop.
const MaxChain = 10000

var (
	ErrFinished = errors.New("the play has ended")
	ErrLoop     = errors.New("infinite loop: more than 10000 nodes without a choice")
)

// Options configure a Session.
type Options struct {
	// Dialogue is the id to start at. Empty means the Story's start, else the
	// first dialogue.
	Dialogue string
	// Locale is the locale id texts are shown in. Empty, or the I18n source
	// language, means the source text.
	Locale string
}

// Session plays one dialogue run. Everything it returns is plain data with
// JSON tags, so a language server can pass it on as is.
type Session struct {
	ix       *index
	start    *osf.Dialogue
	dialogue *osf.Dialogue
	node     *osf.Node
	vars     map[string]osf.Value
	played   history                      // "<dialogue>.<node>" of each node entered
	options  map[*osf.OptionModel]*Option // each remembers whether it was selected
	byKey    map[string][]*Option         // by selected key, for selected conditions
	finished bool
	lines    []Line // since the last choice
	all      []Line
	path     []Visit
	chosen   *Chosen
}

// Line is one node shown to the player.
type Line struct {
	Dialogue string `json:"dialogue"`
	Node     string `json:"node"`
	// Speaker is the display name: the node's title if set, else the speaker
	// document's name; "" for narration.
	Speaker string   `json:"speaker"`
	Text    string   `json:"text"`
	Set     []Change `json:"set,omitempty"` // the node's set, applied when it started
}

// Chosen describes the option picked by the last Choose.
type Chosen struct {
	Index int      `json:"index"`
	ID    string   `json:"id"`
	Text  string   `json:"text"`
	Set   []Change `json:"set,omitempty"` // the option's set, applied when chosen
}

// Change is one variable assignment that was applied.
type Change struct {
	Name  string `json:"name"`
	Value any    `json:"value"` // bool, int64 or string
}

// Choice is an option the player may pick now.
type Choice struct {
	Index int    `json:"index"` // 1-based, as shown to the player
	ID    string `json:"id"`
	Text  string `json:"text"`
}

// Var is a variable with its current value.
type Var struct {
	Name  string `json:"name"`
	Type  string `json:"type"` // boolean, integer or string
	Value any    `json:"value"`
}

// Visit is one step of the path: a node reached and, if the player chose
// there, the option id.
type Visit struct {
	Dialogue string `json:"dialogue"`
	Node     string `json:"node"`
	Option   string `json:"option,omitempty"`
}

// State is what a player sees after Start or Choose.
type State struct {
	Dialogue string   `json:"dialogue"`
	Node     string   `json:"node"`
	Speaker  string   `json:"speaker"`
	Text     string   `json:"text"`
	Options  []Choice `json:"options"`
	Vars     []Var    `json:"vars"`
	Finished bool     `json:"finished"`
	// Chosen is the option picked by the last Choose; nil after Start.
	Chosen *Chosen `json:"chosen,omitempty"`
	// Lines are the nodes shown since the last choice (or since the start),
	// the last one being the current node. Empty when an option ended the play.
	Lines []Line  `json:"lines"`
	Path  []Visit `json:"path"`
}

// New prepares a session; nothing is played until Start.
func New(p *osf.Project, opts Options) (*Session, error) {
	if len(p.Dialogues) == 0 {
		return nil, errors.New("the project has no Dialogue")
	}
	if opts.Locale != "" && !contains(Locales(p), opts.Locale) {
		return nil, fmt.Errorf("unknown locale %q; the project has: %v", opts.Locale, Locales(p))
	}
	ix := newIndex(p, opts.Locale)
	id := opts.Dialogue
	if id == "" {
		id = p.Dialogues[0].ID
		if len(p.Stories) > 0 && p.Stories[0].Start != nil {
			id = p.Stories[0].Start.Value[len("Dialogue."):]
		}
	}
	d, ok := ix.dialogues[id]
	if !ok {
		return nil, fmt.Errorf("Dialogue %s does not exist", id)
	}
	if startNode(d) == nil {
		return nil, fmt.Errorf("Dialogue %s has no nodes", id)
	}
	s := &Session{ix: ix, start: d, options: map[*osf.OptionModel]*Option{}, byKey: map[string][]*Option{}}
	for _, d := range p.Dialogues {
		for _, n := range d.Nodes {
			for _, o := range n.Options {
				opt := &Option{model: o, node: n, dialogue: d, session: s, conditions: conditionsOf(o.Requires, o.RequiresOr, d.ID)}
				s.options[o] = opt
				s.byKey[opt.key()] = append(s.byKey[opt.key()], opt)
			}
		}
	}
	return s, nil
}

// Start plays from the start node through its next chain and returns the
// state at the first node with options or at the end.
func (s *Session) Start() (State, error) {
	s.vars = map[string]osf.Value{}
	for _, v := range s.ix.p.Variables {
		s.vars[v.ID] = v.Default
	}
	s.played = history{}
	for _, o := range s.options {
		o.selected = false
	}
	s.finished, s.lines, s.all, s.path, s.chosen = false, nil, nil, nil, nil
	s.dialogue, s.node = s.start, nil
	if err := s.follow(startNode(s.start).ID); err != nil {
		return s.State(), err
	}
	return s.State(), nil
}

// Restart is Start again.
func (s *Session) Restart() (State, error) { return s.Start() }

// Choose picks the option shown at 1-based index among the allowed ones,
// applies its set and follows its next.
func (s *Session) Choose(index int) (State, error) {
	if s.node == nil {
		if _, err := s.Start(); err != nil {
			return s.State(), err
		}
	}
	if s.finished {
		return s.State(), ErrFinished
	}
	opts := s.allowedOptions()
	if index < 1 || index > len(opts) {
		return s.State(), fmt.Errorf("choice %d is out of range (1-%d)", index, len(opts))
	}
	return s.choose(index, opts[index-1])
}

// ChooseID picks the allowed option with the given id.
func (s *Session) ChooseID(id string) (State, error) {
	if s.node == nil {
		if _, err := s.Start(); err != nil {
			return s.State(), err
		}
	}
	if s.finished {
		return s.State(), ErrFinished
	}
	for i, o := range s.allowedOptions() {
		if o.ID() == id {
			return s.choose(i+1, o)
		}
	}
	return s.State(), fmt.Errorf("option %q is not available", id)
}

func (s *Session) choose(index int, o *Option) (State, error) {
	s.path[len(s.path)-1].Option = o.ID()
	s.lines = nil
	s.chosen = &Chosen{Index: index, ID: o.ID(), Text: o.Text()}
	s.chosen.Set = o.Select()
	next, ok := o.Next(s.world())
	if !ok {
		s.finished = true
		return s.State(), nil
	}
	if err := s.follow(next); err != nil {
		return s.State(), err
	}
	return s.State(), nil
}

// world is what conditions are checked against at this moment.
func (s *Session) world() world {
	return world{dialogue: s.dialogue.ID, vars: s.vars, hist: s}
}

// has reports whether a history key is set: a node that was entered, or an
// option that says it was selected.
func (s *Session) has(key string) bool {
	for _, o := range s.byKey[key] {
		if o.IsSelected() {
			return true
		}
	}
	return s.played[key]
}

// follow goes to a next target: a node of the current dialogue,
// <node>.options or Dialogue.<id>, then keeps following next until a node
// with options or an end.
func (s *Session) follow(next string) error {
	d, n := s.dialogue, (*osf.Node)(nil)
	for steps := 0; ; steps++ {
		if steps >= MaxChain {
			s.finished = true
			return ErrLoop
		}
		if strings.HasPrefix(next, "Dialogue.") {
			nd, ok := s.ix.dialogues[strings.TrimPrefix(next, "Dialogue.")]
			if !ok {
				return fmt.Errorf("next: %s does not exist", next)
			}
			if d, n = nd, startNode(nd); n == nil {
				return fmt.Errorf("next: Dialogue %s has no nodes", nd.ID)
			}
		} else if id, back := backToOptions(next); back {
			// Offer the node's options again: no line, no set.
			if n = findNode(d, id); n == nil {
				return fmt.Errorf("next: node %q does not exist in Dialogue %s", id, d.ID)
			}
			s.dialogue, s.node = d, n
			s.path = append(s.path, Visit{Dialogue: d.ID, Node: n.ID})
			s.finished = len(n.Options) == 0
			return nil
		} else if n = findNode(d, next); n == nil {
			return fmt.Errorf("next: node %q does not exist in Dialogue %s", next, d.ID)
		}
		s.dialogue, s.node = d, n
		s.played[playedKey(d.ID, n.ID)] = true
		line := Line{Dialogue: d.ID, Node: n.ID, Set: s.apply(n.Set)}
		line.Speaker, line.Text = s.speaker(n), s.nodeText(n)
		s.lines = append(s.lines, line)
		s.all = append(s.all, line)
		s.path = append(s.path, Visit{Dialogue: d.ID, Node: n.ID})
		target, ok := s.world().next(n.Branches, n.Next)
		if !ok {
			s.finished = len(n.Options) == 0
			return nil
		}
		next = target
	}
}

func (s *Session) apply(set []osf.Assign) []Change {
	var out []Change
	for _, a := range set {
		s.vars[a.Name] = a.Apply(s.ix.typeOf(a.Name), s.vars[a.Name])
		out = append(out, Change{Name: a.Name, Value: jsonValue(s.vars[a.Name])})
	}
	return out
}

func (s *Session) speaker(n *osf.Node) string {
	key := "Dialogue." + s.dialogue.ID + ".nodes." + n.ID + ".content."
	if n.Title != nil {
		return s.ix.resolve(s.ix.text(key+"title", n.Title), s.vars)
	}
	if n.Speaker != nil {
		id := n.Speaker.Value[len("Character."):]
		if c, ok := s.ix.characters[id]; ok {
			return s.ix.resolve(s.ix.text("Character."+id+".name", c.Name), s.vars)
		}
	}
	return ""
}

func (s *Session) nodeText(n *osf.Node) string {
	key := "Dialogue." + s.dialogue.ID + ".nodes." + n.ID + ".content.text"
	return s.ix.resolve(s.ix.text(key, n.Text), s.vars)
}

// allowedOptions lists the current node's options that are available now.
func (s *Session) allowedOptions() []*Option {
	var out []*Option
	for _, o := range s.node.Options {
		opt := s.options[o]
		if ok, _ := opt.IsAvailable(s.world()); ok {
			out = append(out, opt)
		}
	}
	return out
}

// State is the current state; before Start it is empty.
func (s *Session) State() State {
	st := State{Finished: s.finished, Lines: s.lines, Path: s.path, Vars: s.Vars(), Chosen: s.chosen}
	if s.node == nil {
		return st
	}
	st.Dialogue, st.Node = s.dialogue.ID, s.node.ID
	if n := len(s.all); n > 0 {
		st.Speaker, st.Text = s.all[n-1].Speaker, s.all[n-1].Text
	}
	if !s.finished {
		for i, o := range s.allowedOptions() {
			st.Options = append(st.Options, Choice{Index: i + 1, ID: o.ID(), Text: o.Text()})
		}
	}
	return st
}

// Vars lists the variables with their current values, sorted by name.
func (s *Session) Vars() []Var {
	var out []Var
	for _, v := range s.ix.p.Variables {
		val, ok := s.vars[v.ID]
		if !ok {
			val = v.Default
		}
		out = append(out, Var{Name: v.ID, Type: v.Type.String(), Value: jsonValue(val)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Transcript is every line shown since Start.
func (s *Session) Transcript() []Line { return s.all }

// Dialogue is the dialogue the session starts at.
func (s *Session) Dialogue() string { return s.start.ID }

// Resolve replaces {{ }} references in text using the session's locale and
// current variable values, for callers that show their own texts.
func (s *Session) Resolve(text string) string {
	if s.vars == nil {
		vars := map[string]osf.Value{}
		for _, v := range s.ix.p.Variables {
			vars[v.ID] = v.Default
		}
		return s.ix.resolve(text, vars)
	}
	return s.ix.resolve(text, s.vars)
}

func jsonValue(v osf.Value) any {
	if v.Kind == osf.Bool {
		return v.Bool
	}
	return v.Int
}

// FormatValue is a value as written in .osf: true, 3, "text".
func FormatValue(v any) string {
	if s, ok := v.(string); ok {
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprint(v)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
