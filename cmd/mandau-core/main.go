package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	certPath := flag.String("cert", "certs/core.crt", "Certificate path")
	keyPath := flag.String("key", "certs/core.key", "Key path")
	caPath := flag.String("ca", "certs/ca.crt", "CA certificate path")
	listenAddr := flag.String("listen", ":8443", "Listen address (e.g., :8443)")
	flag.Parse()

	fmt.Printf("Starting Mandau Core on %s...\n", *listenAddr)

	// Load CA certificate
	caCert, err := ioutil.ReadFile(*caPath)
	if err != nil {
		log.Fatalf("failed to read CA cert: %v", err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		log.Fatalf("failed to parse CA cert")
	}

	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		log.Fatalf("failed to load server cert: %v", err)
	}

	// Create TLS credentials
	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	})

	s := grpc.NewServer(grpc.Creds(creds))
	// TODO: Register services

	lis, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	fmt.Printf("Mandau Core listening on %s with TLS\n", *listenAddr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
