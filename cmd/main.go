package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/klog/v2"

	v1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/csi-driver-shared-resource/pkg/controller"
	"github.com/openshift/csi-driver-shared-resource/pkg/hostpath"
)

var (
	cfgFile             string
	endPoint            string
	driverName          string
	nodeID              string
	maxVolumesPerNode   int64
	version             string
	shareRelistInterval string

	shutdownSignals      = []os.Signal{os.Interrupt, syscall.SIGTERM}
	onlyOneSignalHandler = make(chan struct{})
)

var rootCmd = &cobra.Command{
	Use:     "driver-projected-resource",
	Version: "0.0.1",
	Short:   "",
	Long:    ``,
	Run: func(cmd *cobra.Command, args []string) {
		driver, err := hostpath.NewHostPathDriver(hostpath.DataRoot, hostpath.VolumeMapRoot, driverName, nodeID, endPoint, maxVolumesPerNode, version)
		if err != nil {
			fmt.Printf("Failed to initialize driver: %s", err.Error())
			os.Exit(1)
		}
		go runOperator()
		driver.Run()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	klog.InitFlags(nil)

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	cobra.OnInitialize()

	rootCmd.Flags().AddGoFlagSet(flag.CommandLine)
	rootCmd.Flags().StringVar(&endPoint, "endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	rootCmd.Flags().StringVar(&driverName, "drivername", string(v1.SharedResourcesCSIDriver), "name of the driver")
	rootCmd.Flags().StringVar(&nodeID, "nodeid", "", "node id")
	rootCmd.Flags().Int64Var(&maxVolumesPerNode, "maxvolumespernode", 0, "limit of volumes per node")
	rootCmd.Flags().StringVar(&shareRelistInterval, "share-relist-interval", "",
		"the time between controller relist on the share resource expressed with golang time.Duration syntax(default=10m")
}

func runOperator() {
	shareRelist := controller.DefaultResyncDuration
	var err error
	// flag defaulting above did not work well with time.Duration
	if len(shareRelistInterval) > 0 {
		shareRelist, err = time.ParseDuration(shareRelistInterval)
		if err != nil {
			fmt.Printf("Error parsing share-relist-in-min flag, using default")
			shareRelist = controller.DefaultResyncDuration
		}
	}
	c, err := controller.NewController(shareRelist)
	if err != nil {
		fmt.Printf("Failed to set up controller: %s", err.Error())
		os.Exit(1)
	}
	stopCh := setupSignalHandler()
	err = c.Run(stopCh)
	if err != nil {
		fmt.Printf("Controller exited: %s", err.Error())
		os.Exit(1)
	}
}

// setupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func setupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}
