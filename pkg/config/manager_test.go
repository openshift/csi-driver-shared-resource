package config

import (
	"reflect"
	"testing"
)

// TestConfig_DefaultConfig asserts the default configuration attributes is in use when a
// non-existing configuration file is informed.
func TestConfig_DefaultConfig(t *testing.T) {
	mgr := NewManager("/does/not/exist.yaml")
	cfg, err := mgr.LoadConfig()
	if err != nil {
		t.Fatalf("error '%#v' is not expected when configuration file is not found", err)
	}
	if cfg == nil {
		t.Fatal("configuration instance is nil, expect to find default configuration")
	}

	expectedCfg := NewConfig()
	if !reflect.DeepEqual(&expectedCfg, cfg) {
		t.Fatalf("configuration instance '%#v', is not equal to excepted defaults", cfg)
	}
}

// TestConfig_LocalConfigFile asserts the configuration instance is using the attribute values found
// on the informed configuration file, while the non-defined attributes have default values.
func TestConfig_LocalConfigFile(t *testing.T) {
	mgr := NewManager("../../test/config/config.yaml")
	cfg, err := mgr.LoadConfig()
	if err != nil {
		t.Fatalf("error '%#v' is not expected", err)
	}
	if cfg == nil {
		t.Fatal("configuration instance is nil")
	}

	expectedCfg := NewConfig()
	expectedCfg.RefreshResources = false
	expectedCfg.ShareRelistInterval = "20m"
	if !reflect.DeepEqual(&expectedCfg, cfg) {
		t.Fatalf("configuration instance '%#v', is not equal to excepted", cfg)
	}
}

// TestConfig_ConfigHasChanged asserts the configuration payload is probed, and changes can be
// detected based on MD5 signature.
func TestConfig_ConfigHasChanged(t *testing.T) {
	mgr := NewManager("../../test/config/config.yaml")
	_, err := mgr.LoadConfig()
	if err != nil {
		t.Fatalf("error '%#v' is not expected", err)
	}
	if mgr.md5sum == "" {
		t.Fatal("after loading configuration the md5sum attribute is still empty")
	}

	if mgr.ConfigHasChanged() {
		t.Fatal("expect configuration to not have changed")
	}

	mgr.md5sum = "bogus-sum"
	if !mgr.ConfigHasChanged() {
		t.Fatal("expect configuration to have changed")
	}
}
