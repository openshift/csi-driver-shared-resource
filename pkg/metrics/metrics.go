package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	separator = "_"

	sharesSubsystem = "openshift_csi_share"

	mount    = "mount"
	requests = "requests"
	failed   = "failures"

	mountCountName  = sharesSubsystem + separator + mount + separator + requests + separator + "total"
	mountFailedName = sharesSubsystem + separator + mount + separator + failed + separator + "total"

	MetricsPort = 6000
)

var (
	mountCounter       = createMountCounter()
	mountFailedCounter = createMountFailedCounter()
)

func createMountCounter() prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Name: mountCountName,
		Help: "Counts all attempts for csi driver volume mounts.",
	})
}

func createMountFailedCounter() prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Name: mountFailedName,
		Help: "Counts failed mount attempts for csi driver volume mounts.",
	})
}

func init() {
	prometheus.MustRegister(mountCounter)
	prometheus.MustRegister(mountFailedCounter)
}

func IncMountCounter() {
	mountCounter.Inc()
}

func IncMountFailedCounter() {
	mountFailedCounter.Inc()
}
