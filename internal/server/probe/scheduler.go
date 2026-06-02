package probe

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"hawkeye/internal/models"
	"hawkeye/internal/server/storage"
)

// Scheduler periodically runs enabled web probes.
type Scheduler struct {
	db       *storage.DB
	interval time.Duration
	onResult func(result *models.ProbeResult) // callback for real-time push
	mu       sync.Mutex
	// track last run per probe to respect per-probe interval
	lastRun map[int64]time.Time
}

// NewScheduler creates a new probe scheduler.
func NewScheduler(db *storage.DB, intervalS int) *Scheduler {
	if intervalS <= 0 {
		intervalS = 60
	}
	return &Scheduler{
		db:       db,
		interval: time.Duration(intervalS) * time.Second,
		lastRun:  make(map[int64]time.Time),
	}
}

// OnResult sets a callback that fires when a probe result is produced.
func (s *Scheduler) OnResult(fn func(result *models.ProbeResult)) {
	s.onResult = fn
}

// Run starts the probe scheduling loop. Blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run immediately on start
	s.checkAll()

	for {
		select {
		case <-ctx.Done():
			log.Println("[probe-scheduler] stopped")
			return
		case <-ticker.C:
			s.checkAll()
		}
	}
}

func (s *Scheduler) checkAll() {
	probes, err := s.db.GetEnabledProbes()
	if err != nil {
		log.Printf("[probe-scheduler] error getting probes: %v", err)
		return
	}

	now := time.Now()
	for i := range probes {
		p := probes[i]

		// Respect per-probe interval
		interval := time.Duration(p.IntervalS) * time.Second
		if interval <= 0 {
			interval = s.interval
		}

		s.mu.Lock()
		last, ok := s.lastRun[p.ID]
		if ok && now.Sub(last) < interval {
			s.mu.Unlock()
			continue
		}
		s.lastRun[p.ID] = now
		s.mu.Unlock()

		go s.runProbe(&p)
	}
}

func (s *Scheduler) runProbe(probe *models.WebProbe) {
	result := ExecuteProbe(probe)
	if err := s.db.InsertProbeResult(result); err != nil {
		log.Printf("[probe-scheduler] error saving result for probe %d: %v", probe.ID, err)
	}

	if s.onResult != nil {
		s.onResult(result)
	}
}

// ExecuteProbe performs an HTTP probe check and returns the result.
// This is the shared probe execution logic used by both the scheduler and the API handler.
func ExecuteProbe(probe *models.WebProbe) *models.ProbeResult {
	result := &models.ProbeResult{
		ProbeID:   probe.ID,
		Timestamp: time.Now().UTC(),
	}

	// Parse URL for TLS config
	u, _ := url.Parse(probe.URL)
	serverName := u.Hostname()

	client := &http.Client{
		Timeout: time.Duration(probe.TimeoutMs) * time.Millisecond,
		// Ensure TLS handshake happens so we can grab certs
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				// Skip verification so we can still grab certs from self-signed sites
				InsecureSkipVerify: true,
				ServerName:         serverName,
			},
		},
	}

	start := time.Now()
	resp, err := client.Get(probe.URL)
	elapsed := time.Since(start)

	if err != nil {
		result.Error = err.Error()
		result.Success = false
		result.LatencyMs = float64(elapsed.Milliseconds())
		// Try to grab cert info even on TLS error
		result.CertIssuer, result.CertNotAfter, result.CertDaysLeft = fetchCertInfo(u.Host)
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.LatencyMs = float64(elapsed.Milliseconds())
	result.Success = resp.StatusCode == probe.ExpectedStatus
	if !result.Success {
		result.Error = "unexpected status code"
	}

	// Grab SSL certificate info for HTTPS URLs
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		result.CertIssuer = cert.Issuer.CommonName
		notAfter := cert.NotAfter.UTC()
		notAfterStr := notAfter.Format("2006-01-02 15:04:05")
		daysLeft := int(time.Until(cert.NotAfter).Hours() / 24)
		result.CertNotAfter = &notAfterStr
		result.CertDaysLeft = &daysLeft
	}

	return result
}

// fetchCertInfo does a raw TLS handshake to extract certificate info even when the HTTP request fails.
func fetchCertInfo(host string) (issuer string, notAfter *string, daysLeft *int) {
	if !strings.Contains(host, ":") {
		host = host + ":443"
	}
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 5 * time.Second},
		"tcp", host,
		&tls.Config{InsecureSkipVerify: true},
	)
	if err != nil || len(conn.ConnectionState().PeerCertificates) == 0 {
		return
	}
	defer conn.Close()
	cert := conn.ConnectionState().PeerCertificates[0]
	issuer = cert.Issuer.CommonName
	na := cert.NotAfter.UTC().Format("2006-01-02 15:04:05")
	dl := int(time.Until(cert.NotAfter).Hours() / 24)
	notAfter = &na
	daysLeft = &dl
	return
}
