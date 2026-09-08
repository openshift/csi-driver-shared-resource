package tls

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	cryptotls "crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestBuildTLSConfigFromFlags(t *testing.T) {
	tests := []struct {
		name                string
		minVersionFlag      string
		cipherSuitesFlag    string
		wantMinVersion      uint16
		wantCipherSuites    []uint16
		wantCipherSuitesNil bool
		wantErr             bool
	}{
		{
			name:                "empty flags defaults to TLS 1.2",
			minVersionFlag:      "",
			cipherSuitesFlag:    "",
			wantMinVersion:      cryptotls.VersionTLS12,
			wantCipherSuitesNil: true,
		},
		{
			name:             "TLS 1.2 with cipher suites",
			minVersionFlag:   "VersionTLS12",
			cipherSuitesFlag: "ECDHE-RSA-AES128-GCM-SHA256,ECDHE-ECDSA-AES128-GCM-SHA256",
			wantMinVersion:   cryptotls.VersionTLS12,
			wantCipherSuites: []uint16{cryptotls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, cryptotls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256},
		},
		{
			name:                "TLS 1.3 ignores cipher suites",
			minVersionFlag:      "VersionTLS13",
			cipherSuitesFlag:    "ECDHE-RSA-AES128-GCM-SHA256",
			wantMinVersion:      cryptotls.VersionTLS13,
			wantCipherSuitesNil: true,
		},
		{
			name:                "TLS 1.0",
			minVersionFlag:      "VersionTLS10",
			cipherSuitesFlag:    "",
			wantMinVersion:      cryptotls.VersionTLS10,
			wantCipherSuitesNil: true,
		},
		{
			name:                "TLS 1.1",
			minVersionFlag:      "VersionTLS11",
			cipherSuitesFlag:    "",
			wantMinVersion:      cryptotls.VersionTLS11,
			wantCipherSuitesNil: true,
		},
		{
			name:             "unknown version returns error",
			minVersionFlag:   "VersionTLS99",
			cipherSuitesFlag: "",
			wantErr:          true,
		},
		{
			name:             "multiple cipher suites with spaces",
			minVersionFlag:   "VersionTLS12",
			cipherSuitesFlag: "ECDHE-RSA-AES128-GCM-SHA256, ECDHE-ECDSA-AES256-GCM-SHA384, ECDHE-RSA-CHACHA20-POLY1305",
			wantMinVersion:   cryptotls.VersionTLS12,
			wantCipherSuites: []uint16{cryptotls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, cryptotls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, cryptotls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305},
		},
		{
			name:             "unsupported cipher suites are skipped",
			minVersionFlag:   "VersionTLS12",
			cipherSuitesFlag: "ECDHE-RSA-AES128-GCM-SHA256,UNSUPPORTED-CIPHER,ECDHE-ECDSA-AES128-GCM-SHA256",
			wantMinVersion:   cryptotls.VersionTLS12,
			wantCipherSuites: []uint16{cryptotls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, cryptotls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256},
		},
		{
			name:             "all invalid cipher suites returns error",
			minVersionFlag:   "VersionTLS12",
			cipherSuitesFlag: "INVALID-CIPHER-1,INVALID-CIPHER-2",
			wantErr:          true,
		},
		{
			name:             "weak SHA-1 cipher suites are rejected with error",
			minVersionFlag:   "VersionTLS12",
			cipherSuitesFlag: "ECDHE-RSA-AES128-SHA,ECDHE-ECDSA-AES128-SHA,AES128-SHA,AES256-SHA",
			wantErr:          true,
		},
		{
			name:             "3DES cipher suite is rejected with error",
			minVersionFlag:   "VersionTLS12",
			cipherSuitesFlag: "DES-CBC3-SHA",
			wantErr:          true,
		},
		{
			name:             "weak ciphers mixed with strong ciphers only accepts strong",
			minVersionFlag:   "VersionTLS12",
			cipherSuitesFlag: "ECDHE-RSA-AES128-SHA,ECDHE-RSA-AES128-GCM-SHA256,DES-CBC3-SHA,ECDHE-ECDSA-AES128-GCM-SHA256",
			wantMinVersion:   cryptotls.VersionTLS12,
			wantCipherSuites: []uint16{cryptotls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, cryptotls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := BuildTLSConfigFromFlags(tt.minVersionFlag, tt.cipherSuitesFlag)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if cfg.MinVersion != tt.wantMinVersion {
				t.Errorf("MinVersion = %d, want %d", cfg.MinVersion, tt.wantMinVersion)
			}

			if tt.wantCipherSuitesNil {
				if cfg.CipherSuites != nil {
					t.Errorf("CipherSuites = %v, want nil", cfg.CipherSuites)
				}
			} else {
				if !cipherSuitesEqual(cfg.CipherSuites, tt.wantCipherSuites) {
					t.Errorf("CipherSuites = %v, want %v", cfg.CipherSuites, tt.wantCipherSuites)
				}
			}

			// CurvePreferences should always be unset (for ML-KEM support)
			if cfg.CurvePreferences != nil {
				t.Errorf("CurvePreferences = %v, want nil (for ML-KEM support)", cfg.CurvePreferences)
			}
		})
	}
}

func cipherSuitesEqual(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTLSHandshakeWithFlags(t *testing.T) {
	// Test that TLS config built from flags actually works in a real handshake
	minVersionFlag := "VersionTLS12"
	cipherSuitesFlag := "ECDHE-RSA-AES128-GCM-SHA256"

	cert, certPEM, keyPEM := generateTestCert(t)

	serverCfg, err := BuildTLSConfigFromFlags(minVersionFlag, cipherSuitesFlag)
	if err != nil {
		t.Fatalf("Failed to build server TLS config: %v", err)
	}
	serverCfg.MaxVersion = cryptotls.VersionTLS12 // Force TLS 1.2 to test cipher suite negotiation
	serverCert, err := cryptotls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("Failed to create server cert: %v", err)
	}
	serverCfg.Certificates = []cryptotls.Certificate{serverCert}

	clientCfg, err := BuildTLSConfigFromFlags(minVersionFlag, cipherSuitesFlag)
	if err != nil {
		t.Fatalf("Failed to build client TLS config: %v", err)
	}
	clientCfg.MaxVersion = cryptotls.VersionTLS12 // Force TLS 1.2 to test cipher suite negotiation
	certPool := x509.NewCertPool()
	certPool.AddCert(cert)
	clientCfg.RootCAs = certPool
	clientCfg.ServerName = "localhost"

	// Test handshake
	listener, err := cryptotls.Listen("tcp", "127.0.0.1:0", serverCfg)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Errorf("Failed to close listener: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer func() {
			if err := conn.Close(); err != nil {
				t.Errorf("Failed to close server connection: %v", err)
			}
		}()

		// Perform TLS handshake on server side with context
		tlsConn, ok := conn.(*cryptotls.Conn)
		if !ok {
			serverDone <- nil
			return
		}
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			serverDone <- err
			return
		}
		serverDone <- nil
	}()

	dialer := &cryptotls.Dialer{Config: clientCfg}
	conn, err := dialer.DialContext(ctx, "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Client handshake failed: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("Failed to close client connection: %v", err)
		}
	}()

	// Verify client handshake completed
	tlsConn, ok := conn.(*cryptotls.Conn)
	if !ok {
		t.Fatal("Expected *tls.Conn from dialer")
	}
	state := tlsConn.ConnectionState()
	if !state.HandshakeComplete {
		t.Fatal("Client handshake not complete")
	}

	// Verify TLS 1.2 was negotiated
	if state.Version != cryptotls.VersionTLS12 {
		t.Errorf("Expected TLS 1.2, got version 0x%x", state.Version)
	}

	// Verify the expected cipher suite was negotiated
	if state.CipherSuite != cryptotls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 {
		t.Errorf("Expected cipher suite TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, got 0x%x", state.CipherSuite)
	}

	// Wait for server
	if err := <-serverDone; err != nil {
		t.Fatalf("Server handshake failed: %v", err)
	}
}

func TestTLSHandshakeWithTLS13(t *testing.T) {
	// Test TLS 1.3 negotiation to verify ML-KEM readiness (CurvePreferences left unset)
	minVersionFlag := "VersionTLS13"
	cipherSuitesFlag := "" // TLS 1.3 ignores cipher suites

	cert, certPEM, keyPEM := generateTestCert(t)

	serverCfg, err := BuildTLSConfigFromFlags(minVersionFlag, cipherSuitesFlag)
	if err != nil {
		t.Fatalf("Failed to build server TLS config: %v", err)
	}
	serverCert, err := cryptotls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("Failed to create server cert: %v", err)
	}
	serverCfg.Certificates = []cryptotls.Certificate{serverCert}

	clientCfg, err := BuildTLSConfigFromFlags(minVersionFlag, cipherSuitesFlag)
	if err != nil {
		t.Fatalf("Failed to build client TLS config: %v", err)
	}
	certPool := x509.NewCertPool()
	certPool.AddCert(cert)
	clientCfg.RootCAs = certPool
	clientCfg.ServerName = "localhost"

	// Test handshake
	listener, err := cryptotls.Listen("tcp", "127.0.0.1:0", serverCfg)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Errorf("Failed to close listener: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer func() {
			if err := conn.Close(); err != nil {
				t.Errorf("Failed to close server connection: %v", err)
			}
		}()

		tlsConn, ok := conn.(*cryptotls.Conn)
		if !ok {
			serverDone <- nil
			return
		}
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			serverDone <- err
			return
		}
		serverDone <- nil
	}()

	dialer := &cryptotls.Dialer{Config: clientCfg}
	conn, err := dialer.DialContext(ctx, "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Client handshake failed: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("Failed to close client connection: %v", err)
		}
	}()

	// Verify client handshake completed
	tlsConn, ok := conn.(*cryptotls.Conn)
	if !ok {
		t.Fatal("Expected *tls.Conn from dialer")
	}
	state := tlsConn.ConnectionState()
	if !state.HandshakeComplete {
		t.Fatal("Client handshake not complete")
	}

	// Verify TLS 1.3 was negotiated
	if state.Version != cryptotls.VersionTLS13 {
		t.Errorf("Expected TLS 1.3, got version 0x%x", state.Version)
	}

	// Verify CurvePreferences was left unset (ML-KEM readiness)
	if serverCfg.CurvePreferences != nil {
		t.Errorf("Server CurvePreferences should be nil for ML-KEM support, got %v", serverCfg.CurvePreferences)
	}
	if clientCfg.CurvePreferences != nil {
		t.Errorf("Client CurvePreferences should be nil for ML-KEM support, got %v", clientCfg.CurvePreferences)
	}

	// Wait for server
	if err := <-serverDone; err != nil {
		t.Fatalf("Server handshake failed: %v", err)
	}
}

func generateTestCert(t *testing.T) (*x509.Certificate, []byte, []byte) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	keyDER := x509.MarshalPKCS1PrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyDER,
	})

	return cert, certPEM, keyPEM
}
