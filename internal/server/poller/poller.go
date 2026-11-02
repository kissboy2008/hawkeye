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
		if err := db.UpdateAgentVersion(agentID, m.AgentVersion); err != nil {
			log.Printf("[metrics] update agent %d version failed: %v", agentID, err)
		}
	}

	// 三条指标放在一个事务里写：原来 3 次独立 INSERT = 3 次拿写锁 + 3 次提交，
	// 在高频 push（多 agent 每 30s）下是最容易撞 SQLITE_BUSY 的地方。
	items := make(map[string]string, 3)
	if cpuData, err := json.Marshal(m.CPU); err == nil {
		items["cpu"] = string(cpuData)
	} else {
		log.Printf("[metrics] agent %d marshal cpu failed: %v", agentID, err)
	}
	if memData, err := json.Marshal(m.Memory); err == nil {
		items["memory"] = string(memData)
	} else {
		log.Printf("[metrics] agent %d marshal memory failed: %v", agentID, err)
	}
	if m.UptimeS > 0 {
		items["uptime"] = fmt.Sprintf(`{"uptime_seconds":%d}`, m.UptimeS)
	}

	if err := db.InsertMetricsBatch(agentID, items); err != nil {
		log.Printf("[metrics] agent %d store failed: %v", agentID, err)
	}
}
