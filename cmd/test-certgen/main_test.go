package main

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateCreatesMutuallyVerifiableCertificates(t *testing.T) {
	output := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	if err := generate(output, now); err != nil {
		t.Fatalf("generate certificates: %v", err)
	}

	ca := readCertificate(t, filepath.Join(output, "ca.crt"))
	server := readCertificate(t, filepath.Join(output, "server.crt"))
	client := readCertificate(t, filepath.Join(output, "client.crt"))
	roots := x509.NewCertPool()
	roots.AddCert(ca)

	if _, err := server.Verify(x509.VerifyOptions{
		Roots:       roots,
		DNSName:     "testbed-pdp-mtls",
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		CurrentTime: now,
	}); err != nil {
		t.Fatalf("verify server certificate: %v", err)
	}
	if _, err := client.Verify(x509.VerifyOptions{
		Roots:       roots,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		CurrentTime: now,
	}); err != nil {
		t.Fatalf("verify client certificate: %v", err)
	}
	if _, err := server.Verify(x509.VerifyOptions{
		Roots:       roots,
		DNSName:     "wrong.testbed.invalid",
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		CurrentTime: now,
	}); err == nil {
		t.Fatal("server certificate unexpectedly verified for the wrong DNS name")
	}
}

func readCertificate(t *testing.T, path string) *x509.Certificate {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("%s does not contain a PEM certificate", path)
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return certificate
}
