package api

import (
	"net/http"
	"strconv"

	"hawkeye/internal/models"
	"hawkeye/internal/server/alert"
	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
)

func listAlertRules(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rules, err := db.GetAllAlertRules()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if rules == nil {
			rules = []models.AlertRule{}
		}
		c.JSON(http.StatusOK, rules)
	}
}

func createAlertRule(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rule models.AlertRule
		if err := c.ShouldBindJSON(&rule); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if rule.Operator == "" {
			rule.Operator = "gt"
		}
		if rule.CooldownS == 0 {
			rule.CooldownS = 300
		}
		rule.Enabled = true
		id, err := db.CreateAlertRule(&rule)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		rule.ID = id
		c.JSON(http.StatusCreated, rule)
	}
}

func getAlertRule(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		rule, err := db.GetAlertRule(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if rule == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
			return
		}
		c.JSON(http.StatusOK, rule)
	}
}

func updateAlertRule(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var rule models.AlertRule
		if err := c.ShouldBindJSON(&rule); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		rule.ID = id
		if err := db.UpdateAlertRule(&rule); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, rule)
	}
}

func deleteAlertRule(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := db.DeleteAlertRule(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}

func listAlertEvents(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		resolved := c.Query("resolved") != "false" // default: show all
		limit, _ := strconv.Atoi(c.Query("limit"))
		if limit == 0 {
			limit = 50
		}

		events, err := db.GetAlertEvents(resolved, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if events == nil {
			events = []models.AlertEvent{}
		}
		c.JSON(http.StatusOK, events)
	}
}

func deleteAllAlertEvents(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := db.DeleteAllAlertEvents()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted", "count": count})
	}
}

func testAlertRule(db *storage.DB, notifier *alert.Notifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		rule, err := db.GetAlertRule(id)
		if rule == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		webhook := rule.WechatWebhook
		if webhook == "" {
			webhook, _ = db.GetSetting("default_wechat_webhook")
		}

		if webhook == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no webhook configured"})
			return
		}

		if err := notifier.SendTest(webhook, rule.Name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "test notification sent"})
	}
}

// testWebhook sends a test notification to an arbitrary webhook URL.
func testWebhook(notifier *alert.Notifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Webhook string `json:"webhook" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "webhook URL is required"})
			return
		}

		if err := notifier.SendTest(req.Webhook, "通用测试"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "test notification sent"})
	}
}

func getSettings(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := db.GetAllSettings()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, settings)
	}
}

func updateSettings(db *storage.DB, notifier *alert.Notifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		var settings map[string]string
		if err := c.ShouldBindJSON(&settings); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		for key, value := range settings {
			if err := db.SetSetting(key, value); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// Update global webhook at runtime
			if key == "default_wechat_webhook" && notifier != nil {
				notifier.UpdateGlobalWebhook(value)
			}
		}
		c.JSON(http.StatusOK, gin.H{"message": "updated"})
	}
}

// getDatabaseInfo returns the database file size.
func getDatabaseInfo(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		size, err := db.DatabaseSize()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"size_bytes": size})
	}
}

// purgeDatabase deletes all monitoring data (metrics, probe results, alert events)
// while keeping agents, probes, rules, settings, and users.
func purgeDatabase(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := db.PurgeAllData()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "purged", "deleted_rows": count})
	}
}
