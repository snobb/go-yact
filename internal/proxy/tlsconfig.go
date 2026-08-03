package proxy

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

// InitCredentials creates TLS credentials.
func InitCredentials(caPath, certPath, keyPath string) (credentials.TransportCredentials, error) {
	caCert, err := os.ReadFile(caPath) // nolint:gosec
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	serverCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	tc := NewTLSCredentials(serverCert, caCertPool)

	return tc, nil
}

// NewTLSCredentials creates TLS credentials.
func NewTLSCredentials(cert tls.Certificate, caPool *x509.CertPool) credentials.TransportCredentials {
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13,
	}

	return credentials.NewTLS(tlsConfig)
}

// NewGRPCServer creates a new gRPC server.
func NewGRPCServer(
	tc credentials.TransportCredentials,
	keepAliveInterval, keepAliveTimeout time.Duration,
) *grpc.Server {
	return grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    keepAliveInterval,
			Timeout: keepAliveTimeout,
		}),
		grpc.Creds(tc),
	)
}
