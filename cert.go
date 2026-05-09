package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

const (
	certPath = ".ssl/cert.pem"
	keyPath  = ".ssl/key.pem"
)

// GenerateSelfSignedCert creates a new RSA-2048 self-signed certificate valid
// for the given hostnames and IP addresses, writing cert.pem and key.pem to
// the .ssl/ directory. Pure Go — no openssl dependency.
func GenerateSelfSignedCert(hostnames []string, ips []net.IP) error {
	if err := os.MkdirAll(".ssl", 0700); err != nil {
		return fmt.Errorf("creating .ssl dir: %w", err)
	}

	// Generate private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generating RSA key: %w", err)
	}

	// Serial number
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generating serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "gemmalink",
			Organization: []string{"GemmaLink"},
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              hostnames,
		IPAddresses:           ips,
	}

	// Self-sign
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("creating certificate: %w", err)
	}

	// Write cert.pem
	cf, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("writing cert.pem: %w", err)
	}
	defer cf.Close()
	if err := pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return err
	}

	// Write key.pem
	kf, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("writing key.pem: %w", err)
	}
	defer kf.Close()
	return pem.Encode(kf, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
}

// DefaultCertSANs returns the standard set of hostnames and IPs for the cert.
func DefaultCertSANs(cfg *Config) ([]string, []net.IP) {
	hostnames := []string{"localhost"}
	ips := []net.IP{net.ParseIP("127.0.0.1")}

	if cfg.Hostname != "" && cfg.Hostname != "localhost" {
		hostnames = append(hostnames, cfg.Hostname)
		if ip := net.ParseIP(cfg.Hostname); ip != nil {
			ips = append(ips, ip)
		}
	}

	// Add LAN IPs
	if lanIPs := getLANIPs(); len(lanIPs) > 0 {
		ips = append(ips, lanIPs...)
	}

	return hostnames, ips
}
