package alert

import (
	"path/filepath"
	"testing"
	"time"

	"hawkeye/internal/models"
	"hawkeye/internal/server/storage"
)

// 回归测试：离线规则的价值必须来自 last_seen，而不是 metrics 表。
// 场景：一台机器 6 分钟没上报，阈值 5 分钟 → 必须触发；
// 阈值 10 分钟 → 不触发；重新上报后 → 恢复。
func TestAgentOfflineRuleUsesLastSeen(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	agentID, err := db.CreateAgent(&models.Agent{
		Name:      "test-host",
		Address:   "10.0.0.9",
		Mode:      "pull",
		ServerURL: "http://10.0.0.9:32518",
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// 模拟 6 分钟没上报
	sixMinAgo := time.Now().Add(-6 * time.Minute).UTC().Truncate(time.Second)
	if _, err := db.Exec(`UPDATE agents SET last_seen=? WHERE id=?`, sixMinAgo, agentID); err != nil {
		t.Fatalf("set last_seen: %v", err)
	}

	notifier := NewNotifier("")
	eng := NewEngine(db, notifier, 60)

	mkRule := func(ruleID int64, threshold float64) models.AlertRule {
		id := agentID
		return models.AlertRule{
			ID:         ruleID,
			Name:       "offline-test",
			ScopeType:  "agent",
			ScopeID:    &id,
			MetricType: MetricAgentOffline,
			Operator:   "gte",
			Threshold:  threshold,
			Enabled:    true,
		}
	}

	// 阈值 10 分钟：6 分钟没上报不该告警
	r10 := mkRule(1, 10)
	eng.evaluateAgentRule(r10)
	if evs, _ := db.GetUnresolvedAlertEvents(); len(evs) != 0 {
		t.Fatalf("threshold 10min should not fire, got %d events", len(evs))
	}

	// 阈值 5 分钟：应当告警
	r5 := mkRule(2, 5)
	eng.evaluateAgentRule(r5)
	evs, err := db.GetUnresolvedAlertEvents()
	if err != nil {
		t.Fatalf("get events: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("want 1 alert event, got %d", len(evs))
	}
	if evs[0].AgentID == nil || *evs[0].AgentID != agentID {
		t.Fatalf("event not bound to agent %d: %+v", agentID, evs[0])
	}
	if evs[0].Message == "" {
		t.Fatal("event message should describe offline duration")
	}

	// 重复评估不会重复产生事件
	eng.evaluateAgentRule(r5)
	if evs, _ := db.GetUnresolvedAlertEvents(); len(evs) != 1 {
		t.Fatalf("re-evaluate should not duplicate events, got %d", len(evs))
	}

	// 心跳恢复 → 事件被 resolve
	if _, err := db.Exec(`UPDATE agents SET last_seen=? WHERE id=?`, time.Now().UTC(), agentID); err != nil {
		t.Fatalf("touch last_seen: %v", err)
	}
	eng.evaluateAgentRule(r5)
	if evs, _ := db.GetUnresolvedAlertEvents(); len(evs) != 0 {
		t.Fatalf("alert should be resolved after heartbeat, got %d unresolved", len(evs))
	}
}
