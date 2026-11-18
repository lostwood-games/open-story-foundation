package play

import (
	"fmt"
	"strings"

	"github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"
)

type Condition interface {
	Check(now world) (met bool, reason string)
}

// conditionsOf turns a requires written in dialogue into conditions that
// must all be met. A requires with `or` becomes one anyOf condition.
func conditionsOf(requires []osf.Assign, or [][]osf.Assign, dialogue string) []Condition {
	if len(or) == 0 {
		return conditionList(requires, dialogue)
	}
	var any anyOf
	for _, group := range osf.Groups(requires, or) {
		any = append(any, conditionList(group, dialogue))
	}
	return []Condition{any}
}

func conditionList(entries []osf.Assign, dialogue string) []Condition {
	var out []Condition
	for _, a := range entries {
		out = append(out, newCondition(a, dialogue))
	}
	return out
}

// newCondition is a played or selected condition when the name is
// [Dialogue.<id>.]<node>.stateData.played or
// [Dialogue.<id>.]<node>.options.<option>.selected, else a variable one.
func newCondition(a osf.Assign, dialogue string) Condition {
	if key, ok := historyKey(a.Name, dialogue); ok {
		return historyCondition{assign: a, key: key}
	}
	return variableCondition{assign: a}
}

// variableCondition compares a Variable, as in `gold >= 3`.
type variableCondition struct {
	assign osf.Assign
}

func (c variableCondition) Check(now world) (bool, string) {
	value, ok := now.vars[c.assign.Name]
	if !ok {
		return false, fmt.Sprintf("requires %s, but %s has no value", c.assign.RequiresString(), c.assign.Name)
	}
	if !c.assign.Holds(value) {
		return false, fmt.Sprintf("requires %s, but %s is %s", c.assign.RequiresString(), c.assign.Name, value)
	}
	return true, ""
}

// historyCondition asks whether a node was played or an option selected, as
// in `rumours.stateData.played: true`.
type historyCondition struct {
	assign osf.Assign
	key    string // the history key, already resolved in the condition's dialogue
}

func (c historyCondition) Check(now world) (bool, string) {
	happened := now.hist.has(c.key)
	if !c.assign.Holds(osf.Value{Kind: osf.Bool, Bool: happened}) {
		return false, fmt.Sprintf("requires %s, but it is %v", c.assign.RequiresString(), happened)
	}
	return true, ""
}

// anyOf is a requires with `or`: it is met when every condition of one of
// its groups is met.
type anyOf [][]Condition

func (c anyOf) Check(now world) (bool, string) {
	var reasons []string
	for _, group := range c {
		var notMet []string
		for _, cond := range group {
			if met, reason := cond.Check(now); !met {
				notMet = append(notMet, reason)
			}
		}
		if len(notMet) == 0 {
			return true, ""
		}
		reasons = append(reasons, strings.Join(notMet, " and "))
	}
	return false, strings.Join(reasons, " or ")
}
