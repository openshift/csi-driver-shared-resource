package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	separator = "_"

	sharesSubsystem = "openshift_csi_share"

	mount          = "mount"
	mountCountName = sharesSubsystem + separator + mount + separator + "total"

	MetricsPort = 6000
)

var (
	mountCounter = createMountCounter()
)

func createMountCounter() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: mountCountName,
		Help: "Counts share volume mount attempts by success. " +
			"'succeeded' label will hold 'true' in case of succeeded mount, and 'false' otherwise.",
	}, []string{"succeeded"})
}

func init() {
	prometheus.MustRegister(mountCounter)
}

func IncMountCounter(succeeded bool) {
	mountCounter.With(prometheus.Labels{"succeeded": strconv.FormatBool(succeeded)}).Inc()
}
