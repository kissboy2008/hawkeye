package alert

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"hawkeye/internal/models"
	"hawkeye/internal/server/storage"
)

// Engine evaluates alert rules periodically.
type Engine struct {
	db       *storage.DB
	notifier *Notifier
	interval time.Duration

	mu    sync.Mutex
	state map[int64]*ruleState // ruleID -> state
}

type ruleState struct {
	consecutive  int
	fired        bool // whether an alert_event is currently open
	lastNotifyAt time.Time
}

func NewEngine(db *storage.DB, notifier *Notifier, intervalS int) *Engine {
	return &Engine{
		db:       db,
		notifier: notifier,
		interval: time.Duration(intervalS) * time.Second,
		state:    make(map[int64]*ruleState),
	}
}

// Run starts the alert evaluation loop. Blocks until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	// Restore state from unresolved events on startup
	e.restoreState()

	if e.interval <= 0 {
		e.interval = 15 * time.Second
		log.Printf("[alert] engine interval was %v, defaulting to 15s", e.interval)
	}
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[alert] engine stopped")
			return
		case <-ticker.C:
			e.evaluateAll()
		}
	}
}

// restoreState recovers alert state from unresolved events in the database.
// This prevents duplicate alerts after a server restart.
func (e *Engine) restoreState() {
	events, err := e.db.GetUnresolvedAlertEvents()
	if err != nil {
		log.Printf("[alert] failed to restore state: %v", err)
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for _, ev := range events {
		e.state[ev.RuleID] = &ruleState{
			fired:        true,
			lastNotifyAt: ev.FiredAt,
		}
	}

	if len(events) > 0 {
		log.Printf("[alert] restored %d active alert state(s) from database", len(events))
	}
}

func (e *Engine) evaluateAll() {
	rules, err := e.db.GetEnabledAlertRules()
	if err != nil {
		log.Printf("[alert] error getting rules: %v", err)
		return
	}

	for _, rule := range rules {
		e.evaluateRule(rule)
	}
}

func (e *Engine) evaluateRule(rule models.AlertRule) {
	switch rule.ScopeType {
	case "agent":
		e.evaluateAgentRule(rule)
	case "probe":
		e.evaluateProbeRule(rule)
	}
}

func (e *Engine) evaluateAgentRule(rule models.AlertRule) {
	// Determine which agents to check
	var agents []models.Agent
	var err error
	if rule.ScopeID != nil {
		agent, err := e.db.GetAgent(*rule.ScopeID)
		if err != nil || agent == nil {
			return
		}
		agents = []models.Agent{*agent}
	} else {
		agents, err = e.db.GetAgentsByStatus("online")
		if err != nil {
			return
		}
	}

	for _, agent := range agents {
		// Get latest metric
		data, err := e.db.GetLatestMetric(agent.ID, rule.MetricType)
		if err != nil || data == "" {
			continue
		}

		value := extractValue(data, rule.MetricType)
		agentID := agent.ID
		e.checkCondition(rule, &agentID, nil, agent.Name, agent.Address, value)
	}
}

func (e *Engine) evaluateProbeRule(rule models.AlertRule) {
	var probes []models.WebProbe
	var err error

	if rule.ScopeID != nil {
		probe, err := e.db.GetProbe(*rule.ScopeID)
		if err != nil || probe == nil {
			return
		}
		probes = []models.WebProbe{*probe}
	} else {
		probes, err = e.db.GetEnabledProbes()
		if err != nil {
			return
		}
	}

	for _, probe := range probes {
		result, err := e.db.GetLatestProbeResult(probe.ID)
		if err != nil || result == nil {
			continue
		}

		var value float64
		switch rule.MetricType {
		case "probe_status":
			if result.Success {
				value = 1
			} else {
				value = 0
			}
		case "probe_latency":
			value = result.LatencyMs
		case "cert_expiry":
			if result.CertDaysLeft != nil {
				value = float64(*result.CertDaysLeft)
			} else {
				continue // no cert info, skip
			}
		}

		e.checkCondition(rule, nil, &probe.ID, probe.Name, probe.URL, value)
	}
}

func (e *Engine) checkCondition(rule models.AlertRule, agentID *int64, probeID *int64, name, address string, value float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	st, ok := e.state[rule.ID]
	if !ok {
		st = &ruleState{}
		e.state[rule.ID] = st
	}

	triggered := compare(value, rule.Operator, rule.Threshold)

	if triggered {
		st.consecutive++

		// Check duration requirement
		// consecutive * interval >= duration_s
		if rule.DurationS > 0 && st.consecutive*int(e.interval.Seconds()) < rule.DurationS {
			return
		}

		// Check if already fired
		if st.fired {
			// Check repeat notification for open events
			lastEvent, _ := e.db.GetLatestAlertEventByRule(rule.ID)
			if lastEvent != nil && lastEvent.ResolvedAt == nil && rule.RepeatEnabled && rule.RepeatIntervalS > 0 {
				if st.lastNotifyAt.IsZero() {
					st.lastNotifyAt = lastEvent.FiredAt
				}
				if time.Since(st.lastNotifyAt) >= time.Duration(rule.RepeatIntervalS)*time.Second {
					webhook := rule.WechatWebhook
					if webhook == "" {
						webhook, _ = e.db.GetSetting("default_wechat_webhook")
					}
					var agent *models.Agent
					if agentID != nil {
						agent, _ = e.db.GetAgent(*agentID)
					}
					go e.notifier.SendAlert(webhook, lastEvent, &rule, agent)
					st.lastNotifyAt = time.Now()
				}
			}
			return
		}

		// Check cooldown: don't re-fire if last event was resolved within cooldown period
		if rule.CooldownS > 0 {
			lastEvent, _ := e.db.GetLatestAlertEventByRule(rule.ID)
			if lastEvent != nil && lastEvent.ResolvedAt != nil {
				if time.Since(*lastEvent.ResolvedAt) < time.Duration(rule.CooldownS)*time.Second {
					return // still in cooldown
				}
			}
		}

		// Fire alert
		severity := "warning"
		if rule.Operator == "gt" || rule.Operator == "gte" {
			// High values are typically more severe
			if value > rule.Threshold*1.5 {
				severity = "critical"
			}
		}

		message := fmt.Sprintf("> **%s**: %.1f %s %.1f", rule.MetricType, value, rule.Operator, rule.Threshold)

		event := &models.AlertEvent{
			RuleID:   rule.ID,
			Severity: severity,
			Message:  message,
			Value:    &value,
		}
		if agentID != nil {
			event.AgentID = agentID
		}
		if probeID != nil {
			event.ProbeID = probeID
		}
		if _, err := e.db.CreateAlertEvent(event); err != nil {
			log.Printf("[alert] failed to create event: %v", err)
			return
		}

		st.fired = true
		st.lastNotifyAt = time.Now()

		// Send notification
		webhook := rule.WechatWebhook
		if webhook == "" {
			webhook, _ = e.db.GetSetting("default_wechat_webhook")
		}

		var agent *models.Agent
		if agentID != nil {
			agent, _ = e.db.GetAgent(*agentID)
		}

		go e.notifier.SendAlert(webhook, event, &rule, agent)

	} else {
		// Condition cleared
		if st.fired {
			// Resolve the alert
			e.db.ResolveAlertByRule(rule.ID)
			st.fired = false
			st.consecutive = 0

			// Send resolved notification
			webhook := rule.WechatWebhook
			if webhook == "" {
				webhook, _ = e.db.GetSetting("default_wechat_webhook")
			}
			lastEvent, _ := e.db.GetLatestAlertEventByRule(rule.ID)
			if lastEvent != nil {
				go e.notifier.SendResolved(webhook, lastEvent, &rule)
			}
		}
		st.consecutive = 0
	}
}

// compare evaluates: value OPERATOR threshold
func compare(value float64, operator string, threshold float64) bool {
	switch operator {
	case "gt":
		return value > threshold
	case "lt":
		return value < threshold
	case "gte":
		return value >= threshold
	case "lte":
		return value <= threshold
	case "eq":
		return math.Abs(value-threshold) < 0.001
	case "neq":
		return math.Abs(value-threshold) >= 0.001
	default:
		return false
	}
}

// extractValue gets a representative float from metric JSON based on metric type.
func extractValue(jsonStr, metricType string) float64 {
	switch metricType {
	case "cpu":
		return models.ExtractMetricValue(jsonStr, "usage_percent")
	case "memory":
		return models.ExtractMetricValue(jsonStr, "usage_percent")
	case "load1":
		return models.ExtractMetricValue(jsonStr, "load1")
	case "load5":
		return models.ExtractMetricValue(jsonStr, "load5")
	case "load15":
		return models.ExtractMetricValue(jsonStr, "load15")
	default:
		return models.ExtractMetricValue(jsonStr, "usage_percent")
	}
}
