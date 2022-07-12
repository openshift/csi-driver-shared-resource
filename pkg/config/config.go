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
	// RefreshResources toggles actively watching for resources, when disabled it will only read
	// resources before mount.
	RefreshResources bool `yaml:"refreshResources,omitempty"`
}

// SharedConfigMaps configuration attributes
type SharedConfigMaps struct {
	// AuthorizedSharedResources has the list of shared configmaps
	// available in the list of OCP pre-populated Shared Resource
	AuthorizedSharedResources map[string]string `yaml:"authorizedSharedResources,omitempty"`
}

// SharedSecrets configuration attributes
type SharedSecrets struct {
	// AuthorizedSharedResources has the list of shared secrets
	// available in the list of OCP pre-populated Shared Resource
	AuthorizedSharedResources map[string]string `yaml:"authorizedSharedResources,omitempty"`
}

var (
	LoadedConfig           Config
	LoadedSharedConfigMaps SharedConfigMaps
	LoadedSharedSecrets    SharedSecrets
)

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
		RefreshResources:    true,
	}
}

// NewSharedSecrets returns a Shared Secrets instance using the default attribute values.
func NewSharedSecrets() SharedSecrets {
	return SharedSecrets{
		AuthorizedSharedResources: map[string]string{
			"openshift-etc-pki-entitlement": "openshift-config-managed:etc-pki-entitlement",
		},
	}
}

// NewSharedConfigMaps returns a Shared ConfigMaps instance using the default attribute values.
func NewSharedConfigMaps() SharedConfigMaps {
	return SharedConfigMaps{
		AuthorizedSharedResources: map[string]string{},
	}
}
