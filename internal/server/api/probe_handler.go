package api

import (
	"net/http"
	"strconv"
	"time"

	"hawkeye/internal/models"
	probepkg "hawkeye/internal/server/probe"
	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
)

func listProbes(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		probes, err := db.GetAllProbes()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if probes == nil {
			probes = []models.WebProbe{}
		}
		c.JSON(http.StatusOK, probes)
	}
}

func createProbe(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var probe models.WebProbe
		if err := c.ShouldBindJSON(&probe); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if probe.Method == "" {
			probe.Method = "GET"
		}
		if probe.ExpectedStatus == 0 {
			probe.ExpectedStatus = 200
		}
		if probe.TimeoutMs == 0 {
			probe.TimeoutMs = 5000
		}
		if probe.IntervalS == 0 {
			probe.IntervalS = 60
		}
		probe.Enabled = true
		id, err := db.CreateProbe(&probe)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		probe.ID = id
		c.JSON(http.StatusCreated, probe)
	}
}

func getProbe(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		probe, err := db.GetProbe(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if probe == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "probe not found"})
			return
		}
		c.JSON(http.StatusOK, probe)
	}
}

func updateProbe(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var probe models.WebProbe
		if err := c.ShouldBindJSON(&probe); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		probe.ID = id
		if err := db.UpdateProbe(&probe); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, probe)
	}
}

func deleteProbe(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := db.DeleteProbe(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}

func checkProbe(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		probe, err := db.GetProbe(id)
		if probe == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "probe not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Execute probe check
		result := probepkg.ExecuteProbe(probe)
		db.InsertProbeResult(result)

		c.JSON(http.StatusOK, result)
	}
}

func getProbeResults(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var from, to time.Time
		if fromStr := c.Query("from"); fromStr != "" {
			from, _ = time.Parse(time.RFC3339, fromStr)
		}
		if toStr := c.Query("to"); toStr != "" {
			to, _ = time.Parse(time.RFC3339, toStr)
		}

		limit, _ := strconv.Atoi(c.Query("limit"))
		if limit == 0 {
			limit = 100
		}

		results, err := db.GetProbeResults(id, from, to, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if results == nil {
			results = []models.ProbeResult{}
		}
		c.JSON(http.StatusOK, results)
	}
}

// getProbeStats returns aggregated statistics for a probe.
func getProbeStats(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		stats, err := db.GetProbeStats(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, stats)
	}
}

// getProbeCertInfo returns the latest SSL certificate info for a probe.
func getProbeCertInfo(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		cert, err := db.GetLatestCertInfo(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if cert == nil {
			c.JSON(http.StatusOK, nil)
			return
		}
		c.JSON(http.StatusOK, cert)
	}
}

// getUptimeBars returns aggregated uptime bars for a probe.
// Query params: range=1h|1d|7d
func getUptimeBars(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		rng := c.DefaultQuery("range", "1h")
		var hours, bucketSec int
		switch rng {
		case "1d":
			hours, bucketSec = 24, 2880 // 24h / 30 bars
		case "7d":
			hours, bucketSec = 168, 20160 // 7d / 30 bars
		default:
			hours, bucketSec = 1, 120 // 1h / 30 bars
		}

		bars, err := db.GetUptimeBars(id, hours, bucketSec)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, bars)
	}
}

// getResponseTimeTrend returns averaged response time data for the chart.
// Query params: range=1h|1d|7d
func getResponseTimeTrend(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		rng := c.DefaultQuery("range", "1h")
		var hours, bucketSec int
		switch rng {
		case "1d":
			hours, bucketSec = 24, 1200 // 20-min buckets
		case "7d":
			hours, bucketSec = 168, 3600 // 1-hour buckets
		default:
			hours, bucketSec = 1, 120 // 2-min buckets
		}

		points, err := db.GetResponseTimeTrend(id, hours, bucketSec)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if points == nil {
			points = []storage.ResponseTimePoint{}
		}
		c.JSON(http.StatusOK, points)
	}
}

// getUptimePercent returns uptime percentage for a probe over a time window.
// Query params: range=24h|30d
func getUptimePercent(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		rng := c.DefaultQuery("range", "24h")
		var hours int
		switch rng {
		case "30d":
			hours = 720
		default:
			hours = 24
		}

		pct, err := db.GetUptimePercent(id, hours)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"percent": pct})
	}
}

// deleteProbeResults deletes all probe result data.
func deleteProbeResults(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		n, err := db.DeleteAllProbeResults()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted", "count": n})
	}
}

// reorderProbesRequest is the body for PUT /api/v1/probes/reorder
type reorderProbesRequest struct {
	IDs []int64 `json:"ids"`
}

func reorderProbes(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req reorderProbesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(req.IDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ids required"})
			return
		}
		if err := db.ReorderProbes(req.IDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "reordered"})
	}
}
