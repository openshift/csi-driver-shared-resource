package config

import (
	"os"
	"testing"
)

func TestValidateSharedSecretOpenShiftName(t *testing.T) {

	for _, test := range []struct {
		name         string
		shareName    string
		refNamespace string
		refName      string
		envVarValue  string
		valid        bool
	}{
		{
			name:         "non openshift",
			shareName:    "anyshare",
			refNamespace: "foo",
			refName:      "bar",
			valid:        true,
		},
		{
			name:         "openshift that is not present",
			shareName:    "openshift-anyshare",
			refNamespace: "foo",
			refName:      "bar",
			valid:        false,
		},
		{
			name:         "openshift that is present but incorrect",
			shareName:    "openshift-anyshare",
			refNamespace: "fee",
			refName:      "bee",
			envVarValue:  "openshift-anyshare: foo:bar",
		},
		{
			name:         "openshift that is present and correct",
			shareName:    "openshift-anyshare",
			refNamespace: "foo",
			refName:      "bar",
			envVarValue:  "openshift-anyshare: foo:bar",
			valid:        true,
		},
	} {
		if len(test.envVarValue) > 0 {
			os.Setenv(sharedSecretReservedNamesEnvVarName, test.envVarValue)
		}
		rn := SetupNameReservation()
		if test.valid != rn.ValidateSharedSecretOpenShiftName(test.shareName, test.refNamespace, test.refName) {
			t.Errorf("test %s did not provide validity of %v", test.name, test.valid)
		}
	}
}

func TestValidateSharedConfigMapOpenShiftName(t *testing.T) {
	for _, test := range []struct {
		name         string
		shareName    string
		refNamespace string
		refName      string
		envVarValue  string
		valid        bool
	}{
		{
			name:         "non openshift",
			shareName:    "anyshare",
			refNamespace: "foo",
			refName:      "bar",
			valid:        true,
		},
		{
			name:         "openshift that is not present",
			shareName:    "openshift-anyshare",
			refNamespace: "foo",
			refName:      "bar",
			valid:        false,
		},
		{
			name:         "openshift that is present but incorrect",
			shareName:    "openshift-anyshare",
			refNamespace: "fee",
			refName:      "bee",
			envVarValue:  "openshift-anyshare: foo:bar",
		},
		{
			name:         "openshift that is present and correct",
			shareName:    "openshift-anyshare",
			refNamespace: "foo",
			refName:      "bar",
			envVarValue:  "openshift-anyshare: foo:bar",
			valid:        true,
		},
	} {
		if len(test.envVarValue) > 0 {
			os.Setenv(sharedConfigMapReservedNamesEnvVarName, test.envVarValue)
		}
		rn := SetupNameReservation()
		if test.valid != rn.ValidateSharedConfigMapOpenShiftName(test.shareName, test.refNamespace, test.refName) {
			t.Errorf("test %s did not provide validity of %v", test.name, test.valid)
		}
	}

}
