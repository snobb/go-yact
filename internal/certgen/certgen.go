// Package certgen generates mTLS certificates using Ed25519.
package certgen

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	CACertFile     = "ca.crt"
	ServerCertFile = "server.crt"
	ServerKeyFile  = "server.key"
	ClientCertFile = "client.crt"
	ClientKeyFile  = "client.key"

	caLifetime   = 10 * 365 * 24 * time.Hour
	certLifetime = 365 * 24 * time.Hour
)

// Generate creates a CA, server, and client certificate in certDir.
// The CA private key is never written to disk.
func Generate(certDir string, domains []string, ipAddrs []net.IP, nClients int) error {
	if err := os.MkdirAll(certDir, 0o755); err != nil {
		return fmt.Errorf("creating cert directory: %w", err)
	}

	caKey, caCert, caCertDER, err := generateCA()
	if err != nil {
		return fmt.Errorf("generating CA: %w", err)
	}

	if err := writePEM(filepath.Join(certDir, CACertFile), "CERTIFICATE", caCertDER, 0o644); err != nil {
		return fmt.Errorf("writing CA cert: %w", err)
	}

	if err := generateCert(certDir, caKey, caCert, "server",
		x509.ExtKeyUsageServerAuth,
		"server",
		domains,
		ipAddrs,
	); err != nil {
		return fmt.Errorf("generating server cert: %w", err)
	}

	for i := range nClients {
		if err := generateCert(certDir, caKey, caCert,
			fmt.Sprintf("client%d", i+1),
			x509.ExtKeyUsageClientAuth,
			"client",
			nil, // explicitly empty for client cert DNS names.
			nil, // explicitly empty for client cert IP addresses.
		); err != nil {
			return fmt.Errorf("generating client cert: %w", err)
		}
	}

	return nil
}

func generateCA() (ed25519.PrivateKey, *x509.Certificate, []byte, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"Yact"},
			CommonName:   "Yact CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(caLifetime),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pub, priv)
	if err != nil {
		return nil, nil, nil, err
	}

	return priv, template, certDER, nil
}

func generateCert(
	certDir string,
	caKey ed25519.PrivateKey,
	caCert *x509.Certificate,
	name string,
	extKeyUsage x509.ExtKeyUsage,
	role string,
	dnsNames []string,
	ipAddrs []net.IP,
) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	serial, err := randomSerial()
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"Yact"},
			CommonName:   "yact-" + name,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(certLifetime),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{extKeyUsage},
		DNSNames:    dnsNames,
		IPAddresses: ipAddrs,
		URIs:        []*url.URL{{Scheme: "role", Opaque: role}},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, pub, caKey)
	if err != nil {
		return err
	}

	certFile := filepath.Join(certDir, name+".crt")
	if err := writePEM(certFile, "CERTIFICATE", certDER, 0o644); err != nil {
		return err
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}

	keyFile := filepath.Join(certDir, name+".key")
	return writePEM(keyFile, "PRIVATE KEY", privDER, 0o600)
}

func writePEM(path, blockType string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Close()

	return pem.Encode(f, &pem.Block{Type: blockType, Bytes: data})
}

func randomSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}
