// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"errors"
	"launchpad.net/gnuflag"
	"launchpad.net/juju-core/cmd"
	"launchpad.net/juju-core/juju"
	"launchpad.net/juju-core/state/api/params"
	"launchpad.net/juju-core/state/statecmd"
)

// AddUnitCommand is responsible adding additional units to a service.
type AddUnitCommand struct {
	EnvCommandBase
	ServiceName string
	NumUnits    int
}

func (c *AddUnitCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "add-unit",
		Purpose: "add a service unit",
	}
}

func (c *AddUnitCommand) SetFlags(f *gnuflag.FlagSet) {
	c.EnvCommandBase.SetFlags(f)
	f.IntVar(&c.NumUnits, "n", 1, "number of service units to add")
	f.IntVar(&c.NumUnits, "num-units", 1, "")
}

func (c *AddUnitCommand) Init(args []string) error {
	switch len(args) {
	case 1:
		c.ServiceName = args[0]
	case 0:
		return errors.New("no service specified")
	default:
		return cmd.CheckEmpty(args[1:])
	}
	if c.NumUnits < 1 {
		return errors.New("must add at least one unit")
	}
	return nil
}

// Run connects to the environment specified on the command line
// and calls conn.AddUnits.
func (c *AddUnitCommand) Run(_ *cmd.Context) error {
	conn, err := juju.NewConnFromName(c.EnvName)
	if err != nil {
		return err
	}
	defer conn.Close()

	params := params.AddServiceUnits{
		ServiceName: c.ServiceName,
		NumUnits:    c.NumUnits,
	}
	_, err = statecmd.AddServiceUnits(conn.State, params)
	return err
}
