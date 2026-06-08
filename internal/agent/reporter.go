package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"hawkeye/internal/config"
	"hawkeye/internal/models"
)

// Reporter periodically pushes metrics to the server.
type Reporter struct {
	collector  *Collector
	serverURL  string
	token      string
	version    string
	interval   time.Duration
	httpClient *http.Client
}

// NewReporter creates a new push reporter.
func NewReporter(c *Collector, cfg *config.AgentConfig, version string) *Reporter {
	interval := time.Duration(cfg.Server.PushIntervalS) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Reporter{
		collector:  c,
		serverURL:  cfg.Server.URL,
		token:      cfg.Auth.Token,
		version:    version,
		interval:   interval,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Run starts the push loop. Blocks until ctx is cancelled.
// Only starts if serverURL is configured.
// Uses exponential backoff on consecutive failures (30s → 60s → 120s → max 5min).
func (r *Reporter) Run(ctx context.Context) {
	if r.serverURL == "" {
		log.Println("[reporter] server.url not configured, push mode disabled")
		return
	}

	pushURL := strings.TrimRight(r.serverURL, "/") + "/api/v1/agents/push"
	log.Printf("[reporter] push mode enabled, posting to %s every %v", pushURL, r.interval)

	// Push immediately on start
	r.pushOnce(pushURL)

	consecutiveFailures := 0
	const maxBackoff = 5 * time.Minute

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[reporter] stopped")
			return
		case <-ticker.C:
			if r.pushOnce(pushURL) {
				// Success — reset backoff and restore normal interval
				if consecutiveFailures > 0 {
					log.Printf("[reporter] connection restored after %d failures", consecutiveFailures)
					consecutiveFailures = 0
					ticker.Reset(r.interval)
				}
			} else {
				// Failure — increase backoff
				consecutiveFailures++
				backoff := r.interval * time.Duration(1<<uint(consecutiveFailures))
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				ticker.Reset(backoff)
				if consecutiveFailures <= 3 {
					log.Printf("[reporter] backing off to %v after %d consecutive failures", backoff, consecutiveFailures)
				}
			}
		}
	}
}

// pushRequest is the JSON body sent to the server.
type pushRequest struct {
	Token     string               `json:"token"`
	Hostname  string               `json:"hostname"`
	Version   string               `json:"version"`
	Timestamp time.Time            `json:"timestamp"`
	CPU       models.CpuMetrics    `json:"cpu"`
	Memory    models.MemoryMetrics `json:"memory"`
	UptimeS   uint64               `json:"uptime_seconds"`
}

func (r *Reporter) pushOnce(pushURL string) bool {
	metrics, err := r.collector.CollectMetrics()
	if err != nil {
		log.Printf("[reporter] error collecting metrics: %v", err)
		return false
	}

	body := pushRequest{
		Token:     r.token,
		Hostname:  metrics.Hostname,
		Version:   r.version,
		Timestamp: metrics.Timestamp,
		CPU:       metrics.CPU,
		Memory:    metrics.Memory,
		UptimeS:   metrics.UptimeS,
	}

	data, err := json.Marshal(body)
	if err != nil {
		log.Printf("[reporter] error marshaling metrics: %v", err)
		return false
	}

	req, err := http.NewRequest("POST", pushURL, bytes.NewReader(data))
	if err != nil {
		log.Printf("[reporter] error building request: %v", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		log.Printf("[reporter] push failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[reporter] push rejected: status %d", resp.StatusCode)
		return false
	}

	return true
}
