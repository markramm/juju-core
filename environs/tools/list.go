// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package tools

import (
	"launchpad.net/juju-core/log"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/utils/set"
	"launchpad.net/juju-core/version"
	"strings"
)

// List holds tools available in an environment. The order of tools within
// a List is not significant.
type List []*state.Tools

// String returns the versions of the tools in src, separated by semicolons.
func (src List) String() string {
	names := make([]string, len(src))
	for i, tools := range src {
		names[i] = tools.Binary.String()
	}
	return strings.Join(names, ";")
}

// Series returns all series for which some tools in src were built.
func (src List) Series() []string {
	return src.collect(func(tools *state.Tools) string {
		return tools.Series
	})
}

// Arches returns all architectures for which some tools in src were built.
func (src List) Arches() []string {
	return src.collect(func(tools *state.Tools) string {
		return tools.Arch
	})
}

// collect calls f on all values in src and returns an alphabetically
// ordered list of the returned results without duplicates.
func (src List) collect(f func(*state.Tools) string) []string {
	var seen set.Strings
	for _, tools := range src {
		seen.Add(f(tools))
	}
	return seen.SortedValues()
}

// URLs returns download URLs for the tools in src, keyed by binary version.
func (src List) URLs() map[version.Binary]string {
	result := map[version.Binary]string{}
	for _, tools := range src {
		result[tools.Binary] = tools.URL
	}
	return result
}

// Newest returns the greatest version in src, and the tools with that version.
func (src List) Newest() (version.Number, List) {
	var result List
	var best version.Number
	for _, tools := range src {
		if best.Less(tools.Number) {
			// Found new best number; reset result list.
			best = tools.Number
			result = append(result[:0], tools)
		} else if tools.Number == best {
			result = append(result, tools)
		}
	}
	return best, result
}

// Difference returns the tools in src that are not in excluded.
func (src List) Exclude(excluded List) List {
	ignore := make(map[version.Binary]bool, len(excluded))
	for _, tool := range excluded {
		ignore[tool.Binary] = true
	}
	var result List
	for _, tool := range src {
		if !ignore[tool.Binary] {
			result = append(result, tool)
		}
	}
	return result
}

// Match returns a List, derived from src, containing only those tools that
// match the supplied Filter. If no tools match, it returns ErrNoMatches.
func (src List) Match(f Filter) (List, error) {
	var result List
	for _, tools := range src {
		if f.match(tools) {
			result = append(result, tools)
		}
	}
	if len(result) == 0 {
		log.Errorf("environs/tools: cannot match %#v", f)
		return nil, ErrNoMatches
	}
	return result, nil
}

// Filter holds criteria for choosing tools.
type Filter struct {

	// Release, if true, causes the filter to match only tools with a
	// non-development version number.
	Released bool

	// Number, if non-zero, causes the filter to match only tools with
	// that exact version number.
	Number version.Number

	// Series, if not empty, causes the filter to match only tools with
	// that series.
	Series string

	// Arch, if not empty, causes the filter to match only tools with
	// that architecture.
	Arch string
}

// match returns true if the supplied tools match f.
func (f Filter) match(tools *state.Tools) bool {
	if f.Released && tools.IsDev() {
		return false
	}
	if f.Number != version.Zero && tools.Number != f.Number {
		return false
	}
	if f.Series != "" && tools.Series != f.Series {
		return false
	}
	if f.Arch != "" && tools.Arch != f.Arch {
		return false
	}
	return true
}
