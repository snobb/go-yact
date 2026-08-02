// Package client provides client.
package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

const (
	DefaultKeepAliveInterval = 20 * time.Second
	DefaultKeepAliveTimeout  = 5 * time.Second
)

func makeServerConn(cfg *Config) (*grpc.ClientConn, func(), error) {
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
			Time:    DefaultKeepAliveInterval,
			Timeout: DefaultKeepAliveTimeout,
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
	caCert, err := os.ReadFile(cfg.CAPath)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	serverCert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
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
