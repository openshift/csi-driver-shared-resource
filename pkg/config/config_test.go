package config

import (
	"testing"
	"time"
)

func TestConfig_GetShareRelistInterval(t *testing.T) {
	t.Run("arbitrary interval", func(t *testing.T) {
		cfg := NewConfig()
		expectedInterval := 20 * time.Minute
		cfg.ShareRelistInterval = expectedInterval.String()
		if cfg.GetShareRelistInterval() != expectedInterval {
			t.Fail()
		}
	})

	t.Run("bogus interval, expecting default returned", func(t *testing.T) {
		cfg := NewConfig()
		cfg.ShareRelistInterval = "xxxxx"

		if cfg.GetShareRelistInterval() != DefaultResyncDuration {
			t.Fail()
		}
	})
}
