package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"hawkeye/internal/models"
	"hawkeye/internal/server/storage"
)

// HeartbeatChecker periodically checks for agents that haven't reported recently
// and marks them as offline.
type HeartbeatChecker struct {
	db       *storage.DB
	interval time.Duration
	timeout  time.Duration // time after which an agent is considered offline
}

func NewChecker(db *storage.DB, checkIntervalS, offlineTimeoutS int) *HeartbeatChecker {
	return &HeartbeatChecker{
		db:       db,
		interval: time.Duration(checkIntervalS) * time.Second,
		timeout:  time.Duration(offlineTimeoutS) * time.Second,
	}
}

// Run starts the heartbeat check loop. Blocks until ctx is cancelled.
func (hc *HeartbeatChecker) Run(ctx context.Context) {
	if hc.interval <= 0 {
		hc.interval = 15 * time.Second
	}
	if hc.timeout <= 0 {
		hc.timeout = 60 * time.Second
	}

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[heartbeat] stopped")
			return
		case <-ticker.C:
			hc.checkOffline()
		}
	}
}

func (hc *HeartbeatChecker) checkOffline() {
	agents, err := hc.db.GetAllAgents()
	if err != nil {
		log.Printf("[heartbeat] error getting agents: %v", err)
		return
	}

	cutoff := time.Now().Add(-hc.timeout)
	for _, agent := range agents {
		if agent.Status == "online" && agent.LastSeen != nil && agent.LastSeen.Before(cutoff) {
			hc.db.UpdateAgentStatus(agent.ID, "offline")
			log.Printf("[heartbeat] agent %s (%d) marked offline (last seen %v)",
				agent.Name, agent.ID, agent.LastSeen)
		}
	}
}

// StoreMetrics stores metrics for an agent. Exported for use by push handler.
func StoreMetrics(db *storage.DB, agentID int64, m *models.AgentMetricsResponse) {
	// Update agent version if present (for pull mode agents)
	if m.AgentVersion != "" {
		_ = db.UpdateAgentVersion(agentID, m.AgentVersion)
	}
	// CPU
	if cpuData, err := json.Marshal(m.CPU); err == nil {
		db.InsertMetric(agentID, "cpu", string(cpuData))
	}

	// Memory
	if memData, err := json.Marshal(m.Memory); err == nil {
		db.InsertMetric(agentID, "memory", string(memData))
	}

	// Uptime — store as a simple JSON number
	if m.UptimeS > 0 {
		uptimeData := fmt.Sprintf(`{"uptime_seconds":%d}`, m.UptimeS)
		db.InsertMetric(agentID, "uptime", uptimeData)
	}
}
