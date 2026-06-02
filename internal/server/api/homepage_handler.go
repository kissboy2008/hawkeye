package api

import (
	"net/http"

	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
)

// getHomepageStats returns aggregate system statistics for Homepage customapi widget.
// This is a public endpoint (no session auth required) for use with Homepage dashboard.
// Response format matches Homepage customapi widget expectations:
//
//	{
//	  "online_agents": "3/5",
//	  "online_probes": "3/4",
//	  "active_alerts": "0"
//	}
//
// Homepage configuration example:
//
//	widget:
//	  type: customapi
//	  url: http://YOUR_SERVER:18325/api/v1/homepage/stats
//	  mappings:
//	    - field: online_agents
//	      label: 在线机器
//	    - field: online_probes
//	      label: 网站探测
//	    - field: active_alerts
//	      label: 活跃告警
func getHomepageStats(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := db.GetSystemStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}
