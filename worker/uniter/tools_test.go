package uniter_test

import (
	"io/ioutil"
	. "launchpad.net/gocheck"
	"launchpad.net/juju-core/cmd/jujuc/server"
	"launchpad.net/juju-core/environs"
	"launchpad.net/juju-core/worker/uniter"
	"os"
	"path/filepath"
	"time"
)

type ToolsSuite struct {
	varDir, oldVarDir, toolsDir string
}

var _ = Suite(&ToolsSuite{})

func (s *ToolsSuite) SetUpTest(c *C) {
	s.oldVarDir = environs.VarDir
	s.varDir = c.MkDir()
	environs.VarDir = s.varDir
	s.toolsDir = c.MkDir()
	toolsDir := filepath.Join(s.varDir, "tools")
	err := os.Mkdir(toolsDir, 0755)
	c.Assert(err, IsNil)
	err = os.Symlink(s.toolsDir, filepath.Join(toolsDir, "unit-u-123"))
	c.Assert(err, IsNil)
}

func (s *ToolsSuite) TearDownTest(c *C) {
	environs.VarDir = s.oldVarDir
}

func (s *ToolsSuite) TestEnsureTools(c *C) {
	jujuc := filepath.Join(s.toolsDir, "jujuc")
	err := ioutil.WriteFile(jujuc, []byte("assume sane"), 0755)
	c.Assert(err, IsNil)

	assertLink := func(path string) time.Time {
		target, err := os.Readlink(path)
		c.Assert(err, IsNil)
		c.Assert(target, Equals, "./jujuc")
		fi, err := os.Lstat(path)
		c.Assert(err, IsNil)
		return fi.ModTime()
	}

	// Check that EnsureTools writes appropriate symlinks.
	err = uniter.EnsureTools("u/123")
	c.Assert(err, IsNil)
	mtimes := map[string]time.Time{}
	for _, name := range server.CommandNames() {
		tool := filepath.Join(s.toolsDir, name)
		mtimes[tool] = assertLink(tool)
	}

	// Check that EnsureTools doesn't overwrite things that don't need to be.
	err = uniter.EnsureTools("u/123")
	c.Assert(err, IsNil)
	for tool, mtime := range mtimes {
		c.Assert(assertLink(tool), Equals, mtime)
	}
}

func (s *ToolsSuite) TestEnsureToolsBadDir(c *C) {
	err := uniter.EnsureTools("u/999")
	c.Assert(err, ErrorMatches, "cannot initialize hook commands.*: no such file or directory")
}