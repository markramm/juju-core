// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package machine_test

import (
	. "launchpad.net/gocheck"

	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/api/params"
	"launchpad.net/juju-core/state/apiserver/common"
	"launchpad.net/juju-core/state/apiserver/machine"
	statetesting "launchpad.net/juju-core/state/testing"
)

type machinerSuite struct {
	commonSuite

	resources *common.Resources
	machiner  *machine.MachinerAPI
}

var _ = Suite(&machinerSuite{})

func (s *machinerSuite) SetUpTest(c *C) {
	s.commonSuite.SetUpTest(c)

	// Create the resource registry separately to track invocations to
	// Register.
	s.resources = common.NewResources()

	// Create a machiner API for machine 1.
	machiner, err := machine.NewMachinerAPI(
		s.State,
		s.resources,
		s.authorizer,
	)
	c.Assert(err, IsNil)
	s.machiner = machiner
}

func (s *machinerSuite) assertError(c *C, err *params.Error, code, messageRegexp string) {
	c.Assert(err, NotNil)
	c.Assert(params.ErrCode(err), Equals, code)
	c.Assert(err, ErrorMatches, messageRegexp)
}

func (s *machinerSuite) TestMachinerFailsWithNonMachineAgentUser(c *C) {
	anAuthorizer := s.authorizer
	anAuthorizer.MachineAgent = false
	aMachiner, err := machine.NewMachinerAPI(s.State, s.resources, anAuthorizer)
	c.Assert(err, NotNil)
	c.Assert(aMachiner, IsNil)
	c.Assert(err, ErrorMatches, "permission denied")
}

func (s *machinerSuite) TestSetStatus(c *C) {
	err := s.machine0.SetStatus(params.StatusStarted, "blah")
	c.Assert(err, IsNil)
	err = s.machine1.SetStatus(params.StatusStopped, "foo")
	c.Assert(err, IsNil)

	args := params.MachinesSetStatus{
		Machines: []params.MachineSetStatus{
			{Tag: "machine-1", Status: params.StatusError, Info: "not really"},
			{Tag: "machine-0", Status: params.StatusStopped, Info: "foobar"},
			{Tag: "machine-42", Status: params.StatusStarted, Info: "blah"},
		}}
	result, err := s.machiner.SetStatus(args)
	c.Assert(err, IsNil)
	c.Assert(result.Errors, HasLen, 3)
	c.Assert(result.Errors[0], IsNil)
	s.assertError(c, result.Errors[1], params.CodeUnauthorized, "permission denied")
	s.assertError(c, result.Errors[2], params.CodeUnauthorized, "permission denied")

	// Verify machine 0 - no change.
	status, info, err := s.machine0.Status()
	c.Assert(err, IsNil)
	c.Assert(status, Equals, params.StatusStarted)
	c.Assert(info, Equals, "blah")
	// ...machine 1 is fine though.
	status, info, err = s.machine1.Status()
	c.Assert(err, IsNil)
	c.Assert(status, Equals, params.StatusError)
	c.Assert(info, Equals, "not really")
}

func (s *machinerSuite) TestLife(c *C) {
	err := s.machine1.EnsureDead()
	c.Assert(err, IsNil)
	err = s.machine1.Refresh()
	c.Assert(err, IsNil)
	c.Assert(s.machine1.Life(), Equals, state.Dead)

	args := params.Entities{Entities: []params.Entity{
		{Tag: "machine-1"},
		{Tag: "machine-0"},
		{Tag: "machine-42"},
	}}
	result, err := s.machiner.Life(args)
	c.Assert(err, IsNil)
	c.Assert(result.Results, HasLen, 3)
	c.Assert(result.Results[0].Error, IsNil)
	c.Assert(string(result.Results[0].Life), Equals, "dead")
	s.assertError(c, result.Results[1].Error, params.CodeUnauthorized, "permission denied")
	s.assertError(c, result.Results[2].Error, params.CodeUnauthorized, "permission denied")
}

func (s *machinerSuite) TestEnsureDead(c *C) {
	c.Assert(s.machine0.Life(), Equals, state.Alive)
	c.Assert(s.machine1.Life(), Equals, state.Alive)

	args := params.Entities{Entities: []params.Entity{
		{Tag: "machine-1"},
		{Tag: "machine-0"},
		{Tag: "machine-42"},
	}}
	result, err := s.machiner.EnsureDead(args)
	c.Assert(err, IsNil)
	c.Assert(result.Errors, HasLen, 3)
	c.Assert(result.Errors[0], IsNil)
	s.assertError(c, result.Errors[1], params.CodeUnauthorized, "permission denied")
	s.assertError(c, result.Errors[2], params.CodeUnauthorized, "permission denied")

	err = s.machine0.Refresh()
	c.Assert(err, IsNil)
	c.Assert(s.machine0.Life(), Equals, state.Alive)
	err = s.machine1.Refresh()
	c.Assert(err, IsNil)
	c.Assert(s.machine1.Life(), Equals, state.Dead)

	// Try it again on a Dead machine; should work.
	args = params.Entities{
		Entities: []params.Entity{{Tag: "machine-1"}},
	}
	result, err = s.machiner.EnsureDead(args)
	c.Assert(err, IsNil)
	c.Assert(result.Errors, HasLen, 1)
	c.Assert(result.Errors[0], IsNil)

	// Verify Life is unchanged.
	err = s.machine1.Refresh()
	c.Assert(err, IsNil)
	c.Assert(s.machine1.Life(), Equals, state.Dead)
}

func (s *machinerSuite) TestWatch(c *C) {
	c.Assert(s.resources.Count(), Equals, 0)

	args := params.Entities{Entities: []params.Entity{
		{Tag: "machine-1"},
		{Tag: "machine-0"},
		{Tag: "machine-42"},
	}}
	result, err := s.machiner.Watch(args)
	c.Assert(err, IsNil)
	c.Assert(result.Results, HasLen, 3)
	c.Assert(result.Results[0].Error, IsNil)
	s.assertError(c, result.Results[1].Error, params.CodeUnauthorized, "permission denied")
	s.assertError(c, result.Results[2].Error, params.CodeUnauthorized, "permission denied")

	// Verify the resource was registered and stop when done
	c.Assert(s.resources.Count(), Equals, 1)
	c.Assert(result.Results[0].NotifyWatcherId, Equals, "1")
	resource := s.resources.Get("1")
	defer statetesting.AssertStop(c, resource)

	// Check that the Watch has consumed the initial event ("returned" in
	// the Watch call)
	wc := statetesting.NewNotifyWatcherC(c, s.State, resource.(state.NotifyWatcher))
	wc.AssertNoChange()
}
