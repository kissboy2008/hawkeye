package alert

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hawkeye/internal/models"
)

// 回归测试：企业微信出错时仍返回 HTTP 200，错误码在 body 的 errcode 里。
// 旧实现只看状态码，key 失效会静默吞掉所有告警。
func TestSendTextDetectsWebhookErrCode(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":93000,"errmsg":"invalid webhook url, hint: [xxx]"}`))
	}))
	defer bad.Close()

	n := NewNotifier("")
	if err := n.SendText(bad.URL, "hi", nil); err == nil {
		t.Fatal("errcode != 0 must be an error")
	} else if !strings.Contains(err.Error(), "93000") {
		t.Fatalf("error should mention errcode, got: %v", err)
	}

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer ok.Close()

	if err := n.SendText(ok.URL, "hi", nil); err != nil {
		t.Fatalf("healthy webhook should succeed, got %v", err)
	}
}

// 恢复通知要带机器名：一条规则覆盖多台机器时才知道是谁恢复了。
func TestResolvedContentIncludesAgentName(t *testing.T) {
	rule := &models.AlertRule{Name: "机器离线"}
	agent := &models.Agent{Name: "fnOS"}

	got := resolvedContent(rule, agent)
	if !strings.Contains(got, "机器离线") || !strings.Contains(got, "fnOS") {
		t.Fatalf("resolved content should carry rule and agent name, got %q", got)
	}

	// probe 类告警没有 agent，不能输出空名字
	got = resolvedContent(rule, nil)
	if strings.Contains(got, "机器:") {
		t.Fatalf("nil agent must not render agent line, got %q", got)
	}
}
