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

// Engine 是 v2 告警引擎。
//
// 与 v1 的根本区别：
//   - 评估单位是「规则 × 目标」（alert_rule_targets 一行），每台机器/每个网站独立阈值
//   - 状态落库（alert_rule_state），first_true_at 用墙钟，重启不清零、精度不受周期限制
//   - 一次评估中同一规则的多条告警合并成一条消息发出，不再刷屏
//   - 通知失败进 alert_outbox 兜底队列，进程重启不丢
type Engine struct {
	db       *storage.DB
	notifier *Notifier
	interval time.Duration

	mu       sync.Mutex
	health   Health
	inflight map[int64]bool // 正在发送中的规则，防止重复刷
}

// Health 是引擎自监控快照（面板健康条用）。
type Health struct {
	LastEvalAt   time.Time `json:"last_eval_at"`
	LastEvalMs   int64     `json:"last_eval_ms"`
	Rules        int       `json:"rules"`
	Targets      int       `json:"targets"`
	Firing       int       `json:"firing"`
	Pending      int       `json:"pending"`
	Silenced     int       `json:"silenced"`
	OutboxQueued int       `json:"outbox_queued"`

	// BeforeEvalAt 供 dead-man 判断：超过 3 个周期没评估就说明引擎卡了
	intervalS int
}

func NewEngine(db *storage.DB, notifier *Notifier, intervalS int) *Engine {
	return &Engine{
		db:       db,
		notifier: notifier,
		interval: time.Duration(intervalS) * time.Second,
		inflight: make(map[int64]bool),
		health:   Health{intervalS: intervalS},
	}
}

func (e *Engine) Health() Health {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.health
}

// Run 启动评估循环 + 通知兜底队列重试。
func (e *Engine) Run(ctx context.Context) {
	if e.interval <= 0 {
		e.interval = 15 * time.Second
		log.Printf("[alert] engine interval was 0, defaulting to 15s")
	}

	// 启动即评估一次，重启后不用等一个周期
	e.evaluateAll()

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	outboxTicker := time.NewTicker(15 * time.Second)
	defer outboxTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.evaluateAll()
		case <-outboxTicker.C:
			e.flushOutbox()
		}
	}
}

// ---------- 评估 ----------

type notifyItem struct {
	Target  string
	Message string
}

type notifyBatch struct {
	Rule     models.AlertRule
	Fired    []notifyItem
	Resolved []notifyItem
}

func (e *Engine) evaluateAll() {
	start := time.Now()
	rules, err := e.db.GetEnabledAlertRules()
	if err != nil {
		log.Printf("[alert] error getting rules: %v", err)
		return
	}

	batches := make(map[int64]*notifyBatch)
	targetCount := 0

	for _, rule := range rules {
		b := e.evaluateRule(rule, &targetCount)
		if b != nil && (len(b.Fired) > 0 || len(b.Resolved) > 0) {
			batches[rule.ID] = b
		}
	}

	for _, b := range batches {
		e.dispatch(b)
	}

	// 自监控快照
	counts, _ := e.db.CountAlertStates()
	outbox, _ := e.db.CountOutboxByStatus()
	e.mu.Lock()
	e.health.LastEvalAt = time.Now()
	e.health.LastEvalMs = time.Since(start).Milliseconds()
	e.health.Rules = len(rules)
	e.health.Targets = targetCount
	e.health.Firing = counts["firing"]
	e.health.Pending = counts["pending"]
	e.health.Silenced = counts["silenced"]
	e.health.OutboxQueued = outbox["pending"] + outbox["failed"]
	e.mu.Unlock()
}

// evaluateRule 评估一条规则下的所有目标，把触发/恢复收集成一个批次。
func (e *Engine) evaluateRule(rule models.AlertRule, targetCount *int) *notifyBatch {
	targets, err := e.db.GetAlertRuleTargets(rule.ID)
	if err != nil {
		log.Printf("[alert] rule %d: get targets: %v", rule.ID, err)
		return nil
	}
	batch := &notifyBatch{Rule: rule}
	e.fillTargetNames(rule, targets)

	for _, t := range targets {
		if !t.Enabled {
			continue
		}
		*targetCount++
		fired, resolved := e.evaluateTarget(rule, t)
		if fired != nil {
			batch.Fired = append(batch.Fired, *fired)
		}
		if resolved != nil {
			batch.Resolved = append(batch.Resolved, *resolved)
		}
	}
	return batch
}

// fillTargetNames 给目标行补上名字/地址，通知文案和面板都要显示「哪台」。
func (e *Engine) fillTargetNames(rule models.AlertRule, targets []models.AlertRuleTarget) {
	agents := map[int64]*models.Agent{}
	probes := map[int64]*models.WebProbe{}
	for i := range targets {
		t := &targets[i]
		switch t.TargetType {
		case ClassAgent:
			a, ok := agents[t.TargetID]
			if !ok {
				a, _ = e.db.GetAgent(t.TargetID)
				agents[t.TargetID] = a
			}
			if a != nil {
				t.TargetName = a.Name
				t.TargetAddr = a.Address
			}
		case ClassProbe:
			p, ok := probes[t.TargetID]
			if !ok {
				p, _ = e.db.GetProbe(t.TargetID)
				probes[t.TargetID] = p
			}
			if p != nil {
				t.TargetName = p.Name
				t.TargetAddr = p.URL
			}
		}
	}
}

// evaluateTarget 是核心状态机。返回 (新触发, 已恢复)，两者都可能是 nil。
func (e *Engine) evaluateTarget(rule models.AlertRule, t models.AlertRuleTarget) (*notifyItem, *notifyItem) {
	def, _ := MetricDefOf(rule.MetricType)

	value, ok := e.sample(rule, t, def)
	if !ok {
		return nil, nil // 还没数据，跳过（不写状态，避免误判为恢复）
	}

	// 生效参数：目标覆盖 > 规则默认
	op := t.Operator
	if op == "" {
		op = rule.Operator
	}
	if op == "" && rule.Kind == KindAvailability {
		op = "gte"
	}
	if op == "" {
		op = def.DefaultOperator
	}
	threshold := rule.Threshold
	if t.Threshold != nil {
		threshold = *t.Threshold
	}
	forS := rule.DurationS
	if t.ForS != nil {
		forS = *t.ForS
	}

	triggered := compare(value, op, threshold)
	now := time.Now().UTC()

	st, err := e.db.GetAlertRuleState(rule.ID, t.TargetType, t.TargetID)
	if err != nil {
		log.Printf("[alert] get state: %v", err)
		return nil, nil
	}
	if st == nil {
		st = &models.AlertRuleState{
			RuleID: rule.ID, TargetType: t.TargetType, TargetID: t.TargetID, Status: "ok",
		}
	}
	st.LastEvalAt = &now
	st.LastValue = value

	silenced := isSilenced(rule.SilencedUntil, t.SilencedUntil, now)
	agentID, probeID := targetIDs(t)

	// ---------- 条件不成立 ----------
	if !triggered {
		st.FirstTrueAt = nil
		if st.Status == "firing" {
			if err := e.db.ResolveAlertByTarget(rule.ID, agentID, probeID); err != nil {
				log.Printf("[alert] resolve rule %d target %s/%d: %v", rule.ID, t.TargetType, t.TargetID, err)
			}
			st.Status = "ok"
			st.FiredAt = nil
			st.NotifyCount = 0
			_ = e.db.SaveAlertRuleState(st)
			return nil, &notifyItem{Target: t.TargetName, Message: fmt.Sprintf("已恢复（最新值 %.1f）", value)}
		}
		st.Status = "ok"
		_ = e.db.SaveAlertRuleState(st)
		return nil, nil
	}

	// ---------- 条件成立 ----------
	if st.FirstTrueAt == nil {
		st.FirstTrueAt = &now
	}
	held := now.Sub(*st.FirstTrueAt)

	// 还没在告警态：先看持续时长，再看静默，再看恢复后防抖
	if st.Status != "firing" {
		if forS > 0 && held < time.Duration(forS)*time.Second {
			st.Status = "pending"
			_ = e.db.SaveAlertRuleState(st)
			return nil, nil
		}
		if silenced {
			st.Status = "silenced"
			st.SuppressedCount++
			_ = e.db.SaveAlertRuleState(st)
			return nil, nil
		}
		if rule.CooldownS > 0 {
			if last, err := e.db.GetLatestAlertEventByTarget(rule.ID, agentID, probeID); err == nil &&
				last != nil && last.ResolvedAt != nil &&
				now.Sub(*last.ResolvedAt) < time.Duration(rule.CooldownS)*time.Second {
				st.Status = "pending"
				_ = e.db.SaveAlertRuleState(st)
				return nil, nil
			}
		}

		message := buildMessage(rule, def, t, value, op, threshold)
		severity := severityOf(value, threshold, op)
		event := &models.AlertEvent{
			RuleID:   rule.ID,
			Severity: severity,
			Message:  message,
			Value:    &value,
			AgentID:  agentID,
			ProbeID:  probeID,
		}
		if _, err := e.db.CreateAlertEvent(event); err != nil {
			log.Printf("[alert] create event rule %d: %v", rule.ID, err)
			return nil, nil
		}
		st.Status = "firing"
		st.FiredAt = &now
		st.LastNotifyAt = &now
		st.NotifyCount = 1
		_ = e.db.SaveAlertRuleState(st)
		return &notifyItem{Target: t.TargetName, Message: message}, nil
	}

	// 已在告警态：静默只压通知，状态照旧 firing
	if silenced {
		st.SuppressedCount++
		_ = e.db.SaveAlertRuleState(st)
		return nil, nil
	}
	if rule.RepeatEnabled && rule.RepeatIntervalS > 0 {
		if st.LastNotifyAt == nil || now.Sub(*st.LastNotifyAt) >= time.Duration(rule.RepeatIntervalS)*time.Second {
			st.LastNotifyAt = &now
			st.NotifyCount++
			_ = e.db.SaveAlertRuleState(st)
			message := "仍未恢复：" + buildMessage(rule, def, t, value, op, threshold)
			return &notifyItem{Target: t.TargetName, Message: message}, nil
		}
	}
	_ = e.db.SaveAlertRuleState(st)
	return nil, nil
}

// sample 取一个目标在当前指标上的值。
func (e *Engine) sample(rule models.AlertRule, t models.AlertRuleTarget, def MetricDef) (float64, bool) {
	switch t.TargetType {
	case ClassAgent:
		agent, err := e.db.GetAgent(t.TargetID)
		if err != nil || agent == nil {
			return 0, false
		}
		if def.Availability {
			since := agent.CreatedAt
			if agent.LastSeen != nil {
				since = *agent.LastSeen
			}
			v := time.Since(since).Minutes()
			if v < 0 {
				v = 0
			}
			return v, true
		}
		data, err := e.db.GetLatestMetric(agent.ID, rule.MetricType)
		if err != nil || data == "" {
			return 0, false
		}
		return extractAgentValue(data, def), true

	case ClassProbe:
		res, err := e.db.GetLatestProbeResult(t.TargetID)
		if err != nil || res == nil {
			return 0, false
		}
		switch rule.MetricType {
		case "probe_status":
			if res.Success {
				return 1, true
			}
			return 0, true
		case "probe_latency":
			return res.LatencyMs, true
		case "cert_expiry":
			if res.CertDaysLeft == nil {
				return 0, false
			}
			return float64(*res.CertDaysLeft), true
		}
	}
	return 0, false
}

func targetIDs(t models.AlertRuleTarget) (*int64, *int64) {
	id := t.TargetID
	if t.TargetType == ClassProbe {
		return nil, &id
	}
	return &id, nil
}

func isSilenced(ruleUntil, targetUntil *time.Time, now time.Time) bool {
	for _, u := range []*time.Time{ruleUntil, targetUntil} {
		if u != nil && now.Before(*u) {
			return true
		}
	}
	return false
}

func severityOf(value, threshold float64, op string) string {
	if (op == "gt" || op == "gte") && threshold > 0 && value > threshold*1.5 {
		return "critical"
	}
	return "warning"
}

func buildMessage(rule models.AlertRule, def MetricDef, t models.AlertRuleTarget, value float64, op string, threshold float64) string {
	if rule.Kind == KindAvailability || def.Availability {
		return fmt.Sprintf("已离线 %.0f 分钟（阈值 %.0f 分钟）", value, threshold)
	}
	label := def.Label
	if label == "" {
		label = rule.MetricType
	}
	switch def.Unit {
	case "ms":
		return fmt.Sprintf("%s %.0f ms %s %.0f ms", label, value, opSymbol(op), threshold)
	case "days":
		return fmt.Sprintf("%s %.0f 天 %s %.0f 天", label, value, opSymbol(op), threshold)
	default:
		return fmt.Sprintf("%s %.1f %s %.1f", label, value, opSymbol(op), threshold)
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

// ---------- 通知 ----------

// dispatch 把一批触发/恢复合并成一条消息发送。
// 4 台机器同时掉线 → 1 条消息，不再 4 条刷屏。
func (e *Engine) dispatch(b *notifyBatch) {
	content := batchContent(b)
	if content == "" {
		return
	}
	e.send(b.Rule, content)
}

func batchContent(b *notifyBatch) string {
	var s string
	if len(b.Fired) > 0 {
		s += "🚨 Hawkeye 告警\n📋 规则: " + b.Rule.Name + "\n"
		for _, it := range b.Fired {
			s += fmt.Sprintf("• %s %s\n", it.Target, it.Message)
		}
	}
	if len(b.Resolved) > 0 {
		if s != "" {
			s += "\n"
		}
		s += "✅ Hawkeye 已恢复\n📋 规则: " + b.Rule.Name + "\n"
		for _, it := range b.Resolved {
			s += fmt.Sprintf("• %s %s\n", it.Target, it.Message)
		}
	}
	if s == "" {
		return ""
	}
	return s + "⏰ " + time.Now().Format("2006-01-02 15:04:05")
}

// send 发送通知；失败落 outbox 队列，不再「只打日志然后丢弃」。
func (e *Engine) send(rule models.AlertRule, content string) {
	webhook := e.webhookFor(rule)
	if webhook == "" {
		log.Printf("[alert] no webhook configured, dropped: %s", firstLine(content))
		return
	}
	if err := e.notifier.SendText(webhook, content, nil); err != nil {
		log.Printf("[alert] notify failed (%v), queued to outbox", err)
		if qerr := e.db.EnqueueOutbox(rule.ID, content); qerr != nil {
			log.Printf("[alert] enqueue outbox failed: %v", qerr)
		}
	}
}

// flushOutbox 重试兜底队列：最多 3 次，指数退避，最终标记 failed 并在健康面板可见。
func (e *Engine) flushOutbox() {
	items, err := e.db.GetDueOutbox(20)
	if err != nil {
		return
	}
	for _, it := range items {
		rule, err := e.db.GetAlertRule(it.RuleID)
		if err != nil || rule == nil {
			_ = e.db.MarkOutboxFailed(it.ID, "rule gone")
			continue
		}
		webhook := e.webhookFor(*rule)
		if webhook == "" {
			_ = e.db.MarkOutboxFailed(it.ID, "no webhook")
			continue
		}
		if err := e.notifier.SendText(webhook, it.Payload, nil); err != nil {
			attempts := it.Attempts + 1
			if attempts >= 3 {
				_ = e.db.MarkOutboxFailed(it.ID, err.Error())
				log.Printf("[alert] outbox %d failed permanently: %v", it.ID, err)
				continue
			}
			backoff := time.Duration(attempts*30) * time.Second
			_ = e.db.MarkOutboxRetry(it.ID, attempts, time.Now().Add(backoff), err.Error())
			continue
		}
		_ = e.db.MarkOutboxSent(it.ID)
	}
}

func (e *Engine) webhookFor(rule models.AlertRule) string {
	if rule.WechatWebhook != "" {
		return rule.WechatWebhook
	}
	w, _ := e.db.GetSetting("default_wechat_webhook")
	return w
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

// TargetValue 供面板显示「这台当前值是多少」，与引擎评估用同一套取值逻辑。
func (e *Engine) TargetValue(rule models.AlertRule, t models.AlertRuleTarget) (float64, bool) {
	def, _ := MetricDefOf(rule.MetricType)
	return e.sample(rule, t, def)
}
