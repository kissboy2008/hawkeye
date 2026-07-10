package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"hawkeye/internal/models"
	"hawkeye/internal/server/storage"
)

// pollLogged tracks which agents have already logged their first successful poll.
var pollLogged sync.Map

// ActivePoller periodically pulls metrics from agents via HTTP.
type ActivePoller struct {
	db          *storage.DB
	onMetrics   func(int64, *models.AgentMetricsResponse)
	interval    time.Duration
	timeout     time.Duration
	httpClient  *http.Client
}

// NewActivePoller creates a new ActivePoller.
func NewActivePoller(
	db *storage.DB,
	onMetrics func(int64, *models.AgentMetricsResponse),
	intervalS, timeoutS int,
) *ActivePoller {
	return &ActivePoller{
		db:        db,
		onMetrics: onMetrics,
		interval:  time.Duration(intervalS) * time.Second,
		timeout:   time.Duration(timeoutS) * time.Second,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutS) * time.Second,
		},
	}
}

// Run starts the active poll loop. Blocks until ctx is cancelled.
func (ap *ActivePoller) Run(ctx context.Context) {
	if ap.interval <= 0 {
		ap.interval = 15 * time.Second
	}
	if ap.timeout <= 0 {
		ap.timeout = 10 * time.Second
		ap.httpClient.Timeout = ap.timeout
	}

	log.Printf("[active-poller] started (interval=%v, timeout=%v)", ap.interval, ap.timeout)
	ticker := time.NewTicker(ap.interval)
	defer ticker.Stop()

	// Do an immediate first poll
	ap.pollAll()

	for {
		select {
		case <-ctx.Done():
			log.Println("[active-poller] stopped")
			return
		case <-ticker.C:
			ap.pollAll()
		}
	}
}

func (ap *ActivePoller) pollAll() {
	agents, err := ap.db.GetAllAgents()
	if err != nil {
		log.Printf("[active-poller] error getting agents: %v", err)
		return
	}

	for _, agent := range agents {
		// Only poll agents configured for pull mode
		if agent.Mode != "pull" {
			continue
		}
		if agent.Address == "" {
			continue
		}
		ap.pollOne(agent)
	}
}

func (ap *ActivePoller) pollOne(agent models.Agent) {
	url := fmt.Sprintf("%s/api/v1/metrics", agent.Address)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("[active-poller] %s: bad URL: %v", agent.Name, err)
		return
	}

	if agent.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+agent.AuthToken)
	}

	resp, err := ap.httpClient.Do(req)
	if err != nil {
		log.Printf("[active-poller] %s (%s): request failed: %v", agent.Name, agent.Address, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		log.Printf("[active-poller] %s (%s): HTTP %d: %s", agent.Name, agent.Address, resp.StatusCode, string(body))
		return
	}

	var metrics models.AgentMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		log.Printf("[active-poller] %s: decode error: %v", agent.Name, err)
		return
	}

	// Store in database
	StoreMetrics(ap.db, agent.ID, &metrics)

	// Mark online
	if err := ap.db.UpdateAgentStatus(agent.ID, "online"); err != nil {
		log.Printf("[active-poller] %s: status update error: %v", agent.Name, err)
	}

	// Broadcast via WebSocket
	if ap.onMetrics != nil {
		ap.onMetrics(agent.ID, &metrics)
	}

	// Log only first successful poll per agent after startup
	if _, ok := pollLogged.Load(agent.ID); !ok {
		log.Printf("[active-poller] %s: polled successfully", agent.Name)
		pollLogged.Store(agent.ID, true)
	}
}
