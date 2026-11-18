// Package play runs a dialogue of an Open Story project as spec §5 defines:
// it follows next chains and their branches, applies set, remembers what was
// played, filters options by requires and resolves {{ }} in text.
// Session is the engine; Coverage explores every reachable path.
package play

import (
	"errors"
	"fmt"
	"sort"

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
	ix        *index
	start     *Dialogue
	dialogues map[string]*Dialogue // by id
	node      *Node
	vars      map[string]osf.Value
	nodes     map[*osf.NodeModel]*Node     // each remembers whether it was played
	nodesBy   map[string][]*Node           // by played key, for played conditions
	options   map[*osf.OptionModel]*Option // each remembers whether it was selected
	byKey     map[string][]*Option         // by selected key, for selected conditions
	finished  bool
	lines     []Line // since the last choice
	all       []Line
	path      []Visit
	chosen    *Chosen
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
	s := &Session{ix: ix, dialogues: map[string]*Dialogue{},
		nodes: map[*osf.NodeModel]*Node{}, nodesBy: map[string][]*Node{},
		options: map[*osf.OptionModel]*Option{}, byKey: map[string][]*Option{}}
	s.build(p)
	d, ok := s.dialogues[id]
	if !ok {
		return nil, fmt.Errorf("Dialogue %s does not exist", id)
	}
	if d.Start() == nil {
		return nil, fmt.Errorf("Dialogue %s has no nodes", id)
	}
	s.start = d
	return s, nil
}

// build makes the Dialogue, Node and Option objects of p, then links every
// next to the node it leads to, which can be in another dialogue.
func (s *Session) build(p *osf.Project) {
	var all []*Dialogue
	for _, dm := range p.Dialogues {
		d := &Dialogue{model: dm, byID: map[string]*Node{}}
		for _, n := range dm.Nodes {
			node := &Node{model: n, dialogue: d}
			d.nodes = append(d.nodes, node)
			if _, dup := d.byID[n.ID]; !dup {
				d.byID[n.ID] = node
			}
			s.nodes[n] = node
			s.nodesBy[node.key()] = append(s.nodesBy[node.key()], node)
			for _, o := range n.Options {
				opt := &Option{model: o, node: node, dialogue: d, conditions: conditionsOf(o.Requires, o.RequiresOr, dm.ID)}
				node.options = append(node.options, opt)
				s.options[o] = opt
				s.byKey[opt.key()] = append(s.byKey[opt.key()], opt)
			}
		}
		all = append(all, d)
		s.dialogues[dm.ID] = d
	}
	for _, d := range all {
		for _, n := range d.nodes {
			n.branches, n.next = branchesOf(n.model.Branches, d, s.dialogues), s.target(d, n.model.Next)
			for _, o := range n.options {
				o.branches, o.next = branchesOf(o.model.Branches, d, s.dialogues), s.target(d, o.model.Next)
			}
		}
	}
}

// target resolves a plain next written in dialogue d; nil when there is none.
func (s *Session) target(d *Dialogue, next *osf.Str) *Target {
	if next == nil {
		return nil
	}
	return targetIn(d, next.Value, s.dialogues)
}

// Start plays from the start node through its next chain and returns the
// state at the first node with options or at the end.
func (s *Session) Start() (State, error) {
	s.vars = map[string]osf.Value{}
	for _, v := range s.ix.p.Variables {
		s.vars[v.ID] = v.Default
	}
	for _, n := range s.nodes {
		n.played = false
	}
	for _, o := range s.options {
		o.selected = false
	}
	s.finished, s.lines, s.all, s.path, s.chosen = false, nil, nil, nil, nil
	s.node = nil
	if err := s.follow(&Target{Node: s.start.Start()}); err != nil {
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
	opts := s.node.AvailableOptions(s.world())
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
	for i, o := range s.node.AvailableOptions(s.world()) {
		if o.ID() == id {
			return s.choose(i+1, o)
		}
	}
	return s.State(), fmt.Errorf("option %q is not available", id)
}

func (s *Session) choose(index int, o *Option) (State, error) {
	s.path[len(s.path)-1].Option = o.ID()
	s.lines = nil
	s.chosen = &Chosen{Index: index, ID: o.ID(), Text: o.Text(s.world())}
	s.chosen.Set = o.Select(s.world())
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
	return world{vars: s.vars, hist: s, ix: s.ix}
}

// has reports whether a history key is set: a node that was entered, or an
// option that says it was selected.
func (s *Session) has(key string) bool {
	for _, o := range s.byKey[key] {
		if o.IsSelected() {
			return true
		}
	}
	for _, n := range s.nodesBy[key] {
		if n.IsPlayed() {
			return true
		}
	}
	return false
}

// follow goes to a target, then keeps following next until a node with
// options or an end.
func (s *Session) follow(t *Target) error {
	for steps := 0; ; steps++ {
		if steps >= MaxChain {
			s.finished = true
			return ErrLoop
		}
		if t.Err != nil {
			return t.Err
		}
		s.node = t.Node
		d := s.node.Dialogue().ID()
		if t.Back {
			// Offer the node's options again: no line, no set.
			s.path = append(s.path, Visit{Dialogue: d, Node: s.node.ID()})
			s.finished = !s.node.HasOptions()
			return nil
		}
		line := Line{Dialogue: d, Node: s.node.ID(), Set: s.node.Enter(s.world())}
		line.Speaker, line.Text = s.node.Speaker(s.world()), s.node.Text(s.world())
		s.lines = append(s.lines, line)
		s.all = append(s.all, line)
		s.path = append(s.path, Visit{Dialogue: d, Node: s.node.ID()})
		next, ok := s.node.Next(s.world())
		if !ok {
			s.finished = !s.node.HasOptions()
			return nil
		}
		t = next
	}
}

// State is the current state; before Start it is empty.
func (s *Session) State() State {
	st := State{Finished: s.finished, Lines: s.lines, Path: s.path, Vars: s.Vars(), Chosen: s.chosen}
	if s.node == nil {
		return st
	}
	st.Dialogue, st.Node = s.node.Dialogue().ID(), s.node.ID()
	if n := len(s.all); n > 0 {
		st.Speaker, st.Text = s.all[n-1].Speaker, s.all[n-1].Text
	}
	if !s.finished {
		for i, o := range s.node.AvailableOptions(s.world()) {
			st.Options = append(st.Options, Choice{Index: i + 1, ID: o.ID(), Text: o.Text(s.world())})
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
func (s *Session) Dialogue() string { return s.start.ID() }

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
