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
	state map[string]*ruleState // "ruleID/agent/ID" | "ruleID/probe/ID" -> state
}

type ruleState struct {
	consecutive  int
	fired        bool // whether an alert_event is currently open
	lastNotifyAt time.Time
}

// stateKey 标识「一条规则的一个告警目标」。
// 必须带上目标，否则无 scope 的规则遍历多台 agent 时，
// 第一台触发后其余机器永远不会告警。
func stateKey(ruleID int64, agentID, probeID *int64) string {
	switch {
	case agentID != nil:
		return fmt.Sprintf("%d/agent/%d", ruleID, *agentID)
	case probeID != nil:
		return fmt.Sprintf("%d/probe/%d", ruleID, *probeID)
	default:
		return fmt.Sprintf("%d/-", ruleID)
	}
}

func NewEngine(db *storage.DB, notifier *Notifier, intervalS int) *Engine {
	return &Engine{
		db:       db,
		notifier: notifier,
		interval: time.Duration(intervalS) * time.Second,
		state:    make(map[string]*ruleState),
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
		e.state[stateKey(ev.RuleID, ev.AgentID, ev.ProbeID)] = &ruleState{
			fired:        true,
			lastNotifyAt: ev.FiredAt,
		}
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

// checkCondition 评估单个规则对单个目标的条件。
//
// 关键设计：锁内只碰内存状态机，所有 DB 查询和通知 goroutine 都在锁外。
// 旧实现整个函数持 e.mu（一个全局互斥锁）去做 DB 读写 + 起 goroutine，
// 规则多/目标多时（4 台机器 × 多个指标）会互相排队，一次慢查询能拖住所有告警评估。
func (e *Engine) checkCondition(rule models.AlertRule, agentID *int64, probeID *int64, name, address string, value float64) {
	triggered := compare(value, rule.Operator, rule.Threshold)

	e.mu.Lock()
	key := stateKey(rule.ID, agentID, probeID)
	st, ok := e.state[key]
	if !ok {
		st = &ruleState{}
		e.state[key] = st
	}

	if triggered {
		st.consecutive++

		// 持续时间要求：consecutive * interval >= duration_s
		if rule.DurationS > 0 && st.consecutive*int(e.interval.Seconds()) < rule.DurationS {
			e.mu.Unlock()
			return
		}

		if st.fired {
			// 已告警：只在开启重复通知且确实到了间隔时才再发一次
			if !rule.RepeatEnabled || rule.RepeatIntervalS <= 0 {
				e.mu.Unlock()
				return
			}
			interval := time.Duration(rule.RepeatIntervalS) * time.Second
			if !st.lastNotifyAt.IsZero() && time.Since(st.lastNotifyAt) < interval {
				e.mu.Unlock()
				return
			}
			prevNotify, hadPrev := st.lastNotifyAt, !st.lastNotifyAt.IsZero()
			st.lastNotifyAt = time.Now() // 先占位，避免并发 goroutine 重复推送
			e.mu.Unlock()

			lastEvent, err := e.db.GetLatestAlertEventByTarget(rule.ID, agentID, probeID)
			if err != nil || lastEvent == nil || lastEvent.ResolvedAt != nil {
				return
			}
			if !hadPrev && !lastEvent.FiredAt.IsZero() {
				if time.Since(lastEvent.FiredAt) < interval {
					e.mu.Lock()
					st.lastNotifyAt = prevNotify
					e.mu.Unlock()
					return
				}
			}
			go e.notifier.SendAlert(e.webhookFor(rule), lastEvent, &rule, e.agentOf(agentID))
			return
		}

		// 首次触发：先在锁内占住 fired，再出锁做冷却判断/写库/通知
		st.fired = true
		st.lastNotifyAt = time.Now()
		e.mu.Unlock()

		if rule.CooldownS > 0 {
			if lastEvent, err := e.db.GetLatestAlertEventByTarget(rule.ID, agentID, probeID); err == nil &&
				lastEvent != nil && lastEvent.ResolvedAt != nil &&
				time.Since(*lastEvent.ResolvedAt) < time.Duration(rule.CooldownS)*time.Second {
				// 仍在冷却期：撤销刚占的状态
				e.mu.Lock()
				st.fired = false
				e.mu.Unlock()
				return
			}
		}

		severity := "warning"
		if rule.Operator == "gt" || rule.Operator == "gte" {
			if value > rule.Threshold*1.5 {
				severity = "critical"
			}
		}

		metricName := map[string]string{
			"cpu":           "CPU 使用率",
			"memory":        "内存使用率",
			"load1":         "负载 1min",
			"load5":         "负载 5min",
			"load15":        "负载 15min",
			"probe_status":  "探测状态",
			"probe_latency": "探测延迟",
			"cert_expiry":   "证书剩余天数",
		}[rule.MetricType]
		if metricName == "" {
			metricName = rule.MetricType
		}
		opName := map[string]string{"gt": "＞", "lt": "＜", "gte": "≥", "lte": "≤", "eq": "=", "neq": "≠"}[rule.Operator]
		if opName == "" {
			opName = rule.Operator
		}
		message := fmt.Sprintf("%s %.1f %s %.1f", metricName, value, opName, rule.Threshold)

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
			e.mu.Lock()
			st.fired = false
			e.mu.Unlock()
			return
		}

		go e.notifier.SendAlert(e.webhookFor(rule), event, &rule, e.agentOf(agentID))
		return
	}

	// 条件已恢复
	if st.fired {
		st.fired = false
		st.consecutive = 0
		e.mu.Unlock()

		if err := e.db.ResolveAlertByTarget(rule.ID, agentID, probeID); err != nil {
			log.Printf("[alert] resolve for rule %d failed: %v", rule.ID, err)
		}
		lastEvent, err := e.db.GetLatestAlertEventByTarget(rule.ID, agentID, probeID)
		if err == nil && lastEvent != nil {
			go e.notifier.SendResolved(e.webhookFor(rule), lastEvent, &rule)
		}
		return
	}

	st.consecutive = 0
	e.mu.Unlock()
}

// webhookFor 返回该规则生效的 webhook（规则自带优先，否则用全局默认）。
func (e *Engine) webhookFor(rule models.AlertRule) string {
	if rule.WechatWebhook != "" {
		return rule.WechatWebhook
	}
	w, _ := e.db.GetSetting("default_wechat_webhook")
	return w
}

// agentOf 取 agent 详情（用于通知里带机器名），id 为 nil 时返回 nil。
func (e *Engine) agentOf(id *int64) *models.Agent {
	if id == nil {
		return nil
	}
	a, _ := e.db.GetAgent(*id)
	return a
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
