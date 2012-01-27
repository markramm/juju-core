// launchpad.net/juju/state
//
// Copyright (c) 2011-2012 Canonical Ltd.

package state

import (
	"fmt"
	"launchpad.net/goyaml"
	"launchpad.net/gozk/zookeeper"
	"sort"
)

// The protocol version, which is stored in the /topology node under
// the "version" key. The protocol version should *only* be updated
// when we know that a version is in fact actually incompatible.
const topologyVersion = 1

// zkTopology is used to marshal and unmarshal the content
// of the /topology node in ZooKeeper.
type zkTopology struct {
	Version      int
	Services     map[string]*zkService
	UnitSequence map[string]int "unit-sequence"
}

// zkService represents the service data within the /topology
// node in ZooKeeper.
type zkService struct {
	Name  string
	Units map[string]*zkUnit
}

// zkUnit represents the unit data within the /topology
// node in ZooKeeper.
type zkUnit struct {
	Sequence int
	Machine  string
}

// topology is an internal helper that handles the content
// of the /topology node in ZooKeeper.
type topology struct {
	topology *zkTopology
}

// readTopology connects ZooKeeper, retrieves the data as YAML,
// parses it and returns it.
func readTopology(zk *zookeeper.Conn) (*topology, error) {
	yaml, _, err := zk.Get("/topology")
	if err != nil {
		return nil, err
	}
	return parseTopology(yaml)
}

// dump returns the topology as YAML.
func (t *topology) dump() (string, error) {
	topologyYaml, err := goyaml.Marshal(t.topology)
	if err != nil {
		return "", err
	}
	return string(topologyYaml), nil
}

// version returns the version of the topology.
func (t *topology) version() int {
	return t.topology.Version
}

// hasService returns true if a service with the given key exists.
func (t *topology) hasService(key string) bool {
	return t.topology.Services[key] != nil
}

// serviceKey returns the key of the service with the given name.
func (t *topology) serviceKey(name string) (string, error) {
	for key, svc := range t.topology.Services {
		if svc.Name == name {
			return key, nil
		}
	}
	return "", fmt.Errorf("service with name %q cannot be found", name)
}

// hasUnit returns true if a unit with given service and unit keys exists.
func (t *topology) hasUnit(serviceKey, unitKey string) bool {
	if t.hasService(serviceKey) {
		return t.topology.Services[serviceKey].Units[unitKey] != nil
	}
	return false
}

// addUnit adds a new unit and returns the sequence number. This
// sequence number will be increased monotonically for each service.
func (t *topology) addUnit(serviceKey, unitKey string) (int, error) {
	if err := t.assertService(serviceKey); err != nil {
		return -1, err
	}
	// Check if unit key is unused.
	for key, svc := range t.topology.Services {
		if _, ok := svc.Units[unitKey]; ok {
			return -1, fmt.Errorf("unit %q already in use in servie %q", unitKey, key)
		}
	}
	// Add unit and increase sequence number.
	svc := t.topology.Services[serviceKey]
	sequenceNo := t.topology.UnitSequence[svc.Name]
	svc.Units[unitKey] = &zkUnit{Sequence: sequenceNo}
	t.topology.UnitSequence[svc.Name] += 1
	return sequenceNo, nil
}

// removeUnit removes a unit from a service.
func (t *topology) removeUnit(serviceKey, unitKey string) error {
	if err := t.assertUnit(serviceKey, unitKey); err != nil {
		return err
	}
	delete(t.topology.Services[serviceKey].Units, unitKey)
	return nil
}

// unitKeys returns the unit keys for all units of
// the service with the given service key in alphabetical order.
func (t *topology) unitKeys(serviceKey string) ([]string, error) {
	if err := t.assertService(serviceKey); err != nil {
		return nil, err
	}
	keys := []string{}
	svc := t.topology.Services[serviceKey]
	for key, _ := range svc.Units {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, nil
}

// unitName returns the name of a unit by its service key and its own key.
func (t *topology) unitName(serviceKey, unitKey string) (string, error) {
	if err := t.assertUnit(serviceKey, unitKey); err != nil {
		return "", err
	}
	svc := t.topology.Services[serviceKey]
	unit := svc.Units[unitKey]
	return fmt.Sprintf("%s/%d", svc.Name, unit.Sequence), nil
}

// unitKeyFromSequence returns the key of a unit by the its service key
// and its sequence number.
func (t *topology) unitKeyFromSequence(serviceKey string, sequenceNo int) (string, error) {
	if err := t.assertService(serviceKey); err != nil {
		return "", err
	}
	svc := t.topology.Services[serviceKey]
	for key, unit := range svc.Units {
		if unit.Sequence == sequenceNo {
			return key, nil
		}
	}
	return "", fmt.Errorf("unit with sequence number %d cannot be found", sequenceNo)
}

// unitMachineKey returns the key of an assigned machine of the unit. An empty
// key means there is no machine assigned.
func (t *topology) unitMachineKey(serviceKey, unitKey string) (string, error) {
	if err := t.assertUnit(serviceKey, unitKey); err != nil {
		return "", err
	}
	unit := t.topology.Services[serviceKey].Units[unitKey]
	return unit.Machine, nil
}

// unassignUnitFromMachine unassigns the unit from its current machine.
func (t *topology) unassignUnitFromMachine(serviceKey, unitKey string) error {
	if err := t.assertUnit(serviceKey, unitKey); err != nil {
		return err
	}
	unit := t.topology.Services[serviceKey].Units[unitKey]
	if unit.Machine == "" {
		return fmt.Errorf("unit %q in service %q is not assigned to a machine", unitKey, serviceKey)
	}
	unit.Machine = ""
	return nil
}

// assertService checks if a service exists.
func (t *topology) assertService(serviceKey string) error {
	if _, ok := t.topology.Services[serviceKey]; !ok {
		return fmt.Errorf("service with key %q cannot be found", serviceKey)
	}
	return nil
}

// assertUnit checks if a service with a unit exists.
func (t *topology) assertUnit(serviceKey, unitKey string) error {
	if err := t.assertService(serviceKey); err != nil {
		return err
	}
	svc := t.topology.Services[serviceKey]
	if _, ok := svc.Units[unitKey]; !ok {
		return fmt.Errorf("unit with key %q cannot be found", unitKey)
	}
	return nil
}

// parseTopology returns the topology represented by yaml.
func parseTopology(yaml string) (*topology, error) {
	t := &topology{topology: &zkTopology{Version: topologyVersion}}
	if err := goyaml.Unmarshal([]byte(yaml), t.topology); err != nil {
		return nil, err
	}
	if t.topology.Version != topologyVersion {
		return nil, fmt.Errorf("incompatible topology versions: got %d, want %d", t.topology.Version, topologyVersion)
	}
	return t, nil
}

// retryTopologyChange tries to change the topology with f.
// This function can read and modify the topology instance, 
// and after it returns the modified topology will be
// persisted into the /topology node. Note that this f must
// have no side-effects, since it may be called multiple times
// depending on conflict situations.
func retryTopologyChange(zk *zookeeper.Conn, f func(t *topology) error) error {
	change := func(yaml string, stat *zookeeper.Stat) (string, error) {
		var err error
		it := &topology{topology: &zkTopology{Version: 1}}
		if yaml != "" {
			if it, err = parseTopology(yaml); err != nil {
				return "", err
			}
		}
		// Apply the passed function.
		if err = f(it); err != nil {
			return "", err
		}
		return it.dump()
	}
	return zk.RetryChange("/topology", 0, zookeeper.WorldACL(zookeeper.PERM_ALL), change)
}