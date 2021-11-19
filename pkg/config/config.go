package config

import (
	"time"

	"k8s.io/klog/v2"
)

const DefaultResyncDuration = 10 * time.Minute

// Config configuration attributes.
type Config struct {
	// ShareRelistInterval interval to relist all "Share" object instances.
	ShareRelistInterval string `yaml:"shareRelistInterval,omitempty"`
	// IgnoredNamespaces namespace names ignored by this controller.
	IgnoredNamespaces []string `yaml:"ignoredNamespaces,omitempty"`
	// RefreshResources toggles actively watching for resources, when disabled it will only read
	// resources before mount.
	RefreshResources bool `yaml:"refreshResources,omitempty"`
}

var LoadedConfig Config

// GetShareRelistInterval returns the ShareRelistInterval value as duration. On error, default value
// is employed instead.
func (c *Config) GetShareRelistInterval() time.Duration {
	resyncDuration, err := time.ParseDuration(c.ShareRelistInterval)
	if err != nil {
		klog.Errorf("Error on parsing ShareRelistInterval '%s': %s", c.ShareRelistInterval, err)
		return DefaultResyncDuration
	}
	return resyncDuration
}

// NewConfig returns a Config instance using the default attribute values.
func NewConfig() Config {
	return Config{
		ShareRelistInterval: DefaultResyncDuration.String(),
		IgnoredNamespaces:   []string{},
		RefreshResources:    true,
	}
}
