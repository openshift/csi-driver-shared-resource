package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/klog/v2"
)

var (
	// these files are mounted from the openshift secret
	// shared-resource-csi-driver-node-metrics-serving-cert
	// by the csi-driver-shared-resource-operator
	tlsCRT = "/etc/secrets/tls.crt"
	tlsKey = "/etc/secrets/tls.key"
)

// BuildServer creates the http.Server struct
func BuildServer(port int) (*http.Server, error) {
	if port <= 0 {
		klog.Error("invalid port for metric server")
		return nil, errors.New("invalid port for metrics server")
	}

	bindAddr := fmt.Sprintf(":%d", port)
	router := http.NewServeMux()
	router.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{
		Addr:    bindAddr,
		Handler: router,
	}

	return srv, nil
}

// StopServer stops the metrics server
func StopServer(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		klog.Warningf("Problem shutting down HTTP server: %v", err)
	}
}

// RunServer starts the metrics server.
func RunServer(srv *http.Server, stopCh <-chan struct{}) {
	go func() {
		err := srv.ListenAndServeTLS(tlsCRT, tlsKey)
		if err != nil && err != http.ErrServerClosed {
			klog.Errorf("error starting metrics server: %v", err)
		}
	}()
	<-stopCh
	if err := srv.Close(); err != nil {
		klog.Errorf("error closing metrics server: %v", err)
	}
}
