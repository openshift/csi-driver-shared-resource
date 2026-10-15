package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/klog/v2"

	"github.com/openshift/csi-driver-shared-resource/pkg/config"
	tlsprofile "github.com/openshift/csi-driver-shared-resource/pkg/tls"
	"github.com/openshift/csi-driver-shared-resource/pkg/webhook/csidriver"
	"github.com/openshift/csi-driver-shared-resource/pkg/webhook/dispatcher"
)

const (
	WebhookName string = "sharedresourcecsidriver-csidriver"
)

var (
	useTLS          bool
	tlsCert         string
	tlsKey          string
	caCert          string
	listenAddress   string
	listenPort      int
	testHooks       bool
	tlsMinVersion   string
	tlsCipherSuites string
)

var (
	CmdWebhook = &cobra.Command{
		Use:     "csi-driver-shared-resource-webhook",
		Version: "0.0.1",
		Short:   "",
		Long:    ``,
		Run: func(cmd *cobra.Command, args []string) {
			startServer()
		},
	}
)

func init() {
	klog.InitFlags(nil)
	CmdWebhook.Flags().BoolVar(&useTLS, "tls", false, "Use TLS? Must specify -tlskey, -tlscert, -cacert")
	CmdWebhook.Flags().StringVar(&tlsCert, "tlscert", "", "File containing the x509 Certificate for HTTPS")
	CmdWebhook.Flags().StringVar(&tlsKey, "tlskey", "", "File containing the x509 private key")
	CmdWebhook.Flags().StringVar(&caCert, "cacert", "", "File containing the x509 CA cert for HTTPS")
	CmdWebhook.Flags().StringVar(&listenAddress, "listen", "0.0.0.0", "Listen address")
	CmdWebhook.Flags().IntVar(&listenPort, "port", 5000, "Secure port that the webhook listens on")
	CmdWebhook.Flags().BoolVar(&testHooks, "testHooks", false, "Test webhook URI uniqueness and quit")
	CmdWebhook.Flags().StringVar(&tlsMinVersion, "tls-min-version", "", "Minimum TLS version (e.g., VersionTLS12)")
	CmdWebhook.Flags().StringVar(&tlsCipherSuites, "tls-cipher-suites", "", "Comma-separated list of cipher suites in IANA format (e.g., TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256)")
}

func startServer() {
	webhook := csidriver.NewWebhook(config.SetupNameReservation())
	dispatcher := dispatcher.NewDispatcher(webhook)
	http.HandleFunc(webhook.GetURI(), dispatcher.HandleRequest)

	if testHooks {
		os.Exit(0)
	} else {
		fmt.Printf("HTTP server running at: %s", net.JoinHostPort(listenAddress, strconv.Itoa(listenPort)))
	}

	server := &http.Server{
		Addr:         net.JoinHostPort(listenAddress, strconv.Itoa(listenPort)),
		ReadTimeout:  30 * time.Second, // Maximum webhook timeout in Kubernetes (default is 10s)
		WriteTimeout: 30 * time.Second, // Allow time for admission response
		IdleTimeout:  120 * time.Second,
	}
	//TODO do we want to explore signal handling / graceful shutdown for the webhook, with some wrapper around the http server
	var err error
	if useTLS {
		var cafile []byte
		cafile, err = os.ReadFile(caCert)
		if err != nil {
			fmt.Printf("Couldn't read CA cert file: %s", err.Error())
			os.Exit(1)
		}
		certpool := x509.NewCertPool()
		if !certpool.AppendCertsFromPEM(cafile) {
			fmt.Printf("Failed to parse CA certificate from %s\n", caCert)
			os.Exit(1)
		}

		// Build TLS config from flags
		var tlsCfg *tls.Config
		tlsCfg, err = tlsprofile.BuildTLSConfigFromFlags(tlsMinVersion, tlsCipherSuites)
		if err != nil {
			fmt.Printf("Invalid TLS configuration: %s\n", err.Error())
			os.Exit(1)
		}
		tlsCfg.ClientCAs = certpool
		server.TLSConfig = tlsCfg

		err = server.ListenAndServeTLS(tlsCert, tlsKey)
	} else {
		err = server.ListenAndServe()
	}
	if err != nil {
		fmt.Printf("Error serving connection: %s", err.Error())
		os.Exit(1)
	}
}

func main() {
	if err := CmdWebhook.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
