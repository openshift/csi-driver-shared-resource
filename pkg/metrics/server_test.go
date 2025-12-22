package metrics

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	mr "math/rand"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	portOffset uint32 = 0
)

func TestMain(m *testing.M) {
	var err error

	mr.Seed(time.Now().UnixNano())

	tlsKey, tlsCRT, err = generateTempCertificates()
	if err != nil {
		panic(err)
	}

	// sets the default http client to skip certificate check.
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	code := m.Run()
	os.Remove(tlsKey)
	os.Remove(tlsCRT)
	os.Exit(code)
}

func generateTempCertificates() (string, string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", "", err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, key.Public(), key)
	if err != nil {
		return "", "", err
	}

	cert, err := os.CreateTemp("", "testcert-")
	if err != nil {
		return "", "", err
	}
	defer cert.Close()
	pem.Encode(cert, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	keyPath, err := os.CreateTemp("", "testkey-")
	if err != nil {
		return "", "", err
	}
	defer keyPath.Close()
	pem.Encode(keyPath, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return keyPath.Name(), cert.Name(), nil
}

func blockUntilServerStarted(port int) error {
	return wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		if _, err := http.Get(fmt.Sprintf("https://localhost:%d/metrics", port)); err != nil {
			// in case error is "connection refused", server is not up (yet)
			// it is possible that it is still being started
			// in that case we need to try more
			if utilnet.IsConnectionRefused(err) {
				return false, nil
			}

			// in case of a different error, return immediately
			return true, err
		}

		// no error, stop polling the server, continue with the test logic
		return true, nil
	})
}

func runMetricsServer(t *testing.T) (int, chan<- struct{}) {
	var port int = MetricsPort + int(atomic.AddUint32(&portOffset, 1))

	ch := make(chan struct{})
	server, err := BuildServer(port)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	go RunServer(server, ch)

	if err := blockUntilServerStarted(port); err != nil {
		t.Fatalf("error while waiting for metrics server: %v", err)
	}

	return port, ch
}

func TestRunServer(t *testing.T) {
	port, ch := runMetricsServer(t)
	defer close(ch)

	resp, err := http.Get(fmt.Sprintf("https://localhost:%d/metrics", port))
	if err != nil {
		t.Fatalf("error while querying metrics server: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("Server response status is %q instead of 200", resp.Status)
	}
}

func findMetricByLabel(metrics []*io_prometheus_client.Metric, label, value string) *io_prometheus_client.Metric {
	for _, m := range metrics {
		for _, l := range m.Label {
			if l != nil && *l.Name == label && *l.Value == value {
				return m
			}
		}
	}

	return nil
}

func testServerForExpected(t *testing.T, testName string, port int, expected []metric) {
	resp, err := http.Get(fmt.Sprintf("https://localhost:%d/metrics", port))
	if err != nil {
		t.Fatalf("error requesting metrics server: %v in test %q", err, testName)
	}
	var p expfmt.TextParser
	mf, err := p.TextToMetricFamilies(resp.Body)
	if err != nil {
		t.Fatalf("error parsing server response: %v in test %q", err, testName)
	}

	for _, e := range expected {
		if mf[e.name] == nil {
			t.Fatalf("expected metric %v not found in server response: in test %q", e.name, testName)
		}
		v := *(mf[e.name].GetMetric()[0].GetCounter().Value)
		if v != e.value {
			t.Fatalf("metric value %v differs from expected %v: in test %q", v, e.value, testName)
		}
	}
}

type metric struct {
	name  string
	value float64
}

func TestMetricQueries(t *testing.T) {
	for _, test := range []struct {
		name     string
		expected []metric
		mounts   map[bool]int
	}{
		{
			name: "One true, two false",
			expected: []metric{
				{
					name:  mountCountName,
					value: 3,
				},
				{
					name:  mountFailureCountName,
					value: 2,
				},
			},
			mounts: map[bool]int{true: 1, false: 2},
		},
		{
			name: "Zero true, two false",
			expected: []metric{
				{
					name:  mountCountName,
					value: 2,
				},
				{
					name:  mountFailureCountName,
					value: 2,
				},
			},
			mounts: map[bool]int{false: 2},
		},
		{
			name: "Three true, zero false",
			expected: []metric{
				{
					name:  mountCountName,
					value: 3,
				},
				{
					name:  mountFailureCountName,
					value: 0,
				},
			},
			mounts: map[bool]int{true: 3},
		},
	} {
		prometheus.Unregister(mountCounter)
		prometheus.Unregister(failedMountCounter)
		mountCounter, failedMountCounter = createMountCounters()
		prometheus.MustRegister(mountCounter)
		prometheus.MustRegister(failedMountCounter)

		for k, v := range test.mounts {
			for i := 0; i < v; i += 1 {
				IncMountCounters(k)
			}
		}

		port, ch := runMetricsServer(t)
		testServerForExpected(t, test.name, port, test.expected)
		close(ch)
	}
}
