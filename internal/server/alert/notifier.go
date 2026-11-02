package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"hawkeye/internal/models"
)

// Notifier sends alert notifications via WeChat Work webhook.
type Notifier struct {
	mu            sync.RWMutex
	globalWebhook string
	httpClient    *http.Client
}

func NewNotifier(globalWebhook string) *Notifier {
	return &Notifier{
		globalWebhook: globalWebhook,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}
}

// SendText sends a WeChat Work text notification with retry (3 attempts, exponential backoff).
func (n *Notifier) SendText(webhook, content string, mentionedList []string) error {
	if webhook == "" {
		n.mu.RLock()
		webhook = n.globalWebhook
		n.mu.RUnlock()
	}
	if webhook == "" {
		return nil
	}

	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content":        content,
			"mentioned_list": mentionedList,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	// Retry up to 3 times with exponential backoff (1s, 2s, 4s)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			time.Sleep(backoff)
		}

		resp, err := n.httpClient.Post(webhook, "application/json", bytes.NewReader(body))
		if err != nil {
			lastErr = fmt.Errorf("http post (attempt %d): %w", attempt+1, err)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("webhook returned status %d (attempt %d)", resp.StatusCode, attempt+1)
			continue
		}
		if readErr != nil {
			lastErr = fmt.Errorf("read webhook response (attempt %d): %w", attempt+1, readErr)
			continue
		}
		// 企业微信/钉钉这类网关出错时仍返回 200，错误码在 body 里（如 93000 = key 无效），
		// 只看状态码会把「通知全丢」当成成功，所以必须解析 errcode。
		var whResp struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		if len(body) > 0 && json.Unmarshal(body, &whResp) == nil && whResp.ErrCode != 0 {
			return fmt.Errorf("webhook errcode %d: %s", whResp.ErrCode, whResp.ErrMsg)
		}
		return nil
	}

	return lastErr
}

// SendAlert sends a formatted alert notification.
func (n *Notifier) SendAlert(webhook string, event *models.AlertEvent, rule *models.AlertRule, agent *models.Agent) {
	severity := "[警告]"
	if event.Severity == "critical" {
		severity = "[严重]"
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("🔔 Hawkeye 监控告警 %s", severity))

	if agent != nil {
		lines = append(lines, fmt.Sprintf("📍 机器: %s", agent.Name))
	}

	lines = append(lines, fmt.Sprintf("📋 规则: %s", rule.Name))

	if event.Value != nil {
		lines = append(lines, fmt.Sprintf("📊 当前值: %.1f（阈值: %.1f）", *event.Value, rule.Threshold))
	}

	if event.Message != "" {
		lines = append(lines, fmt.Sprintf("💬 %s", event.Message))
	}

	content := ""
	for _, l := range lines {
		if content != "" {
			content += "\n"
		}
		content += l
	}

	if err := n.SendText(webhook, content, nil); err != nil {
		log.Printf("[alert] failed to send notification: %v", err)
	}
}

// resolvedContent 构造恢复通知文本。多机器共用一条规则时，必须带上机器名，
// 否则「已恢复」到底是谁恢复了完全看不出来。
func resolvedContent(rule *models.AlertRule, agent *models.Agent) string {
	content := fmt.Sprintf("✅ Hawkeye 告警已恢复\n📋 规则: %s", rule.Name)
	if agent != nil {
		content += fmt.Sprintf("\n📍 机器: %s", agent.Name)
	}
	return content
}

// SendResolved sends a resolved alert notification.
func (n *Notifier) SendResolved(webhook string, event *models.AlertEvent, rule *models.AlertRule, agent *models.Agent) {
	content := resolvedContent(rule, agent)

	if err := n.SendText(webhook, content, nil); err != nil {
		log.Printf("[alert] failed to send resolved notification: %v", err)
	}
}

// SendTest sends a test notification to verify the webhook is working.
func (n *Notifier) SendTest(webhook string, ruleName string) error {
	content := fmt.Sprintf(
		"🧪 Hawkeye 通知测试\n📋 规则: %s\n\n这是一条测试消息，通知通道正常工作。",
		ruleName,
	)
	return n.SendText(webhook, content, nil)
}

// UpdateGlobalWebhook updates the global webhook URL at runtime.
func (n *Notifier) UpdateGlobalWebhook(webhook string) {
	n.mu.Lock()
	n.globalWebhook = webhook
	n.mu.Unlock()
}
