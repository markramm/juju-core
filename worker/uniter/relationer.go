package uniter

import (
	"fmt"
	"launchpad.net/juju-core/cmd/jujuc/server"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/presence"
)

// Relationer manages a unit's presence in a relation.
type Relationer struct {
	ctx    *server.RelationContext
	ru     *state.RelationUnit
	rs     *RelationState
	pinger *presence.Pinger
	queue  *HookQueue
	hooks  chan<- HookInfo
}

// NewRelationer creates a new Relationer. The unit will not join the
// relation until explicitly requested.
func NewRelationer(ru *state.RelationUnit, rs *RelationState, hooks chan<- HookInfo) *Relationer {
	// TODO lifecycle handling?
	return &Relationer{
		ctx:   server.NewRelationContext(ru, rs.Members),
		ru:    ru,
		rs:    rs,
		hooks: hooks,
	}
}

// Join starts the periodic signalling of the unit's presence in the relation.
// It must not be called again until Abandon has been called.
func (r *Relationer) Join() error {
	if r.pinger != nil {
		panic("unit already joined!")
	}
	pinger, err := r.ru.Join()
	if err != nil {
		return err
	}
	r.pinger = pinger
	return nil
}

// Abandon stops the periodic signalling of the unit's presence in the relation.
// It does not immediately signal that the unit has departed the relation; see
// the Depart method.
func (r *Relationer) Abandon() error {
	if r.pinger == nil {
		return nil
	}
	pinger := r.pinger
	r.pinger = nil
	return pinger.Stop()
}

// Depart immediately signals that the unit has departed the relation, and
// cleans up local state.
func (r *Relationer) Depart() error {
	panic("not implemented")
}

// StartHooks starts watching the relation, and sending HookInfo events on the
// hooks channel. It will panic if called when already responding to relation
// changes.
func (r *Relationer) StartHooks() {
	if r.queue != nil {
		panic("hooks already started!")
	}
	r.queue = NewHookQueue(r.rs, r.hooks, r.ru.Watch())
}

// StopHooks ensures that the relationer is not watching the relation, or sending
// HookInfo events on the hooks channel.
func (r *Relationer) StopHooks() error {
	if r.queue == nil {
		return nil
	}
	queue := r.queue
	r.queue = nil
	return queue.Stop()
}

// Context returns the RelationContext associated with r.
func (r *Relationer) Context() *server.RelationContext {
	return r.ctx
}

// PrepareHook checks that the relation is in a state such that it makes
// sense to execute the supplied hook, and ensures that the relation context
// contains the latest relation state as communicated in the HookInfo. It
// returns the name of the hook that must be run.
func (r *Relationer) PrepareHook(hi HookInfo) (hookName string, err error) {
	if err = r.rs.Validate(hi); err != nil {
		return "", err
	}
	r.ctx.SetMembers(hi.Members)
	relName := r.ru.Endpoint().RelationName
	return fmt.Sprintf("%s-relation-%s", relName, hi.HookKind), nil
}

// CommitHook persists the fact of the supplied hook's completion.
func (r *Relationer) CommitHook(hi HookInfo) error {
	return r.rs.Commit(hi)
}