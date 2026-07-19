package daemon

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// makeTestCert returns a self-signed PEM keypair for domain expiring at
// notAfter.
func makeTestCert(t *testing.T, domain string, notAfter time.Time) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		DNSNames:     []string{domain},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

func writeTestCert(t *testing.T, dir, domain string, notAfter time.Time) (certFile, keyFile string) {
	t.Helper()
	certPEM, keyPEM := makeTestCert(t, domain, notAfter)
	certFile = filepath.Join(dir, domain+".crt")
	keyFile = filepath.Join(dir, domain+".key")
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}

func TestCertManagerEnsure(t *testing.T) {
	const domain = "host.ts.net"
	ctx := context.Background()
	now := time.Now()
	notAfter := now.Add(90 * 24 * time.Hour)
	mints := 0
	m := newCertManager(filepath.Join(t.TempDir(), "tls"))
	m.now = func() time.Time { return now }
	m.mint = func(_ context.Context, dnsName, dir string) (string, string, error) {
		mints++
		certFile, keyFile := writeTestCert(t, dir, dnsName, notAfter)
		return certFile, keyFile, nil
	}

	m.ensure(ctx, "")
	if mints != 0 || m.get() != nil {
		t.Fatalf("empty domain: mints = %d, cert = %v, want no mint and nil", mints, m.get())
	}

	m.ensure(ctx, domain)
	if mints != 1 {
		t.Fatalf("first ensure: mints = %d, want 1", mints)
	}
	cert := m.get()
	if cert == nil {
		t.Fatal("first ensure left no cert")
	}
	if got := cert.Leaf.NotAfter.Unix(); got != notAfter.Unix() {
		t.Fatalf("leaf NotAfter = %d, want %d", got, notAfter.Unix())
	}

	m.ensure(ctx, domain)
	if mints != 1 {
		t.Fatalf("fresh cert re-minted: mints = %d, want 1", mints)
	}

	now = notAfter.Add(-15 * 24 * time.Hour)
	renewed := notAfter.Add(90 * 24 * time.Hour)
	notAfter = renewed
	m.ensure(ctx, domain)
	if mints != 2 {
		t.Fatalf("within refresh window: mints = %d, want 2", mints)
	}
	if got := m.get().Leaf.NotAfter.Unix(); got != renewed.Unix() {
		t.Fatalf("renewed leaf NotAfter = %d, want %d", got, renewed.Unix())
	}

	now = renewed.Add(-24 * time.Hour)
	m.mint = func(context.Context, string, string) (string, string, error) {
		mints++
		return "", "", errors.New("boom")
	}
	m.ensure(ctx, domain)
	if mints != 3 {
		t.Fatalf("failing mint not invoked: mints = %d, want 3", mints)
	}
	if got := m.get(); got == nil || got.Leaf.NotAfter.Unix() != renewed.Unix() {
		t.Fatalf("mint failure evicted the prior cert: %v", got)
	}
}

func TestCertManagerGetCertificate(t *testing.T) {
	m := newCertManager(t.TempDir())
	if _, err := m.GetCertificate(nil); !errors.Is(err, errNoCert) {
		t.Fatalf("empty manager GetCertificate err = %v, want errNoCert", err)
	}
}

func TestCertManagerDomainChange(t *testing.T) {
	ctx := context.Background()
	notAfter := time.Now().Add(90 * 24 * time.Hour)
	mints := 0
	m := newCertManager(filepath.Join(t.TempDir(), "tls"))
	m.mint = func(_ context.Context, dnsName, dir string) (string, string, error) {
		mints++
		certFile, keyFile := writeTestCert(t, dir, dnsName, notAfter)
		return certFile, keyFile, nil
	}

	m.ensure(ctx, "old.ts.net")
	if mints != 1 || m.get() == nil {
		t.Fatalf("first ensure: mints = %d, cert = %v, want 1 and non-nil", mints, m.get())
	}

	m.ensure(ctx, "new.ts.net")
	if mints != 2 {
		t.Fatalf("domain change did not re-mint: mints = %d, want 2", mints)
	}
	cert := m.get()
	if cert == nil || !slices.Contains(cert.Leaf.DNSNames, "new.ts.net") {
		t.Fatalf("served cert not re-minted for new domain: %v", cert)
	}
}

func TestCertManagerExpiredCert(t *testing.T) {
	const domain = "host.ts.net"
	ctx := context.Background()
	now := time.Now()
	notAfter := now.Add(90 * 24 * time.Hour)
	m := newCertManager(filepath.Join(t.TempDir(), "tls"))
	m.now = func() time.Time { return now }
	m.mint = func(_ context.Context, dnsName, dir string) (string, string, error) {
		certFile, keyFile := writeTestCert(t, dir, dnsName, notAfter)
		return certFile, keyFile, nil
	}

	m.ensure(ctx, domain)
	if m.get() == nil {
		t.Fatal("first ensure left no cert")
	}

	m.mint = func(context.Context, string, string) (string, string, error) {
		return "", "", errors.New("tailscale down")
	}
	now = notAfter.Add(time.Minute)
	m.ensure(ctx, domain)
	if got := m.get(); got != nil {
		t.Fatalf("expired cert still served: %v", got)
	}
	if _, err := m.GetCertificate(nil); !errors.Is(err, errNoCert) {
		t.Fatalf("expired cert GetCertificate err = %v, want errNoCert", err)
	}
}

func TestCertManagerSingleFlight(t *testing.T) {
	const domain = "host.ts.net"
	certFile, keyFile := writeTestCert(t, t.TempDir(), domain, time.Now().Add(90*24*time.Hour))
	m := newCertManager(filepath.Join(t.TempDir(), "tls"))
	block := make(chan struct{})
	started := make(chan struct{})
	var mints atomic.Int32
	m.mint = func(context.Context, string, string) (string, string, error) {
		mints.Add(1)
		close(started)
		<-block
		return certFile, keyFile, nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		m.ensure(context.Background(), domain)
	}()
	<-started

	// With the first mint in flight, a concurrent ensure returns promptly
	// instead of queueing behind the mint lock.
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		m.ensure(context.Background(), domain)
	}()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent ensure blocked behind the in-flight mint")
	}
	if got := mints.Load(); got != 1 {
		t.Fatalf("mints = %d, want 1", got)
	}

	close(block)
	<-done
	if m.get() == nil {
		t.Fatal("blocked mint never landed a cert")
	}
}
