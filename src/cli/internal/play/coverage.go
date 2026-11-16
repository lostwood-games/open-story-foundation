package play

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

// MaxStates bounds the coverage search; a project with more distinct
// (dialogue, node, variables) states is reported as an error.
const MaxStates = 200000

// Finding is one coverage problem.
type Finding struct {
	Kind     string `json:"kind"` // dead end, unreachable option, loop with no exit
	Dialogue string `json:"dialogue"`
	Node     string `json:"node"`
	Option   string `json:"option,omitempty"`
	Msg      string `json:"msg"` // the full line as printed
}

// DialogueCoverage counts the nodes of one dialogue reached from the start.
type DialogueCoverage struct {
	ID        string   `json:"id"`
	Reached   int      `json:"reached"`
	Total     int      `json:"total"`
	Unreached []string `json:"unreached,omitempty"`
}

// Report is the result of Coverage.
type Report struct {
	Start     string             `json:"start"`
	Dialogues []DialogueCoverage `json:"dialogues"`
	States    int                `json:"states"`
	Endings   int                `json:"endings"`
	Findings  []Finding          `json:"findings"`
}

// state is a place in the play: a node whose set has been applied, with the
// variable values at that moment.
type state struct {
	d    *osf.Dialogue
	n    *osf.Node
	vars map[string]osf.Value
}

type explorer struct {
	ix      *index
	states  map[string]*state
	order   []string            // state keys in discovery order
	edges   map[string][]string // state key -> successor keys; end is the ending
	stuck   map[string][]string // node key -> var keys with no available option
	visited map[string][]string // node key -> var keys reached
	usable  map[string]bool     // option key -> allowed somewhere
	endings int
}

const end = "\x00end"

// Coverage explores every reachable path from dialogue (the Story's start, or
// the first dialogue, when empty) and reports dead ends, unreachable options
// and loops with no exit.
func Coverage(p *osf.Project, dialogue string) (*Report, error) {
	s, err := New(p, Options{Dialogue: dialogue})
	if err != nil {
		return nil, err
	}
	e := &explorer{
		ix:      s.ix,
		states:  map[string]*state{},
		edges:   map[string][]string{},
		stuck:   map[string][]string{},
		visited: map[string][]string{},
		usable:  map[string]bool{},
	}
	vars := map[string]osf.Value{}
	for _, v := range p.Variables {
		vars[v.ID] = v.Default
	}
	e.add(s.start, startNode(s.start), vars)
	for i := 0; i < len(e.order); i++ {
		if len(e.order) > MaxStates {
			return nil, fmt.Errorf("coverage: more than %d states; the variable space is too large to explore", MaxStates)
		}
		e.expand(e.order[i])
	}
	r := &Report{Start: s.start.ID, States: len(e.order), Endings: e.endings}
	e.summary(p, r)
	e.deadEnds(p, r)
	e.unreachableOptions(p, r)
	e.loops(p, r)
	return r, nil
}

func (e *explorer) varsKey(vars map[string]osf.Value) string {
	var parts []string
	for _, v := range e.ix.p.Variables {
		parts = append(parts, v.ID+" = "+vars[v.ID].String())
	}
	return strings.Join(parts, ", ")
}

// add enters node n with vars, applies its set and returns the state key.
func (e *explorer) add(d *osf.Dialogue, n *osf.Node, vars map[string]osf.Value) string {
	next := copyVars(vars)
	for _, a := range n.Set {
		next[a.Name] = a.Apply(e.ix.typeOf(a.Name), next[a.Name])
	}
	vk := e.varsKey(next)
	key := d.ID + "." + n.ID + " [" + vk + "]"
	if _, ok := e.states[key]; !ok {
		e.states[key] = &state{d, n, next}
		e.order = append(e.order, key)
		nk := d.ID + "." + n.ID
		e.visited[nk] = append(e.visited[nk], vk)
	}
	return key
}

func (e *explorer) expand(key string) {
	st := e.states[key]
	if st.n.Next != nil {
		if d, n := e.target(st.d, st.n.Next.Value); n != nil {
			e.edges[key] = append(e.edges[key], e.add(d, n, st.vars))
		}
		return
	}
	if len(st.n.Options) == 0 {
		e.edges[key] = append(e.edges[key], end)
		e.endings++
		return
	}
	any := false
	for _, o := range st.n.Options {
		if !allowed(o, st.vars) {
			continue
		}
		any = true
		e.usable[st.d.ID+"."+st.n.ID+"."+o.ID] = true
		vars := copyVars(st.vars)
		for _, a := range o.Set {
			vars[a.Name] = a.Apply(e.ix.typeOf(a.Name), vars[a.Name])
		}
		if o.Next == nil {
			e.edges[key] = append(e.edges[key], end)
			e.endings++
			continue
		}
		if d, n := e.target(st.d, o.Next.Value); n != nil {
			e.edges[key] = append(e.edges[key], e.add(d, n, vars))
		}
	}
	if !any {
		nk := st.d.ID + "." + st.n.ID
		e.stuck[nk] = append(e.stuck[nk], e.varsKey(st.vars))
	}
}

// target resolves a next value; nil when it points nowhere (validation
// reports that separately).
func (e *explorer) target(d *osf.Dialogue, next string) (*osf.Dialogue, *osf.Node) {
	if strings.HasPrefix(next, "Dialogue.") {
		nd, ok := e.ix.dialogues[strings.TrimPrefix(next, "Dialogue.")]
		if !ok {
			return nil, nil
		}
		return nd, startNode(nd)
	}
	return d, findNode(d, next)
}

func (e *explorer) summary(p *osf.Project, r *Report) {
	for _, d := range p.Dialogues {
		dc := DialogueCoverage{ID: d.ID, Total: len(d.Nodes)}
		for _, n := range d.Nodes {
			if len(e.visited[d.ID+"."+n.ID]) > 0 {
				dc.Reached++
			} else {
				dc.Unreached = append(dc.Unreached, n.ID)
			}
		}
		r.Dialogues = append(r.Dialogues, dc)
	}
}

func (e *explorer) deadEnds(p *osf.Project, r *Report) {
	for _, d := range p.Dialogues {
		for _, n := range d.Nodes {
			nk := d.ID + "." + n.ID
			stuck := e.stuck[nk]
			if len(stuck) == 0 {
				continue
			}
			var when string
			if len(stuck) == len(e.visited[nk]) || len(p.Variables) == 0 {
				when = "in every reachable state"
			} else {
				sort.Strings(stuck)
				when = "when " + strings.Join(stuck, " or when ")
			}
			r.Findings = append(r.Findings, Finding{Kind: "dead end", Dialogue: d.ID, Node: n.ID,
				Msg: fmt.Sprintf("dead end: %s has no available option %s", nk, when)})
		}
	}
}

func (e *explorer) unreachableOptions(p *osf.Project, r *Report) {
	for _, d := range p.Dialogues {
		for _, n := range d.Nodes {
			if len(e.visited[d.ID+"."+n.ID]) == 0 {
				continue // the node itself is never reached; the summary says so
			}
			for _, o := range n.Options {
				ok := d.ID + "." + n.ID + "." + o.ID
				if e.usable[ok] {
					continue
				}
				var groups []string
				for _, g := range osf.Groups(o.Requires, o.RequiresOr) {
					var req []string
					for _, a := range g {
						req = append(req, strings.Replace(a.RequiresString(), ": ", " = ", 1))
					}
					groups = append(groups, strings.Join(req, ", "))
				}
				r.Findings = append(r.Findings, Finding{Kind: "unreachable option", Dialogue: d.ID, Node: n.ID, Option: o.ID,
					Msg: fmt.Sprintf("unreachable option: %s (requires %s)", ok, strings.Join(groups, " or "))})
			}
		}
	}
}

// loops finds cyclic strongly connected sets of states from which no ending
// is reachable, and reports each distinct set of nodes once.
func (e *explorer) loops(p *osf.Project, r *Report) {
	// States that can reach an ending: walk the reversed graph from end.
	rev := map[string][]string{}
	for from, tos := range e.edges {
		for _, to := range tos {
			rev[to] = append(rev[to], from)
		}
	}
	canEnd := map[string]bool{end: true}
	queue := []string{end}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		for _, from := range rev[k] {
			if !canEnd[from] {
				canEnd[from] = true
				queue = append(queue, from)
			}
		}
	}
	seen := map[string]bool{}
	var msgs []string
	for _, scc := range e.sccs() {
		cyclic := len(scc) > 1
		if !cyclic {
			for _, to := range e.edges[scc[0]] {
				cyclic = cyclic || to == scc[0]
			}
		}
		if !cyclic || canEnd[scc[0]] {
			continue
		}
		nodes := map[string]bool{}
		for _, k := range scc {
			nodes[e.states[k].d.ID+"."+e.states[k].n.ID] = true
		}
		var list []string
		for nk := range nodes {
			list = append(list, nk)
		}
		sort.Strings(list)
		msg := "loop with no exit: " + strings.Join(list, ", ")
		if !seen[msg] {
			seen[msg] = true
			msgs = append(msgs, msg)
		}
	}
	sort.Strings(msgs)
	for _, msg := range msgs {
		first := strings.SplitN(strings.TrimPrefix(msg, "loop with no exit: "), ", ", 2)[0]
		dn := strings.SplitN(first, ".", 2)
		r.Findings = append(r.Findings, Finding{Kind: "loop with no exit", Dialogue: dn[0], Node: dn[1], Msg: msg})
	}
}

// sccs is Tarjan's algorithm over the explored states, iterative over the
// discovery order so the result is deterministic.
func (e *explorer) sccs() [][]string {
	index := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var out [][]string
	next := 0
	var visit func(k string)
	visit = func(k string) {
		index[k], low[k] = next, next
		next++
		stack = append(stack, k)
		onStack[k] = true
		for _, to := range e.edges[k] {
			if to == end {
				continue
			}
			if _, ok := index[to]; !ok {
				visit(to)
				low[k] = min(low[k], low[to])
			} else if onStack[to] {
				low[k] = min(low[k], index[to])
			}
		}
		if low[k] == index[k] {
			var scc []string
			for {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[top] = false
				scc = append(scc, top)
				if top == k {
					break
				}
			}
			out = append(out, scc)
		}
	}
	for _, k := range e.order {
		if _, ok := index[k]; !ok {
			visit(k)
		}
	}
	return out
}

func copyVars(vars map[string]osf.Value) map[string]osf.Value {
	out := make(map[string]osf.Value, len(vars))
	for k, v := range vars {
		out[k] = v
	}
	return out
}

// Write prints the report in the format documented in tests/play/README.md.
func (r *Report) Write(w io.Writer) {
	fmt.Fprintf(w, "== Coverage from Dialogue %s\n", r.Start)
	for _, d := range r.Dialogues {
		switch {
		case d.Reached == 0:
			fmt.Fprintf(w, "%s: not reached\n", d.ID)
		case len(d.Unreached) > 0:
			fmt.Fprintf(w, "%s: %d/%d nodes reached (not reached: %s)\n", d.ID, d.Reached, d.Total, strings.Join(d.Unreached, ", "))
		default:
			fmt.Fprintf(w, "%s: %d/%d nodes reached\n", d.ID, d.Reached, d.Total)
		}
	}
	fmt.Fprintf(w, "%s, %s\n", plural(r.States, "state"), plural(r.Endings, "ending"))
	for _, f := range r.Findings {
		fmt.Fprintln(w, f.Msg)
	}
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "== No findings")
	} else {
		fmt.Fprintf(w, "== %s\n", plural(len(r.Findings), "finding"))
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
