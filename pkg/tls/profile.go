package tls

import (
	"crypto/tls"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

// parseTLSVersion converts TLS version strings to crypto/tls constants.
// Accepts both short forms (1.0-1.3) and long forms (VersionTLS10-13) for compatibility.
func parseTLSVersion(v string) (uint16, error) {
	versions := map[string]uint16{
		// Short forms (like Shipwright)
		"1.0": tls.VersionTLS10,
		"1.1": tls.VersionTLS11,
		"1.2": tls.VersionTLS12,
		"1.3": tls.VersionTLS13,
		// Long forms
		"VersionTLS10": tls.VersionTLS10,
		"VersionTLS11": tls.VersionTLS11,
		"VersionTLS12": tls.VersionTLS12,
		"VersionTLS13": tls.VersionTLS13,
	}
	if ver, ok := versions[v]; ok {
		return ver, nil
	}
	return 0, fmt.Errorf("invalid --tls-min-version %q (allowed: 1.0-1.3 or VersionTLS10-13)", v)
}

// parseCurvePreferences converts curve names to crypto/tls.CurveID constants.
// Supports standard ECDHE curves and post-quantum hybrid curves (ML-KEM).
// Accepts TLSGroup enum values from openshift/api (secp*) as well as Go names (CurveP*).
func parseCurvePreferences(curveNames []string) []tls.CurveID {
	curveMap := map[string]tls.CurveID{
		// OCP TLSGroup enum values (from openshift/api PR #2583)
		"X25519":         tls.X25519,
		"X25519MLKEM768": tls.X25519MLKEM768, // PQC hybrid, not FIPS-approved
		"secp256r1":      tls.CurveP256,
		"secp384r1":      tls.CurveP384,
		"secp521r1":      tls.CurveP521,
		// Go crypto/tls names
		"CurveP256": tls.CurveP256,
		"CurveP384": tls.CurveP384,
		"CurveP521": tls.CurveP521,
		// Legacy/alternative names for compatibility
		"P-256": tls.CurveP256,
		"P-384": tls.CurveP384,
		"P-521": tls.CurveP521,
	}

	out := make([]tls.CurveID, 0, len(curveNames))
	for _, name := range curveNames {
		name = strings.TrimSpace(name)
		if id, ok := curveMap[name]; ok {
			out = append(out, id)
		} else {
			klog.Warningf("Curve %q not recognized, skipping", name)
		}
	}
	return out
}

// mapCipherSuites converts IANA cipher names (standard Kubernetes format)
// to Go crypto/tls constants. Only secure ciphers are accepted - insecure ciphers
// are logged and skipped. Uses dynamic mapping from Go's runtime.
func mapCipherSuites(names []string) []uint16 {
	// Build map from secure cipher suites only (excludes InsecureCipherSuites)
	m := make(map[string]uint16, 64)
	for _, cs := range tls.CipherSuites() {
		m[cs.Name] = cs.ID
	}

	out := make([]uint16, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if id, ok := m[name]; ok {
			out = append(out, id)
		} else {
			klog.Warningf("Cipher suite %q not supported by Go, skipping", name)
		}
	}
	return out
}

// BuildTLSConfigFromFlags builds a tls.Config from TLS profile flags.
// These flags are injected by the operator based on the cluster TLS profile
// (apiserver.config.openshift.io/v1 APIServer.Spec.TLSSecurityProfile)
// to ensure strict adherence to centrally-managed security policies for PQC readiness.
//
// The operator MUST always inject minVersionFlag based on the cluster's TLS profile.
//
// The curvePreferencesFlag is optional and corresponds to TLSProfileSpec.Groups
// (added in OCP 4.20+ with TLSGroupPreferences feature gate, openshift/api PR #2583).
// When omitted, this means "no opinion" per the API design - Go's default curve
// preferences apply (which includes ML-KEM post-quantum curves in Go 1.23+).
// When provided, it's used as a filter of allowed groups (Go's internal preference
// order still applies, as documented in the TLSProfile API).
//
// Returns an error if required flags are missing or contain invalid values.
func BuildTLSConfigFromFlags(minVersionFlag, cipherSuitesFlag, curvePreferencesFlag string) (*tls.Config, error) {
	cfg := &tls.Config{}

	// Parse minimum TLS version - REQUIRED for strict TLS adherence
	if minVersionFlag == "" {
		return nil, fmt.Errorf("--tls-min-version is required (must be injected by operator from cluster TLS profile)")
	}
	minVer, err := parseTLSVersion(minVersionFlag)
	if err != nil {
		return nil, err
	}
	cfg.MinVersion = minVer

	// Only set cipher suites for TLS 1.2 and below; Go hardcodes TLS 1.3 ciphers
	if cfg.MinVersion < tls.VersionTLS13 && cipherSuitesFlag != "" {
		cipherNames := strings.Split(cipherSuitesFlag, ",")
		suites := mapCipherSuites(cipherNames)
		if len(suites) == 0 {
			// All cipher suites were invalid - fail fast to prevent security policy violation
			return nil, fmt.Errorf("no valid cipher suites found in %q", cipherSuitesFlag)
		}

		// Validate HTTP/2 cipher suite requirement
		hasHTTP2Cipher := false
		for _, suite := range suites {
			if suite == tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 ||
				suite == tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256 {
				hasHTTP2Cipher = true
				break
			}
		}
		if !hasHTTP2Cipher {
			return nil, fmt.Errorf("cipher suite list must include TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 or TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256 for HTTP/2 support")
		}

		cfg.CipherSuites = suites
	} else if cfg.MinVersion >= tls.VersionTLS13 && cipherSuitesFlag != "" {
		klog.Infof("TLS 1.3+ configured, cipher suite list %q will be ignored (Go uses built-in TLS 1.3 ciphers)", cipherSuitesFlag)
	}

	// Parse curve preferences for strict TLS adherence (OCP 4.22+)
	// Curves are REQUIRED in Strict mode (OCP 5.0+) but optional for backwards
	// compatibility with OCP < 4.22 where TLSCurvePreferences didn't exist.
	// The operator handles version-specific logic:
	//   - OCP < 4.22: Don't inject (profile has no curves) → Go defaults apply
	//   - OCP 4.22+: Inject from profile if present → explicit curves used
	//   - OCP 5.0+ Strict: MUST inject from profile → no Go defaults
	if curvePreferencesFlag != "" {
		curveNames := strings.Split(curvePreferencesFlag, ",")
		curves := parseCurvePreferences(curveNames)
		if len(curves) == 0 {
			return nil, fmt.Errorf("no valid curves found in %q (check cluster TLS profile configuration)", curvePreferencesFlag)
		}
		cfg.CurvePreferences = curves
	}
	// If empty, leave unset (Go defaults apply) for backwards compatibility
	// with OCP < 4.22. In OCP 5.0+ Strict mode, operator MUST inject curves.

	return cfg, nil
}
