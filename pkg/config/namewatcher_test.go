package config

import (
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/types"
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
			valid:        false,
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

func TestParseEnvVar(t *testing.T) {
	for _, test := range []struct {
		name        string
		envVarValue string
		expectedMap map[string]types.NamespacedName
	}{
		{
			name:        "empty",
			envVarValue: "",
			expectedMap: map[string]types.NamespacedName{},
		},
		{
			name:        "one entry",
			envVarValue: "openshift-foo: foo:bar",
			expectedMap: map[string]types.NamespacedName{
				"openshift-foo": {
					Namespace: "foo",
					Name:      "bar",
				},
			},
		},
		{
			name:        "two entries",
			envVarValue: "openshift-foo: foo:bar; openshift-bar: bar:foo",
			expectedMap: map[string]types.NamespacedName{
				"openshift-foo": {
					Namespace: "foo",
					Name:      "bar",
				},
				"openshift-bar": {
					Namespace: "bar",
					Name:      "foo",
				},
			},
		},
		{
			name:        "bad one entry",
			envVarValue: "openshift-foo: ",
			expectedMap: map[string]types.NamespacedName{},
		},
		{
			name:        "bad two entries",
			envVarValue: "openshift-foo: foo:bar, openshift-bar: bar:foo",
			expectedMap: map[string]types.NamespacedName{},
		},
	} {
		retMap := parseEnvVar(test.envVarValue, "")
		if len(retMap) != len(test.expectedMap) {
			t.Errorf("test %s envVarValue %s got map %#v with different number of entries than %s",
				test.name,
				test.envVarValue,
				retMap,
				test.expectedMap)
			continue
		}
		for key, nsName := range test.expectedMap {
			v, ok := retMap[key]
			if !ok {
				t.Errorf("test %s envVarValue %s return map %s missing key %s",
					test.name,
					test.envVarValue,
					retMap,
					key)
				continue
			}
			if nsName.String() != v.String() {
				t.Errorf("test %s envVarValue %s return map %s key %s had value %s instead of %s",
					test.name,
					test.envVarValue,
					retMap,
					key,
					v,
					nsName)
				continue
			}
		}
	}
}

func TestStartsWithOpenshift(t *testing.T) {
	for _, test := range []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "empty",
			input:    "",
			expected: false,
		},
		{
			name:     "non-openshift",
			input:    "foo",
			expected: false,
		},
		{
			name:     "openshift hyphen",
			input:    "openshift-foo",
			expected: true,
		},
		{
			name:     "openshift underscore",
			input:    "openshift_foo",
			expected: true,
		},
		{
			name:     "just openshift",
			input:    "openshift",
			expected: true,
		},
		{
			name:     "case sensitive",
			input:    "OpenShift",
			expected: false,
		},
	} {
		output := startsWithOpenShift(test.input)
		if output != test.expected {
			t.Errorf("test %s with input %s got %v instead of %v",
				test.name,
				test.input,
				output,
				test.expected)
		}
	}
}
