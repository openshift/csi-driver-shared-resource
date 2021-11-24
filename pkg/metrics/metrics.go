package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	separator = "_"

	sharesSubsystem = "openshift_csi_share"

	mount                 = "mount"
	mountCountName        = sharesSubsystem + separator + mount + separator + "requests_total"
	mountFailureCountName = sharesSubsystem + separator + mount + separator + "failures_total"

	MetricsPort = 6000
)

var (
	mountCounter, failedMountCounter = createMountCounters()
)

func createMountCounters() (prometheus.Counter, prometheus.Counter) {
	return prometheus.NewCounter(prometheus.CounterOpts{
			Name: mountCountName,
			Help: "Counts share volume mount attempts.",
		}),
		prometheus.NewCounter(prometheus.CounterOpts{
			Name: mountFailureCountName,
			Help: "Counts failed share volume mount attempts.",
		})
}

func init() {
	prometheus.MustRegister(mountCounter)
	prometheus.MustRegister(failedMountCounter)
}

func IncMountCounters(succeeded bool) {
	if !succeeded {
		failedMountCounter.Inc()
	}
	mountCounter.Inc()
}
