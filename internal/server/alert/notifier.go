package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"hawkeye/internal/models"
)

// Notifier sends alert notifications via WeChat Work webhook.
type Notifier struct {
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
		webhook = n.globalWebhook
	}
	if webhook == "" {
		log.Println("[alert] no webhook URL configured, skipping notification")
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
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return nil
		}
		lastErr = fmt.Errorf("webhook returned status %d (attempt %d)", resp.StatusCode, attempt+1)
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
	lines = append(lines, fmt.Sprintf("Hawkeye 监控告警 %s", severity))

	if agent != nil {
		lines = append(lines, fmt.Sprintf("机器: %s (%s)", agent.Name, agent.Address))
	}

	lines = append(lines, fmt.Sprintf("规则: %s", rule.Name))

	if event.Value != nil {
		lines = append(lines, fmt.Sprintf("当前值: %.1f (阈值: %.1f)", *event.Value, rule.Threshold))
	}

	if event.Message != "" {
		lines = append(lines, fmt.Sprintf("详情: %s", event.Message))
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
	} else {
		log.Printf("[alert] notification sent for rule: %s", rule.Name)
	}
}

// SendResolved sends a resolved alert notification.
func (n *Notifier) SendResolved(webhook string, event *models.AlertEvent, rule *models.AlertRule) {
	content := fmt.Sprintf(
		"Hawkeye 告警恢复 [恢复]\n规则: %s",
		rule.Name,
	)

	if err := n.SendText(webhook, content, nil); err != nil {
		log.Printf("[alert] failed to send resolved notification: %v", err)
	}
}

// SendTest sends a test notification to verify the webhook is working.
func (n *Notifier) SendTest(webhook string, ruleName string) error {
	content := fmt.Sprintf(
		"Hawkeye 通知测试 [测试]\n规则: %s\n\n这是一条测试消息，通知通道工作正常。",
		ruleName,
	)
	return n.SendText(webhook, content, nil)
}

// UpdateGlobalWebhook updates the global webhook URL at runtime.
func (n *Notifier) UpdateGlobalWebhook(webhook string) {
	n.globalWebhook = webhook
}
