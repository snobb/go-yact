// Package client provides client.
package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

func makeControlConn(cfg *Config) (*grpc.ClientConn, func(), error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}

	tc, err := initCredentials(cfg)
	if err != nil {
		return nil, nil, err
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tc),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    cfg.KeepAliveInterval,
			Timeout: cfg.KeepAliveTimeout,
		}),
	}

	conn, err := grpc.NewClient(cfg.ProxyAddr, opts...)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		_ = conn.Close()
	}

	return conn, cleanup, nil
}

func initCredentials(cfg *Config) (credentials.TransportCredentials, error) {
	ca, cert, key := cfg.TLS.ClientPaths()
	caCert, err := os.ReadFile(ca)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	serverCert, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	tc := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverCert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS13,
	})

	return tc, nil
}
