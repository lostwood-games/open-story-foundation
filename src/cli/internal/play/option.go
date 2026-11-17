package play

import "github.com/lostwood-games/open-story-foundation/src/cli/internal/osf"

type Option struct {
	model    *osf.Option
	node     *osf.Node
	dialogue *osf.Dialogue
	session  *Session
	selected bool
}

func (o *Option) ID() string { return o.model.ID }

// IsAvailable reports whether the option is offered now: its requires are
// met by the current values and play history.
func (o *Option) IsAvailable() bool { return o.world().allowed(o.model) }

// IsSelected reports whether the option was chosen in this play
// (<node>.options.<option>.selected).
func (o *Option) IsSelected() bool { return o.selected }

// Select chooses the option: it is marked selected and its set is applied.
// It returns the changes.
func (o *Option) Select() []Change {
	o.selected = true
	return o.session.apply(o.model.Set)
}

// Next is where the option leads now: the first branch whose requires are
// met, else its plain next. ok is false when it leads nowhere, which ends the
// play.
func (o *Option) Next() (target string, ok bool) {
	return o.world().next(o.model.Branches, o.model.Next)
}

// Text is the option's text in the play's locale, with references resolved.
func (o *Option) Text() string {
	key := "Dialogue." + o.dialogue.ID + ".nodes." + o.node.ID + ".options." + o.model.ID + ".content.text"
	return o.session.ix.resolve(o.session.ix.text(key, o.model.Text), o.session.vars)
}

// key is the option's played-history key, as selected conditions name it.
func (o *Option) key() string { return selectedKey(o.dialogue.ID, o.node.ID, o.model.ID) }

// world is what the option's conditions are checked against: the session's
// values and history, seen from the option's dialogue.
func (o *Option) world() world {
	return world{dialogue: o.dialogue.ID, vars: o.session.vars, hist: o.session}
}
