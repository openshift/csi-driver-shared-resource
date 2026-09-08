package tls

import (
	"crypto/tls"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

// parseTLSVersion converts TLS version strings to crypto/tls constants.
func parseTLSVersion(v string) (uint16, error) {
	versions := map[string]uint16{
		"VersionTLS10": tls.VersionTLS10,
		"VersionTLS11": tls.VersionTLS11,
		"VersionTLS12": tls.VersionTLS12,
		"VersionTLS13": tls.VersionTLS13,
	}
	if ver, ok := versions[v]; ok {
		return ver, nil
	}
	return 0, fmt.Errorf("unknown TLS version: %s", v)
}

// mapCipherSuites converts OpenSSL-style cipher names (used in OpenShift TLS
// profiles) to Go crypto/tls constants. Ciphers without a Go constant are
// silently skipped.
func mapCipherSuites(names []string) []uint16 {
	// OpenSSL to Go cipher suite name mapping
	// Only includes modern cipher suites with strong integrity (SHA256+) and encryption.
	// SHA-1 and 3DES cipher suites are excluded for security compliance.
	m := map[string]uint16{
		"ECDHE-RSA-AES128-GCM-SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"ECDHE-ECDSA-AES128-GCM-SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"ECDHE-RSA-AES256-GCM-SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		"ECDHE-ECDSA-AES256-GCM-SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		"ECDHE-RSA-CHACHA20-POLY1305":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		"ECDHE-ECDSA-CHACHA20-POLY1305": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		"ECDHE-RSA-AES128-SHA256":       tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		"ECDHE-ECDSA-AES128-SHA256":     tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		"AES128-GCM-SHA256":             tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		"AES256-GCM-SHA384":             tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		"AES128-SHA256":                 tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	}

	out := make([]uint16, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if id, ok := m[name]; ok {
			out = append(out, id)
		} else {
			klog.V(4).Infof("Cipher suite %q not supported by Go, skipping", name)
		}
	}
	return out
}

// BuildTLSConfigFromFlags builds a tls.Config from TLS version and cipher suite flags.
// These flags are injected by the operator based on the cluster TLS profile.
// The returned config has MinVersion and CipherSuites set. CurvePreferences is
// left unset so Go defaults (including ML-KEM) apply.
// Returns an error if non-empty flags contain invalid values that cannot be applied.
func BuildTLSConfigFromFlags(minVersionFlag string, cipherSuitesFlag string) (*tls.Config, error) {
	cfg := &tls.Config{}

	// Parse minimum TLS version
	if minVersionFlag != "" {
		minVer, err := parseTLSVersion(minVersionFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS version %q: %w", minVersionFlag, err)
		}
		cfg.MinVersion = minVer
	} else {
		// Safe default when not configured
		cfg.MinVersion = tls.VersionTLS12
	}

	// Only set cipher suites for TLS 1.2 and below; Go hardcodes TLS 1.3 ciphers
	if cfg.MinVersion < tls.VersionTLS13 && cipherSuitesFlag != "" {
		cipherNames := strings.Split(cipherSuitesFlag, ",")
		suites := mapCipherSuites(cipherNames)
		if len(suites) == 0 {
			// All cipher suites were invalid - fail fast to prevent security policy violation
			return nil, fmt.Errorf("no valid cipher suites found in %q", cipherSuitesFlag)
		}
		cfg.CipherSuites = suites
	}

	// CurvePreferences intentionally left unset to enable ML-KEM support (Go 1.23+)
	return cfg, nil
}
