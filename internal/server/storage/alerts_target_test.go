package storage

import (
	"path/filepath"
	"testing"

	"hawkeye/internal/models"
)

// 回归测试：同一条规则作用在多台 agent 上时，resolved 与「最新事件」必须按目标区分。
// 修复前 ResolveAlertByRule 会把该规则所有 agent 的未解决事件一次性 resolved。
func TestAlertTargetScoped(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	const ruleID int64 = 1
	agent1, agent2 := int64(11), int64(22)
	probe1 := int64(33)

	ev1, err := db.CreateAlertEvent(&models.AlertEvent{RuleID: ruleID, AgentID: &agent1, Message: "cpu agent1"})
	if err != nil {
		t.Fatalf("create event 1: %v", err)
	}
	if _, err := db.CreateAlertEvent(&models.AlertEvent{RuleID: ruleID, AgentID: &agent2, Message: "cpu agent2"}); err != nil {
		t.Fatalf("create event 2: %v", err)
	}
	if _, err := db.CreateAlertEvent(&models.AlertEvent{RuleID: ruleID, ProbeID: &probe1, Message: "probe1 down"}); err != nil {
		t.Fatalf("create event 3: %v", err)
	}

	// 只解决 agent1 的事件
	if err := db.ResolveAlertByTarget(ruleID, &agent1, nil); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	open, err := db.GetUnresolvedAlertEvents()
	if err != nil {
		t.Fatalf("list unresolved: %v", err)
	}
	if len(open) != 2 {
		t.Fatalf("want 2 unresolved events, got %d", len(open))
	}
	for _, ev := range open {
		if ev.AgentID != nil && *ev.AgentID == agent1 {
			t.Fatalf("agent1 event %d should be resolved but is still open", ev.ID)
		}
	}

	// 已解决的那条不能再出现在 agent1 的「最新事件」里
	latest, err := db.GetLatestAlertEventByTarget(ruleID, &agent1, nil)
	if err != nil {
		t.Fatalf("latest by target: %v", err)
	}
	if latest == nil {
		t.Fatalf("agent1 should still have its event history")
	}
	if latest.ID != ev1 {
		t.Fatalf("latest for agent1 = %d, want %d", latest.ID, ev1)
	}
	if latest.ResolvedAt == nil {
		t.Fatalf("agent1 latest event should be resolved")
	}

	// probe 目标互不干扰
	probeLatest, err := db.GetLatestAlertEventByTarget(ruleID, nil, &probe1)
	if err != nil {
		t.Fatalf("latest by probe: %v", err)
	}
	if probeLatest == nil || probeLatest.ProbeID == nil || *probeLatest.ProbeID != probe1 {
		t.Fatalf("probe1 latest event not found")
	}
	if probeLatest.ResolvedAt != nil {
		t.Fatalf("probe1 event should still be open")
	}
}
