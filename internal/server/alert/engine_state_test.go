package alert

import "testing"

// 回归测试：同一条规则对不同 agent / probe 必须是不同的状态条目，
// 否则第一台机器触发后其余机器永远不会告警。
func TestStateKeyDistinguishesTargets(t *testing.T) {
	a1, a2 := int64(1), int64(2)
	p1 := int64(3)

	keys := map[string]bool{
		stateKey(7, &a1, nil): true,
		stateKey(7, &a2, nil): true,
		stateKey(7, nil, &p1): true,
		stateKey(8, &a1, nil): true,
		stateKey(7, nil, nil): true,
	}
	if len(keys) != 5 {
		t.Fatalf("want 5 distinct keys, got %d: %v", len(keys), keys)
	}

	if stateKey(7, &a1, nil) != stateKey(7, &a1, nil) {
		t.Fatalf("stateKey is not deterministic")
	}
	if stateKey(7, &a1, nil) == stateKey(7, &a2, nil) {
		t.Fatalf("different agents must not share state")
	}
}
