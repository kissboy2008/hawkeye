package api

import (
	"net/http"

	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
)

// getHawkeyeStats returns aggregate system statistics for Hawkeye dashboard widget.
// This is a public endpoint (no session auth required).
//
//	{
//	  "online_agents": "3/5",
//	  "online_probes": "3/4",
//	  "active_alerts": "0"
//	}
func getHawkeyeStats(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := db.GetSystemStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}
