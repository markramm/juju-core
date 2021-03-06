// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package local

import (
	"fmt"
	"path/filepath"

	"launchpad.net/loggo"

	"launchpad.net/juju-core/environs"
	"launchpad.net/juju-core/environs/config"
	"launchpad.net/juju-core/instance"
	"launchpad.net/juju-core/utils"
)

var logger = loggo.GetLogger("juju.environs.local")

var _ environs.EnvironProvider = (*environProvider)(nil)

type environProvider struct{}

var provider environProvider

func init() {
	environs.RegisterProvider("local", &environProvider{})
}

var (
	defaultRootDir = "/var/lib/juju/"
)

// Open implements environs.EnvironProvider.Open.
func (environProvider) Open(cfg *config.Config) (environs.Environ, error) {
	logger.Infof("opening environment %q", cfg.Name())
	environ := &localEnviron{name: cfg.Name()}
	err := environ.SetConfig(cfg)
	if err != nil {
		logger.Errorf("failure setting config: %v", err)
		return nil, err
	}
	return environ, nil
}

// Validate implements environs.EnvironProvider.Validate.
func (provider environProvider) Validate(cfg, old *config.Config) (valid *config.Config, err error) {
	// Check for valid changes for the base config values.
	if err := config.Validate(cfg, old); err != nil {
		return nil, err
	}
	v, err := configChecker.Coerce(cfg.UnknownAttrs(), nil)
	if err != nil {
		return nil, err
	}
	localConfig := newEnvironConfig(cfg, v.(map[string]interface{}))
	// Before potentially creating directories, make sure that the
	// root directory has not changed.
	if old != nil {
		oldLocalConfig, err := provider.newConfig(old)
		if err != nil {
			return nil, fmt.Errorf("old config is not a valid local config: %v", old)
		}
		if localConfig.rootDir() != oldLocalConfig.rootDir() {
			return nil, fmt.Errorf("cannot change root-dir from %q to %q",
				oldLocalConfig.rootDir(),
				localConfig.rootDir())
		}
	}
	dir := utils.NormalizePath(localConfig.rootDir())
	if dir == "." {
		dir = filepath.Join(defaultRootDir, localConfig.namespace())
		localConfig.attrs["root-dir"] = dir
	}

	// Apply the coerced unknown values back into the config.
	return cfg.Apply(localConfig.attrs)
}

// BoilerplateConfig implements environs.EnvironProvider.BoilerplateConfig.
func (environProvider) BoilerplateConfig() string {
	return `
## https://juju.ubuntu.com/get-started/local/
local:
  type: local
  # Override the directory that is used for the storage files and database.
  # The default location is /var/lib/juju/<USER>-<ENV>
  # root-dir: ~/.juju/local

`[1:]
}

// SecretAttrs implements environs.EnvironProvider.SecretAttrs.
func (environProvider) SecretAttrs(cfg *config.Config) (map[string]interface{}, error) {
	// don't have any secret attrs
	return nil, nil
}

// Location specific methods that are able to be called by any instance that
// has been created by this provider type.  So a machine agent may well call
// these methods to find out its own address or instance id.

// PublicAddress implements environs.EnvironProvider.PublicAddress.
func (environProvider) PublicAddress() (string, error) {
	return "", fmt.Errorf("not implemented")
}

// PrivateAddress implements environs.EnvironProvider.PrivateAddress.
func (environProvider) PrivateAddress() (string, error) {
	return "", fmt.Errorf("not implemented")
}

// InstanceId implements environs.EnvironProvider.InstanceId.
func (environProvider) InstanceId() (instance.Id, error) {
	return "", fmt.Errorf("not implemented")
}

func (environProvider) newConfig(cfg *config.Config) (*environConfig, error) {
	valid, err := provider.Validate(cfg, nil)
	if err != nil {
		return nil, err
	}
	return newEnvironConfig(valid, valid.UnknownAttrs()), nil
}
