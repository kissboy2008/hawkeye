package api

import (
	"net/http"
	"strconv"
	"time"

	"hawkeye/internal/models"
	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
)

func getLatestMetrics(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		result := make(map[string]string)
		types := []string{"cpu", "memory", "uptime"}
		for _, t := range types {
			data, _ := db.GetLatestMetric(id, t)
			if data != "" {
				result[t] = data
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"agent_id":  id,
			"timestamp": time.Now().UTC(),
			"metrics":   result,
		})
	}
}

func getMetricsTimeRange(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		metricType := c.Param("type")

		// Parse time parameters
		from, to := parseTimeRange(c.Query("from"), c.Query("to"))

		// For hourly data if range > 1 day
		if to.Sub(from) > 24*time.Hour {
			hourlyPoints, err := db.GetHourlyMetrics(id, metricType, from, to)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, models.MetricsTimeSeries{
				MetricType: metricType,
				DataPoints: hourlyPoints,
			})
			return
		}

		points, err := db.GetMetricsTimeRange(id, metricType, from, to)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, models.MetricsTimeSeries{
			MetricType: metricType,
			DataPoints: points,
		})
	}
}

func parseTimeRange(fromStr, toStr string) (time.Time, time.Time) {
	now := time.Now().UTC()

	to := now
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}

	from := to.Add(-1 * time.Hour) // default 1 hour
	if fromStr != "" {
		// Support relative durations like "1h", "24h", "7d"
		if duration, err := parseDuration(fromStr); err == nil {
			from = to.Add(-duration)
		} else if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		}
	}

	return from, to
}

func parseDuration(s string) (time.Duration, error) {
	// Simple parser for "1h", "24h", "168h", "7d", "30m"
	if len(s) > 1 && s[len(s)-1] == 'd' {
		// Convert "7d" -> "168h"
		if days, err := time.ParseDuration(s[:len(s)-1] + "h"); err == nil {
			return days * 24, nil
		}
	}
	return time.ParseDuration(s)
}

