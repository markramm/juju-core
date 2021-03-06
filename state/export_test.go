// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"

	"labix.org/v2/mgo"
	. "launchpad.net/gocheck"

	"launchpad.net/juju-core/charm"
	"launchpad.net/juju-core/environs/config"
	"launchpad.net/juju-core/instance"
	"launchpad.net/juju-core/testing"
)

// transactionHook holds Before and After func()s that will be called
// respectively before and after a particular state transaction is executed.
type TransactionHook transactionHook

// TransactionChecker values are returned from the various Set*Hooks calls,
// and should be run after the code under test has been executed to check
// that the expected number of transactions were run.
type TransactionChecker func()

func (c TransactionChecker) Check() {
	c()
}

// SetTransactionHooks queues up hooks to be applied to the next transactions,
// and returns a function that asserts all hooks have been run (and removes any
// that have not). Each hook function can freely execute its own transactions
// without causing other hooks to be triggered.
// It returns a function that asserts that all hooks have been run, and removes
// any that have not. It is an error to set transaction hooks when any are
// already queued; and setting transaction hooks renders the *State goroutine-
// unsafe.
func SetTransactionHooks(c *C, st *State, transactionHooks ...TransactionHook) TransactionChecker {
	converted := make([]transactionHook, len(transactionHooks))
	for i, hook := range transactionHooks {
		converted[i] = transactionHook(hook)
		c.Logf("%d: %#v", i, converted[i])
	}
	original := <-st.transactionHooks
	st.transactionHooks <- converted
	c.Assert(original, HasLen, 0)
	return func() {
		remaining := <-st.transactionHooks
		st.transactionHooks <- nil
		c.Assert(remaining, HasLen, 0)
	}
}

// SetBeforeHooks uses SetTransactionHooks to queue N functions to be run
// immediately before the next N transactions. Nil values are accepted, and
// useful, in that they can be used to ensure that a transaction is run at
// the expected time, without having to make any changes or assert any state.
func SetBeforeHooks(c *C, st *State, fs ...func()) TransactionChecker {
	transactionHooks := make([]TransactionHook, len(fs))
	for i, f := range fs {
		transactionHooks[i] = TransactionHook{Before: f}
	}
	return SetTransactionHooks(c, st, transactionHooks...)
}

// SetRetryHooks uses SetTransactionHooks to inject a block function designed
// to disrupt a transaction built against recent state, and a check function
// designed to verify that the replacement transaction against the new state
// has been applied as expected.
func SetRetryHooks(c *C, st *State, block, check func()) TransactionChecker {
	return SetTransactionHooks(c, st, TransactionHook{
		Before: block,
	}, TransactionHook{
		After: check,
	})
}

// TestingInitialize initializes the state and returns it. If state was not
// already initialized, and cfg is nil, the minimal default environment
// configuration will be used.
func TestingInitialize(c *C, cfg *config.Config) *State {
	if cfg == nil {
		cfg = testing.EnvironConfig(c)
	}
	st, err := Initialize(TestingStateInfo(), cfg, TestingDialOpts())
	c.Assert(err, IsNil)
	return st
}

type (
	CharmDoc    charmDoc
	MachineDoc  machineDoc
	RelationDoc relationDoc
	ServiceDoc  serviceDoc
	UnitDoc     unitDoc
)

func (doc *MachineDoc) String() string {
	m := &Machine{doc: machineDoc(*doc)}
	return m.String()
}

func ServiceSettingsRefCount(st *State, serviceName string, curl *charm.URL) (int, error) {
	key := serviceSettingsKey(serviceName, curl)
	var doc settingsRefsDoc
	if err := st.settingsrefs.FindId(key).One(&doc); err == nil {
		return doc.RefCount, nil
	}
	return 0, mgo.ErrNotFound
}

func AddTestingCharm(c *C, st *State, name string) *Charm {
	return addCharm(c, st, "series", testing.Charms.Dir(name))
}

func AddCustomCharm(c *C, st *State, name, filename, content, series string, revision int) *Charm {
	path := testing.Charms.ClonedDirPath(c.MkDir(), name)
	if filename != "" {
		config := filepath.Join(path, filename)
		err := ioutil.WriteFile(config, []byte(content), 0644)
		c.Assert(err, IsNil)
	}
	ch, err := charm.ReadDir(path)
	c.Assert(err, IsNil)
	if revision != -1 {
		ch.SetRevision(revision)
	}
	return addCharm(c, st, series, ch)
}

func addCharm(c *C, st *State, series string, ch charm.Charm) *Charm {
	ident := fmt.Sprintf("%s-%s-%d", series, ch.Meta().Name, ch.Revision())
	curl := charm.MustParseURL("local:" + series + "/" + ident)
	bundleURL, err := url.Parse("http://bundles.testing.invalid/" + ident)
	c.Assert(err, IsNil)
	sch, err := st.AddCharm(ch, curl, bundleURL, ident+"-sha256")
	c.Assert(err, IsNil)
	return sch
}

func MachineIdLessThan(id1, id2 string) bool {
	return machineIdLessThan(id1, id2)
}

// SCHEMACHANGE
// This method is used to reset a deprecated machine attriute.
func SetMachineInstanceId(m *Machine, instanceId string) {
	m.doc.InstanceId = instance.Id(instanceId)
}

func init() {
	logSize = logSizeTests
}

// MinUnitsRevno returns the Revno of the minUnits document
// associated with the given service name.
func MinUnitsRevno(st *State, serviceName string) (int, error) {
	var doc minUnitsDoc
	if err := st.minUnits.FindId(serviceName).One(&doc); err != nil {
		return 0, err
	}
	return doc.Revno, nil
}
