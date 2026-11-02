package alert

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"hawkeye/internal/models"
	"hawkeye/internal/server/storage"
)

// ---------- 测试脚手架 ----------

type harness struct {
	db       *storage.DB
	engine   *Engine
	mu       sync.Mutex
	messages []string
	agentSeq int
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	h := &harness{db: db}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		h.mu.Lock()
		h.messages = append(h.messages, string(buf[:n]))
		h.mu.Unlock()
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	t.Cleanup(srv.Close)
	if _, err := db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('default_wechat_webhook', ?)`, srv.URL); err != nil {
		t.Fatalf("set webhook: %v", err)
	}

	h.engine = NewEngine(db, NewNotifier(srv.URL), 60)
	return h
}

func (h *harness) sent() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.messages))
	copy(out, h.messages)
	return out
}

// makeAgent 建一台机器，lastSeenAgo 为「多少分钟前有心跳」。
func (h *harness) makeAgent(t *testing.T, name string, lastSeenAgo time.Duration) int64 {
	t.Helper()
	h.mu.Lock()
	h.agentSeq++
	seq := h.agentSeq
	h.mu.Unlock()
	addr := fmt.Sprintf("10.0.0.%d", 10+seq)
	id, err := h.db.CreateAgent(&models.Agent{
		Name: name, Address: addr, Mode: "pull", ServerURL: "http://" + addr + ":32518",
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if _, err := h.db.Exec(`UPDATE agents SET last_seen=? WHERE id=?`,
		time.Now().UTC().Add(-lastSeenAgo).Truncate(time.Second), id); err != nil {
		t.Fatalf("set last_seen: %v", err)
	}
	return id
}

func (h *harness) makeAvailabilityRule(t *testing.T, name string, thresholdMin float64, cooldownS int) int64 {
	t.Helper()
	rule := models.AlertRule{
		Name: name, MetricType: MetricAgentOffline, Operator: "gte", Threshold: thresholdMin,
		Enabled: true, CooldownS: cooldownS, TargetClass: ClassAgent, Kind: KindAvailability,
	}
	id, err := h.db.CreateAlertRule(&rule)
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	return id
}

func (h *harness) addTarget(t *testing.T, ruleID int64, class string, targetID int64, threshold *float64, forS *int) {
	t.Helper()
	m := models.AlertRuleTarget{
		RuleID: ruleID, TargetType: class, TargetID: targetID, Enabled: true,
		Threshold: threshold, ForS: forS,
	}
	if _, err := h.db.UpsertAlertRuleTarget(&m); err != nil {
		t.Fatalf("add target: %v", err)
	}
}

func (h *harness) unresolved(t *testing.T) []models.AlertEvent {
	t.Helper()
	evs, err := h.db.GetUnresolvedAlertEvents()
	if err != nil {
		t.Fatalf("unresolved: %v", err)
	}
	return evs
}

// ---------- 用例 ----------

// 离线告警的值必须来自 last_seen；阈值 5 分钟时，离线 6 分钟才触发。
func TestAvailabilityUsesLastSeen(t *testing.T) {
	h := newHarness(t)
	a := h.makeAgent(t, "test-host", 6*time.Minute)

	low := h.makeAvailabilityRule(t, "offline-10", 10, 0)
	h.addTarget(t, low, ClassAgent, a, nil, nil)
	h.engine.evaluateAll()
	if got := h.unresolved(t); len(got) != 0 {
		t.Fatalf("threshold 10min should not fire, got %d", len(got))
	}

	high := h.makeAvailabilityRule(t, "offline-5", 5, 0)
	h.addTarget(t, high, ClassAgent, a, nil, nil)
	h.engine.evaluateAll()

	got := h.unresolved(t)
	if len(got) != 1 {
		t.Fatalf("want 1 event, got %d", len(got))
	}
	if got[0].AgentID == nil || *got[0].AgentID != a {
		t.Fatalf("event not bound to agent %d", a)
	}
	if !strings.Contains(got[0].Message, "离线") {
		t.Fatalf("message should describe offline duration, got %q", got[0].Message)
	}

	// 重复评估不重复产生事件
	h.engine.evaluateAll()
	if got := h.unresolved(t); len(got) != 1 {
		t.Fatalf("re-evaluate should not duplicate, got %d", len(got))
	}

	// 心跳恢复 → 自动 resolve，且发出恢复通知
	if _, err := h.db.Exec(`UPDATE agents SET last_seen=? WHERE id=?`, time.Now().UTC(), a); err != nil {
		t.Fatalf("touch last_seen: %v", err)
	}
	h.engine.evaluateAll()
	if got := h.unresolved(t); len(got) != 0 {
		t.Fatalf("should resolve after heartbeat, got %d unresolved", len(got))
	}
	if msgs := h.sent(); len(msgs) == 0 || !strings.Contains(msgs[len(msgs)-1], "已恢复") {
		t.Fatalf("resolved notification missing: %v", h.sent())
	}
}

// 逐台独立：同一规则下 A 设 3 分钟、B 设 10 分钟，离线 5 分钟时只有 A 告警。
func TestPerTargetThresholdsAreIndependent(t *testing.T) {
	h := newHarness(t)
	a := h.makeAgent(t, "fnOS", 5*time.Minute)
	b := h.makeAgent(t, "aliyun", 5*time.Minute)

	ruleID := h.makeAvailabilityRule(t, "offline", 5, 0)
	three, ten := 3.0, 10.0
	h.addTarget(t, ruleID, ClassAgent, a, &three, nil)
	h.addTarget(t, ruleID, ClassAgent, b, &ten, nil)

	h.engine.evaluateAll()

	evs := h.unresolved(t)
	if len(evs) != 1 {
		t.Fatalf("want exactly 1 event (fnOS only), got %d", len(evs))
	}
	if evs[0].AgentID == nil || *evs[0].AgentID != a {
		t.Fatalf("the fired target should be fnOS (id %d), got %v", a, evs[0].AgentID)
	}
}

// 重启不丢：状态里 first_true_at 已经是 20 分钟前，新引擎（模拟重启）必须立刻判定成立。
func TestStateSurvivesRestart(t *testing.T) {
	h := newHarness(t)
	a := h.makeAgent(t, "pve", 1*time.Minute) // 当前是「在线」
	ruleID := h.makeAvailabilityRule(t, "offline", 5, 0)
	forS := 300
	h.addTarget(t, ruleID, ClassAgent, a, nil, &forS)

	// 上次评估时条件已成立（离线 6 分钟），first_true_at 记在 20 分钟前
	old := time.Now().UTC().Add(-20 * time.Minute)
	st := &models.AlertRuleState{
		RuleID: ruleID, TargetType: ClassAgent, TargetID: a,
		Status: "pending", FirstTrueAt: &old, LastValue: 6,
	}
	if err := h.db.SaveAlertRuleState(st); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// 机器现在确实离线 6 分钟
	if _, err := h.db.Exec(`UPDATE agents SET last_seen=? WHERE id=?`,
		time.Now().UTC().Add(-6*time.Minute), a); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	// 换一个新引擎 = 服务重启（内存状态清零）
	fresh := NewEngine(h.db, NewNotifier(""), 60)
	fresh.evaluateAll()

	if evs := h.unresolved(t); len(evs) != 1 {
		t.Fatalf("after restart the alert must still fire immediately, got %d events", len(evs))
	}
}

// 持续语义：条件曾成立但中途断过，first_true_at 必须清零重新计时。
func TestForSResetsWhenConditionBreaks(t *testing.T) {
	h := newHarness(t)
	a := h.makeAgent(t, "nas", 1*time.Minute) // 在线 → 条件不成立
	ruleID := h.makeAvailabilityRule(t, "offline", 5, 0)
	forS := 300
	h.addTarget(t, ruleID, ClassAgent, a, nil, &forS)

	// 先让条件成立且已持续 200 秒
	if _, err := h.db.Exec(`UPDATE agents SET last_seen=? WHERE id=?`,
		time.Now().UTC().Add(-6*time.Minute), a); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	state, _ := h.db.GetAlertRuleState(ruleID, ClassAgent, a)
	if state == nil {
		state = &models.AlertRuleState{RuleID: ruleID, TargetType: ClassAgent, TargetID: a, Status: "pending"}
	}
	started := time.Now().UTC().Add(-200 * time.Second)
	state.FirstTrueAt = &started
	if err := h.db.SaveAlertRuleState(state); err != nil {
		t.Fatalf("save: %v", err)
	}

	// 中间心跳恢复一次（条件断开）→ 必须清零
	if _, err := h.db.Exec(`UPDATE agents SET last_seen=? WHERE id=?`, time.Now().UTC(), a); err != nil {
		t.Fatalf("touch: %v", err)
	}
	h.engine.evaluateAll()
	st, err := h.db.GetAlertRuleState(ruleID, ClassAgent, a)
	if err != nil || st == nil {
		t.Fatalf("state missing: %v", err)
	}
	if st.FirstTrueAt != nil {
		t.Fatalf("first_true_at must be cleared when condition breaks")
	}

	// 再离线一次，持续时长必须从头算 → 不触发
	if _, err := h.db.Exec(`UPDATE agents SET last_seen=? WHERE id=?`,
		time.Now().UTC().Add(-6*time.Minute), a); err != nil {
		t.Fatalf("backdate2: %v", err)
	}
	h.engine.evaluateAll()
	if evs := h.unresolved(t); len(evs) != 0 {
		t.Fatalf("for_s=300 must not fire immediately after restart of the condition, got %d", len(evs))
	}
}

// 多台同时掉线 → 一条消息里列出全部，不再刷屏。
func TestNotificationsAreBatchedPerRule(t *testing.T) {
	h := newHarness(t)
	a := h.makeAgent(t, "fnOS", 6*time.Minute)
	b := h.makeAgent(t, "UNRAID", 6*time.Minute)
	c := h.makeAgent(t, "PVE", 6*time.Minute)

	ruleID := h.makeAvailabilityRule(t, "机器离线", 5, 0)
	for _, id := range []int64{a, b, c} {
		h.addTarget(t, ruleID, ClassAgent, id, nil, nil)
	}
	h.engine.evaluateAll()

	msgs := h.sent()
	if len(msgs) != 1 {
		t.Fatalf("3 targets should produce 1 merged message, got %d: %v", len(msgs), msgs)
	}
	for _, name := range []string{"fnOS", "UNRAID", "PVE"} {
		if !strings.Contains(msgs[0], name) {
			t.Fatalf("merged message should mention %s, got %s", name, msgs[0])
		}
	}
}

// 静默：只压通知，状态仍然 firing（并记 suppressed 次数）。
func TestSilenceSuppressesNotificationButKeepsState(t *testing.T) {
	h := newHarness(t)
	a := h.makeAgent(t, "quiet-host", 6*time.Minute)
	ruleID := h.makeAvailabilityRule(t, "offline", 5, 0)
	until := time.Now().Add(time.Hour)
	h.addTarget(t, ruleID, ClassAgent, a, nil, nil)
	if err := h.db.SaveAlertRuleState(&models.AlertRuleState{}); err == nil {
		_ = err
	}
	targets, _ := h.db.GetAlertRuleTargets(ruleID)
	targets[0].SilencedUntil = &until
	if _, err := h.db.UpsertAlertRuleTarget(&targets[0]); err != nil {
		t.Fatalf("silence: %v", err)
	}

	h.engine.evaluateAll()

	if msgs := h.sent(); len(msgs) != 0 {
		t.Fatalf("silenced alert must not notify, got %v", msgs)
	}
	st, _ := h.db.GetAlertRuleState(ruleID, ClassAgent, a)
	if st == nil || st.Status != "silenced" {
		t.Fatalf("state should be silenced, got %+v", st)
	}
	if st.SuppressedCount == 0 {
		t.Fatalf("suppressed_count should be incremented")
	}
	if evs := h.unresolved(t); len(evs) != 0 {
		t.Fatalf("silenced alert should not create events, got %d", len(evs))
	}
}

// 冷却只防抖：恢复后 60 秒内不重报，超过就立刻重报（不再 10 分钟黑洞）。
func TestCooldownOnlyAntiFlap(t *testing.T) {
	h := newHarness(t)
	a := h.makeAgent(t, "flappy", 6*time.Minute)
	ruleID := h.makeAvailabilityRule(t, "offline", 5, 60)
	h.addTarget(t, ruleID, ClassAgent, a, nil, nil)

	h.engine.evaluateAll()
	evs := h.unresolved(t)
	if len(evs) != 1 {
		t.Fatalf("first fire failed: %d", len(evs))
	}

	// 手动把事件 resolve 在 30 秒前 → 仍在冷却
	if _, err := h.db.Exec(`UPDATE alert_events SET resolved_at=? WHERE id=?`,
		time.Now().UTC().Add(-30*time.Second), evs[0].ID); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	st, _ := h.db.GetAlertRuleState(ruleID, ClassAgent, a)
	st.Status = "ok"
	st.FiredAt = nil
	_ = h.db.SaveAlertRuleState(st)

	h.engine.evaluateAll()
	if got := h.unresolved(t); len(got) != 0 {
		t.Fatalf("within cooldown must not re-fire, unresolved=%d", len(got))
	}

	// 事件 resolve 在 90 秒前 → 冷却已过，必须重报
	if _, err := h.db.Exec(`UPDATE alert_events SET resolved_at=? WHERE id=?`,
		time.Now().UTC().Add(-90*time.Second), evs[0].ID); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	h.engine.evaluateAll()
	if got := h.unresolved(t); len(got) != 1 {
		t.Fatalf("after cooldown must re-fire, unresolved=%d", len(got))
	}
}

// 通知失败 → 进 outbox，进程重启后仍在（不再静默丢弃）。
func TestNotifyFailureGoesToOutbox(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	defer db.Close()

	// 打不通的 webhook
	if _, err := db.Exec(`INSERT OR REPLACE INTO settings (key, value) VALUES ('default_wechat_webhook', 'http://127.0.0.1:1/nope')`); err != nil {
		t.Fatalf("setting: %v", err)
	}
	id, err := db.CreateAgent(&models.Agent{Name: "x", Address: "10.0.0.1", Mode: "pull", ServerURL: "http://x"})
	if err != nil {
		t.Fatalf("agent: %v", err)
	}
	if _, err := db.Exec(`UPDATE agents SET last_seen=? WHERE id=?`, time.Now().UTC().Add(-30*time.Minute), id); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	rule := models.AlertRule{Name: "offline", MetricType: MetricAgentOffline, Operator: "gte",
		Threshold: 5, Enabled: true, TargetClass: ClassAgent, Kind: KindAvailability}
	ruleID, err := db.CreateAlertRule(&rule)
	if err != nil {
		t.Fatalf("rule: %v", err)
	}
	if _, err := db.UpsertAlertRuleTarget(&models.AlertRuleTarget{
		RuleID: ruleID, TargetType: ClassAgent, TargetID: id, Enabled: true,
	}); err != nil {
		t.Fatalf("target: %v", err)
	}

	eng := NewEngine(db, NewNotifier("http://127.0.0.1:1/nope"), 60)
	eng.evaluateAll()

	due, err := db.GetDueOutbox(10)
	if err != nil {
		t.Fatalf("outbox: %v", err)
	}
	if len(due) == 0 {
		t.Fatal("failed notification must be queued in outbox, not dropped")
	}
	if !strings.Contains(due[0].Payload, "离线") {
		t.Fatalf("outbox payload lost, got %q", due[0].Payload)
	}
}
