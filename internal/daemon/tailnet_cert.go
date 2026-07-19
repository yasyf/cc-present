package daemon

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yasyf/synckit/meshtrust"
)

// certRefreshWindow is how close to expiry the tailnet cert may get before the
// next ensure re-mints it.
const certRefreshWindow = 30 * 24 * time.Hour

// mintTimeout bounds one `tailscale cert` issuance so a hung tailscaled can't
// pin the mint lock forever.
const mintTimeout = 3 * time.Minute

// errNoCert fails a TLS handshake while no valid cert is available — before
// the first successful mint, or after expiry outlasts every renewal attempt.
var errNoCert = errors.New("tailnet: no valid TLS certificate")

// mintedCert pairs a loaded certificate with the domain it was minted for, so
// ensure re-mints when the tailnet's cert domain changes.
type mintedCert struct {
	domain string
	cert   tls.Certificate
}

// certManager holds the daemon's tailnet TLS certificate, minted for the
// MagicDNS cert domain via `tailscale cert` and re-minted as expiry
// approaches. ensure runs off the serving path (a boot goroutine and the
// reconcile ticker), never inside a TLS handshake. mint and now are injected
// boundaries; newCertManager wires the real ones.
type certManager struct {
	dir  string
	mint func(ctx context.Context, dnsName, dir string) (string, string, error)
	now  func() time.Time

	mintMu sync.Mutex
	cert   atomic.Pointer[mintedCert]
}

// newCertManager returns a manager that mints certs into dir.
func newCertManager(dir string) *certManager {
	return &certManager{dir: dir, mint: meshtrust.MintCert, now: time.Now}
}

// ensure mints the cert for domain — again on a domain change or within
// certRefreshWindow of expiry — and swaps it into the served pointer. An
// empty domain is a no-op; a mint already in flight is skipped, not queued
// behind (the next tick retries). Mint and load failures warn and leave the
// previously served cert as-is.
func (m *certManager) ensure(ctx context.Context, domain string) {
	if domain == "" {
		return
	}
	if !m.mintMu.TryLock() {
		return
	}
	defer m.mintMu.Unlock()
	if c := m.cert.Load(); c != nil && c.domain == domain && m.now().Add(certRefreshWindow).Before(c.cert.Leaf.NotAfter) {
		return
	}
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		slog.Warn("tailnet: create cert dir", "dir", m.dir, "err", err)
		return
	}
	mctx, cancel := context.WithTimeout(ctx, mintTimeout)
	defer cancel()
	certFile, keyFile, err := m.mint(mctx, domain, m.dir)
	if err != nil {
		slog.Warn("tailnet: mint cert", "domain", domain, "err", err)
		return
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		slog.Warn("tailnet: load cert", "domain", domain, "err", err)
		return
	}
	// Parse the leaf explicitly: LoadX509KeyPair only populates Leaf when the
	// x509keypairleaf GODEBUG default is in effect, and expiry checks need it.
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		slog.Warn("tailnet: parse cert leaf", "domain", domain, "err", err)
		return
	}
	cert.Leaf = leaf
	m.cert.Store(&mintedCert{domain: domain, cert: cert})
	slog.Info("tailnet: cert ready", "domain", domain, "notAfter", cert.Leaf.NotAfter)
}

func (m *certManager) valid() *mintedCert {
	c := m.cert.Load()
	if c == nil || !m.now().Before(c.cert.Leaf.NotAfter) {
		return nil
	}
	return c
}

// get returns the served cert; nil before the first successful mint and nil
// again once the leaf expires, so an expired cert is never served and display
// falls back to http on raw IPs.
func (m *certManager) get() *tls.Certificate {
	if c := m.valid(); c != nil {
		return &c.cert
	}
	return nil
}

// mintedDomain names the domain the served cert actually covers, empty when
// no valid cert is held — the display path keys https URLs on it so a printed
// host always matches the handshake.
func (m *certManager) mintedDomain() string {
	if c := m.valid(); c != nil {
		return c.domain
	}
	return ""
}

// GetCertificate is the tls.Config callback: the cached cert, or an error
// that fails the handshake while no valid cert is held.
func (m *certManager) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	if c := m.get(); c != nil {
		return c, nil
	}
	return nil, errNoCert
}

// tlsConfig builds the server-side TLS config for a dual-mode tailnet leg.
// h2 is offered deliberately: http.Server.Serve configures HTTP/2 for
// hand-wrapped *tls.Conn values when Server.TLSConfig is nil, which is how
// cc-interact's shared server runs.
func (m *certManager) tlsConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: m.GetCertificate,
		MinVersion:     tls.VersionTLS12,
		NextProtos:     []string{"h2", "http/1.1"},
	}
}
