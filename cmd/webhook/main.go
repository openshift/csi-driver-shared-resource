package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
)

const (
	WebhookName string = "sharedresourcecsidriver-csidriver"
)

var (
	listenAddress = flag.String("listen", "0.0.0.0", "listen address")
	listenPort    = flag.String("port", "5000", "port to listen on")
)

func index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>The CSI Shared Resource webhook is under development<h1>")
}
func main() {
	flag.Parse()
	http.HandleFunc("/"+WebhookName, index)
	server := &http.Server{
		Addr: net.JoinHostPort(*listenAddress, *listenPort),
	}
	server.ListenAndServe()
}
