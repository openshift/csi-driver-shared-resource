package metrics

import (
	"fmt"
	"io"
	"net/http"
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

func blockUntilServerStarted(port int) error {
	return wait.PollImmediate(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		if _, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port)); err != nil {
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

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
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

func testQueryCounterMetric(t *testing.T, testName string, port, amount int, query string) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
	if err != nil {
		t.Fatalf("error requesting metrics server: %v in test %q", err, testName)
	}
	metric := findMetricByCounter(resp.Body, query)
	if metric == nil {
		t.Fatalf("unable to locate metric %q in test %q", query, testName)
	}
	if metric.Counter.Value == nil {
		t.Fatalf("metric did not have value %q in test %q", query, testName)
	}
	if *metric.Counter.Value != float64(amount) {
		t.Fatalf("incorrect metric value %v for query %q in test %q", *metric.Counter.Value, query, testName)
	}
}

func findMetricByCounter(buf io.ReadCloser, name string) *io_prometheus_client.Metric {
	defer buf.Close()
	mf := io_prometheus_client.MetricFamily{}
	decoder := expfmt.NewDecoder(buf, "text/plain")
	for err := decoder.Decode(&mf); err == nil; err = decoder.Decode(&mf) {
		if *mf.Name == name {
			if len(mf.Metric) == 0 {
				return nil
			}
			return mf.Metric[0]
		}
	}
	return nil
}

type expectedMetricValue struct {
	total  int
	failed int
}

func TestMetricQueries(t *testing.T) {
	for _, test := range []struct {
		name     string
		expected expectedMetricValue
		mounts   map[bool]int
	}{
		{
			name: "One true, two false",
			expected: expectedMetricValue{
				total:  3,
				failed: 2,
			},
			mounts: map[bool]int{true: 1, false: 2},
		},
		{
			name: "Zero true, two false",
			expected: expectedMetricValue{
				total:  2,
				failed: 2,
			},
			mounts: map[bool]int{false: 2},
		},
		{
			name: "Three true, zero false",
			expected: expectedMetricValue{
				total:  3,
				failed: 0,
			},
			mounts: map[bool]int{true: 3},
		},
	} {
		prometheus.Unregister(mountCounter)
		prometheus.Unregister(mountFailedCounter)

		mountCounter = createMountCounter()
		mountFailedCounter = createMountFailedCounter()

		prometheus.MustRegister(mountCounter)
		prometheus.MustRegister(mountFailedCounter)

		for k, v := range test.mounts {
			for i := 0; i < v; i += 1 {
				IncMountCounter()
				if !k {
					IncMountFailedCounter()
				}
			}
		}

		port, ch := runMetricsServer(t)
		testQueryCounterMetric(t, test.name, port, test.expected.total, mountCountName)
		testQueryCounterMetric(t, test.name, port, test.expected.failed, mountFailedName)
		close(ch)
	}
}
