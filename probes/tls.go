package probes

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/guregu/null"
	"github.com/maddsua/pulse/config"
	"github.com/maddsua/pulse/storage"
)

func NewTlsProbe(label string, opts config.TlsProbeConfig, proxies config.ProxyConfigMap) (*tlsProbe, error) {

	hostname := opts.Host
	hostAddr := opts.Host
	if host, _, err := net.SplitHostPort(opts.Host); err == nil {
		hostname = host
	} else {
		hostAddr = net.JoinHostPort(opts.Host, "443")
	}

	if _, err := net.ResolveIPAddr("ip", hostname); err != nil {
		return nil, err
	}

	return &tlsProbe{
		probeTask: probeTask{
			nextRun:  time.Now().Add(time.Second * time.Duration(opts.Interval())),
			interval: time.Second * time.Duration(opts.Interval()),
			label:    label,
			timeout:  time.Second * time.Duration(opts.Timeout()),
		},
		host:     hostAddr,
		hostname: hostname,
		dialer:   &tls.Dialer{},
	}, nil
}

type tlsProbe struct {
	probeTask
	host     string
	hostname string
	dialer   *tls.Dialer
}

func (this *tlsProbe) Type() string {
	return "tls"
}

func (this *tlsProbe) Do(ctx context.Context, storageDriver storage.Storage) error {

	if err := this.probeTask.Lock(); err != nil {
		return err
	}

	defer this.probeTask.Unlock()

	started := time.Now()

	dialCtx, canceldial := context.WithTimeout(ctx, this.timeout)
	defer canceldial()

	stats, err := this.queryTargetTls(dialCtx)
	if err != nil {
		return this.dispatchEntry(storageDriver, storage.TlsSecurityEntry{
			Time:     time.Now(),
			Label:    this.label,
			Security: "none",
			Secure:   false,
		}, time.Since(started))
	}

	cert := this.findRelevantCert(stats.PeerCertificates)
	if cert == nil {
		return this.dispatchEntry(storageDriver, storage.TlsSecurityEntry{
			Time:     time.Now(),
			Label:    this.label,
			Security: "none",
			Secure:   false,
		}, time.Since(started))
	}

	hash := sha1.New()
	hash.Write(cert.Signature)

	return this.dispatchEntry(storageDriver, storage.TlsSecurityEntry{
		Time:            time.Now(),
		Label:           this.label,
		Security:        fmt.Sprintf("tls 1.%d", cert.Version),
		Secure:          true,
		CertSubject:     null.StringFrom(cert.Subject.String()),
		CertIssuer:      null.StringFrom(cert.Issuer.String()),
		CertExpires:     null.TimeFrom(cert.NotAfter),
		CertFingerprint: null.StringFrom(hex.EncodeToString(hash.Sum(nil))),
	}, time.Since(started))
}

func (this *tlsProbe) dispatchEntry(storageDriver storage.Storage, entry storage.TlsSecurityEntry, elapsed time.Duration) error {

	slog.Debug("upd tls "+this.label,
		slog.String("security", entry.Security),
		slog.String("issuer", entry.CertIssuer.String),
		slog.Time("expires", entry.CertExpires.Time),
		slog.Duration("elapsed", elapsed))

	return storageDriver.PushTlsEntry(entry)
}

func (this *tlsProbe) queryTargetTls(ctx context.Context) (tls.ConnectionState, error) {

	conn, err := this.dialer.DialContext(ctx, "tcp", this.host)
	if err != nil {
		return tls.ConnectionState{}, err
	}

	defer conn.Close()

	return conn.(*tls.Conn).ConnectionState(), nil
}

func (this *tlsProbe) findRelevantCert(certs []*x509.Certificate) *x509.Certificate {

	if len(certs) == 0 {
		return nil
	}

	for _, cert := range certs {
		for _, name := range cert.DNSNames {
			if name == this.hostname {
				return cert
			}
		}
	}

	return certs[0]
}
