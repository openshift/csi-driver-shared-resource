package config

import (
	"crypto/md5"
	"encoding/hex"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

// Manager controls the configuration file loading, and can assert if it has changed on disk.
type Manager struct {
	cfgFilePath string // path to configuration file
	md5sum      string // md5sum of the initial content
}

// ConfigHasChanged checks the current configuration contents, comparing with that it has been
// instantiated with.
func (m *Manager) ConfigHasChanged() bool {
	// given the md5sum is not yet set, the configuration payload won't be marked as changed
	if m.md5sum == "" {
		return false
	}

	// reading the configration file payload again and comparing with the md5sum stored, when there
	// are errors reading the file, it does not mark the configuration as changed
	payload, err := ioutil.ReadFile(m.cfgFilePath)
	if err != nil {
		klog.Errorf("Reading configuration-file '%s': '%#v'", m.cfgFilePath, err)
		return false
	}
	sum := md5.Sum(payload)
	return m.md5sum != hex.EncodeToString(sum[:])
}

// LoadConfig read the local configuration file, make sure the current contents are summed, so we can
// assert if there are changes later on.
func (m *Manager) LoadConfig() (*Config, error) {
	cfg := NewConfig()

	if _, err := os.Stat(m.cfgFilePath); os.IsNotExist(err) {
		klog.Info("Configuration file is not found, using default values!")
		return &cfg, nil
	}

	// in the case of issues to read the mounted file, and in case of errors marshaling to the
	// destination struct, this method will surface those errors directly, and we may want to create
	// means to differentiate the error scenarios
	klog.Infof("Loading configuration-file '%s'", m.cfgFilePath)
	payload, err := ioutil.ReadFile(m.cfgFilePath)
	if err != nil {
		return nil, err
	}
	sum := md5.Sum(payload)
	m.md5sum = hex.EncodeToString(sum[:])

	// overwriting attributes found on the configuration file with the defaults
	if err = yaml.Unmarshal(payload, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// NewManager instantiate the manager.
func NewManager(cfgFilePath string) *Manager {
	return &Manager{cfgFilePath: cfgFilePath}
}
