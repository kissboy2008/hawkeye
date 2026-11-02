package agent

import (
	"testing"
	"time"
)

// 验证 #22：重复调用 CollectMetrics 不再每次阻塞一个采样窗口。
func TestCollectMetricsNonBlockingAfterWarmup(t *testing.T) {
	c := NewCollector("test")
	c.SetSampleInterval(200 * time.Millisecond)

	if _, err := c.CollectMetrics(); err != nil {
		t.Fatalf("first CollectMetrics: %v", err)
	}
	time.Sleep(400 * time.Millisecond) // 等后台采样器产出样本

	start := time.Now()
	for i := 0; i < 5; i++ {
		m, err := c.CollectMetrics()
		if err != nil {
			t.Fatalf("CollectMetrics #%d: %v", i, err)
		}
		if m.CPU.Cores == 0 && m.CPU.UsagePercent == 0 && m.CPU.ModelName == "" {
			t.Logf("warning: cpu metrics empty on call %d", i)
		}
	}
	if d := time.Since(start); d > 150*time.Millisecond {
		t.Fatalf("5 calls took %v, expected them to be non-blocking", d)
	}
}

// 验证 #22：sampleInterval 可配置且非法值被忽略。
func TestSetSampleInterval(t *testing.T) {
	c := NewCollector("test")
	c.SetSampleInterval(0) // 非法，应被忽略
	if got := c.getSampleInterval(); got != time.Second {
		t.Fatalf("zero interval should be ignored, got %v", got)
	}
	c.SetSampleInterval(250 * time.Millisecond)
	if got := c.getSampleInterval(); got != 250*time.Millisecond {
		t.Fatalf("got %v", got)
	}
}
