package juju_test

import (
	"io/ioutil"
	. "launchpad.net/gocheck"
	"launchpad.net/juju/go/juju"
	"os"
	"path/filepath"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type ConnSuite struct{}

var _ = Suite(&ConnSuite{})

func (s *ConnSuite) TestNewConn(c *C) {
	home := c.MkDir()
	defer os.Setenv("HOME", os.Getenv("HOME"))
	os.Setenv("HOME", home)
	conn, err := juju.NewConn("")
	c.Assert(conn, IsNil)
	c.Assert(err, ErrorMatches, ".*: no such file or directory")

	if err := os.Mkdir(filepath.Join(home, ".juju"), 0755); err != nil {
		c.Log("Could not create directory structure")
		c.Fail()
	}
	envs := filepath.Join(home, ".juju", "environments.yaml")
	err = ioutil.WriteFile(envs, []byte(`
default:
    erewhemos
environments:
    erewhemos:
        type: dummy
`), 0644)
	if err != nil {
		c.Log("Could not create environments.yaml")
		c.Fail()
	}

	// Tests current behaviour, not intended behaviour: once we have a
	// globally-registered dummy provider, we'll expect to get a non-nil
	// Conn back, and will have to figure out what needs to be tested on that.
	conn, err = juju.NewConn("")
	c.Assert(err, ErrorMatches, `environment "erewhemos" has an unknown provider type: "dummy"`)
	c.Assert(conn, IsNil)
}