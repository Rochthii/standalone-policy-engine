// Command test-certgen creates short-lived certificates for the isolated Docker testbed.
// It must never be used to provision production credentials.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

const fileMode = 0o444

func main() {
	output := flag.String("output", "/certs", "directory for generated test certificates")
	flag.Parse()
	if err := generate(*output, time.Now().UTC()); err != nil {
		fmt.Fprintf(os.Stderr, "test certificate generation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("MTLS-CERTGEN PASS: short-lived CA, server and Odoo client certificates written to %s\n", *output)
}

func generate(output string, now time.Time) error {
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Standalone PDP Testbed CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	if err := writePEM(filepath.Join(output, "ca.crt"), "CERTIFICATE", caDER); err != nil {
		return err
	}
	if err := issueLeaf(output, "server", ca, caKey, now, true); err != nil {
		return err
	}
	return issueLeaf(output, "client", ca, caKey, now, false)
}

func issueLeaf(output, name string, ca *x509.Certificate, caKey *ecdsa.PrivateKey, now time.Time, server bool) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return err
	}
	commonName := "odoo-testbed-client"
	usage := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  usage,
	}
	if server {
		template.Subject.CommonName = "testbed-pdp-mtls"
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		template.DNSNames = []string{"testbed-pdp-mtls", "localhost"}
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		return err
	}
	if err := writePEM(filepath.Join(output, name+".crt"), "CERTIFICATE", certificateDER); err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	return writePEM(filepath.Join(output, name+".key"), "EC PRIVATE KEY", keyDER)
}

func writePEM(path, blockType string, data []byte) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: data}), fileMode)
}
