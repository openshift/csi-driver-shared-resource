package tls

import (
	cryptotls "crypto/tls"
	"reflect"
	"testing"
)

func TestParseCurvePreferences(t *testing.T) {
	tests := []struct {
		name       string
		curveNames []string
		want       []cryptotls.CurveID
	}{
		{
			name:       "OCP default profile groups (Old/Intermediate/Modern)",
			curveNames: []string{"X25519MLKEM768", "X25519", "secp256r1", "secp384r1"},
			want:       []cryptotls.CurveID{cryptotls.X25519MLKEM768, cryptotls.X25519, cryptotls.CurveP256, cryptotls.CurveP384},
		},
		{
			name:       "X25519 and ML-KEM hybrid",
			curveNames: []string{"X25519", "X25519MLKEM768"},
			want:       []cryptotls.CurveID{cryptotls.X25519, cryptotls.X25519MLKEM768},
		},
		{
			name:       "NIST P-curves",
			curveNames: []string{"CurveP256", "CurveP384", "CurveP521"},
			want:       []cryptotls.CurveID{cryptotls.CurveP256, cryptotls.CurveP384, cryptotls.CurveP521},
		},
		{
			name:       "OCP TLSGroup enum names (secp*)",
			curveNames: []string{"secp256r1", "secp384r1", "secp521r1"},
			want:       []cryptotls.CurveID{cryptotls.CurveP256, cryptotls.CurveP384, cryptotls.CurveP521},
		},
		{
			name:       "legacy P-curve names",
			curveNames: []string{"P-256", "P-384", "P-521"},
			want:       []cryptotls.CurveID{cryptotls.CurveP256, cryptotls.CurveP384, cryptotls.CurveP521},
		},
		{
			name:       "mixed formats",
			curveNames: []string{"X25519", "P-256", "CurveP384"},
			want:       []cryptotls.CurveID{cryptotls.X25519, cryptotls.CurveP256, cryptotls.CurveP384},
		},
		{
			name:       "with whitespace",
			curveNames: []string{" X25519 ", " P-256", "CurveP384 "},
			want:       []cryptotls.CurveID{cryptotls.X25519, cryptotls.CurveP256, cryptotls.CurveP384},
		},
		{
			name:       "invalid curve is skipped",
			curveNames: []string{"X25519", "InvalidCurve", "P-256"},
			want:       []cryptotls.CurveID{cryptotls.X25519, cryptotls.CurveP256},
		},
		{
			name:       "all invalid curves returns empty",
			curveNames: []string{"InvalidCurve1", "InvalidCurve2"},
			want:       []cryptotls.CurveID{},
		},
		{
			name:       "empty input returns empty",
			curveNames: []string{},
			want:       []cryptotls.CurveID{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCurvePreferences(tt.curveNames)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCurvePreferences() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildTLSConfigFromFlags_WithCurves(t *testing.T) {
	tests := []struct {
		name                 string
		minVersionFlag       string
		cipherSuitesFlag     string
		curvePreferencesFlag string
		wantCurves           []cryptotls.CurveID
		wantErr              bool
	}{
		{
			name:                 "curve preferences set",
			minVersionFlag:       "VersionTLS13",
			cipherSuitesFlag:     "",
			curvePreferencesFlag: "X25519,X25519MLKEM768,P-256",
			wantCurves:           []cryptotls.CurveID{cryptotls.X25519, cryptotls.X25519MLKEM768, cryptotls.CurveP256},
		},
		{
			name:                 "empty curve preferences allowed for backwards compat (OCP < 4.22)",
			minVersionFlag:       "VersionTLS13",
			cipherSuitesFlag:     "",
			curvePreferencesFlag: "",
			wantCurves:           nil, // Go defaults apply
		},
		{
			name:                 "ML-KEM only for PQC readiness",
			minVersionFlag:       "VersionTLS13",
			cipherSuitesFlag:     "",
			curvePreferencesFlag: "X25519MLKEM768",
			wantCurves:           []cryptotls.CurveID{cryptotls.X25519MLKEM768},
		},
		{
			name:                 "invalid curves are skipped with warning",
			minVersionFlag:       "VersionTLS13",
			cipherSuitesFlag:     "",
			curvePreferencesFlag: "X25519,InvalidCurve,P-256",
			wantCurves:           []cryptotls.CurveID{cryptotls.X25519, cryptotls.CurveP256},
		},
		{
			name:                 "all invalid curves returns error (no valid curves to apply)",
			minVersionFlag:       "VersionTLS13",
			cipherSuitesFlag:     "",
			curvePreferencesFlag: "InvalidCurve1,InvalidCurve2",
			wantErr:              true, // Error because flag was set but no valid curves found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := BuildTLSConfigFromFlags(tt.minVersionFlag, tt.cipherSuitesFlag, tt.curvePreferencesFlag)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(cfg.CurvePreferences, tt.wantCurves) {
				t.Errorf("CurvePreferences = %v, want %v", cfg.CurvePreferences, tt.wantCurves)
			}
		})
	}
}
