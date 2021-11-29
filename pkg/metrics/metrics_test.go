package metrics

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type fakeResponseWriter struct {
	bytes.Buffer
	statusCode int
	header     http.Header
}

func (f *fakeResponseWriter) Header() http.Header {
	return f.header
}

func (f *fakeResponseWriter) WriteHeader(statusCode int) {
	f.statusCode = statusCode
}

func TestMetrics(t *testing.T) {
	for _, test := range []struct {
		name        string
		expected    []string
		notExpected []string
		mounts      map[bool]int
	}{
		{
			name: "One true, two false",
			expected: []string{
				`# TYPE openshift_csi_share_mount_requests_total counter`,
				`# TYPE openshift_csi_share_mount_failures_total counter`,
				`openshift_csi_share_mount_requests_total 3`,
				`openshift_csi_share_mount_failures_total 2`,
			},
			mounts:      map[bool]int{true: 1, false: 2},
			notExpected: []string{},
		},
		{
			name: "Two true, no false",
			expected: []string{
				`# TYPE openshift_csi_share_mount_requests_total counter`,
				`# TYPE openshift_csi_share_mount_failures_total counter`,
				`openshift_csi_share_mount_requests_total 2`,
				`openshift_csi_share_mount_failures_total 0`,
			},
			notExpected: []string{
				`openshift_csi_share_mount_total{succeeded="false"}`,
			},
			mounts: map[bool]int{true: 2},
		},
		{
			name: "No true, three false",
			expected: []string{
				`# TYPE openshift_csi_share_mount_requests_total counter`,
				`# TYPE openshift_csi_share_mount_failures_total counter`,
				`openshift_csi_share_mount_requests_total 3`,
				`openshift_csi_share_mount_failures_total 3`,
			},
			notExpected: []string{
				`openshift_csi_share_mount_total{succeeded="true"}`,
			},
			mounts: map[bool]int{false: 3},
		},
	} {
		registry := prometheus.NewRegistry()
		mountCounter = createMountCounter()
		mountFailedCounter = createMountFailedCounter()
		registry.MustRegister(mountCounter)
		registry.MustRegister(mountFailedCounter)

		for k, v := range test.mounts {
			for i := 0; i < v; i += 1 {
				IncMountCounter()
				if !k {
					IncMountFailedCounter()
				}
			}
		}

		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorHandling: promhttp.PanicOnError})
		rw := &fakeResponseWriter{header: http.Header{}}
		h.ServeHTTP(rw, &http.Request{})

		respStr := rw.String()

		for _, s := range test.expected {
			if !strings.Contains(respStr, s) {
				t.Errorf("testcase %s: expected string %s did not appear in %s", test.name, s, respStr)
			}
		}

		for _, s := range test.notExpected {
			if strings.Contains(respStr, s) {
				t.Errorf("testcase %s: expected to not find string %s in %s", test.name, s, respStr)
			}
		}
	}
}
