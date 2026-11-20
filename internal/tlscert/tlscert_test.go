package tlscert

import (
	"crypto/x509"
	"net"
	"testing"
	"time"
)

func TestSelfSignedCertificate(t *testing.T) {
	cert, err := SelfSignedCertificate()
	if err != nil {
		t.Fatalf("SelfSignedCertificate() error = %v", err)
	}

	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}

	if now := time.Now(); parsed.NotBefore.After(now) || parsed.NotAfter.Before(now) {
		t.Errorf("certificate validity = [%v, %v], want to cover now", parsed.NotBefore, parsed.NotAfter)
	}

	if got := parsed.DNSNames; len(got) != 1 || got[0] != "localhost" {
		t.Errorf("DNSNames = %v, want [localhost]", got)
	}

	if !containsIP(parsed.IPAddresses, net.ParseIP("127.0.0.1")) {
		t.Errorf("IPAddresses = %v, want to contain 127.0.0.1", parsed.IPAddresses)
	}

	if err := parsed.CheckSignatureFrom(parsed); err != nil {
		t.Errorf("CheckSignatureFrom(self) error = %v, want self-signed", err)
	}
}

func TestSelfSignedCertificateUniqueKeys(t *testing.T) {
	first, err := SelfSignedCertificate()
	if err != nil {
		t.Fatalf("SelfSignedCertificate() error = %v", err)
	}
	second, err := SelfSignedCertificate()
	if err != nil {
		t.Fatalf("SelfSignedCertificate() error = %v", err)
	}

	firstParsed, err := x509.ParseCertificate(first.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}
	secondParsed, err := x509.ParseCertificate(second.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}

	if firstParsed.SerialNumber.Cmp(secondParsed.SerialNumber) == 0 {
		t.Error("serial numbers are equal, want unique")
	}
}

func containsIP(ips []net.IP, target net.IP) bool {
	for _, ip := range ips {
		if ip.Equal(target) {
			return true
		}
	}
	return false
}
