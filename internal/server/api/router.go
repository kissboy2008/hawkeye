package api

import (
	"io/fs"
	"net/http"
	"path"
	"time"

	"hawkeye/internal/models"
	"hawkeye/internal/server/alert"
	"hawkeye/internal/server/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Router sets up all API routes and returns the Gin engine.
func Router(db *storage.DB, hub *Hub, frontendFS http.FileSystem, downloadFS fs.FS, notifier *alert.Notifier, onMetrics func(int64, *models.AgentMetricsResponse), serverVersion string, corsOrigins []string, authDisabled bool, bgDir string) *gin.Engine {
	r := gin.Default()

	// CORS
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"*"}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
	}))

	// Public agent push endpoint (agents use their own auth token, no session auth)
	// Rate limited: 30 requests per minute per token
	r.POST("/api/v1/agents/push", RateLimitByTokenMiddleware(30, time.Minute), pushMetrics(db, onMetrics))

	// Homepage widget endpoint (public, for Homepage dashboard integration)
	r.GET("/api/v1/homepage/stats", getHomepageStats(db))

	// Version endpoint (public)
	r.GET("/api/v1/version", getVersion(db, serverVersion))

	// Auth routes (public)
	auth := r.Group("/api/v1/auth")
	{
		auth.GET("/check", checkHandler(db))
		auth.POST("/register", registerHandler(db))
		auth.POST("/login", loginHandler(db))
	}

	// API v1 group (authenticated)
	v1 := r.Group("/api/v1")
	if !authDisabled {
		v1.Use(AuthMiddleware(db))
	}
	{
		// Agent management
		agents := v1.Group("/agents")
		{
			agents.GET("", listAgents(db))
			agents.POST("", createAgent(db))
			agents.GET("/:id", getAgent(db))
			agents.PUT("/:id", updateAgent(db))
			agents.PUT("/:id/server-url", pushServerURL(db))
			agents.PUT("/:id/auth-token", pushAuthToken(db))
			agents.DELETE("/:id", deleteAgent(db))
			agents.POST("/:id/test", testAgent(db))
			agents.DELETE("/:id/metrics", deleteAgentMetrics(db))
			agents.PUT("/reorder", reorderAgents(db))
		}

		// Metrics
		v1.GET("/agents/:id/metrics/latest", getLatestMetrics(db))
		v1.GET("/agents/:id/metrics/:type", getMetricsTimeRange(db))

		// Web probes
		probes := v1.Group("/probes")
		{
			probes.GET("", listProbes(db))
			probes.POST("", createProbe(db))
			probes.GET("/:id", getProbe(db))
			probes.PUT("/:id", updateProbe(db))
			probes.DELETE("/:id", deleteProbe(db))
			probes.POST("/:id/check", checkProbe(db))
			probes.GET("/:id/results", getProbeResults(db))
			probes.GET("/:id/stats", getProbeStats(db))
			probes.GET("/:id/cert", getProbeCertInfo(db))
			probes.GET("/:id/uptime-bars", getUptimeBars(db))
			probes.GET("/:id/response-time", getResponseTimeTrend(db))
			probes.GET("/:id/uptime-percent", getUptimePercent(db))
			probes.DELETE("/results", deleteProbeResults(db))
			probes.PUT("/reorder", reorderProbes(db))
		}

		// Alert rules
		alerts := v1.Group("/alerts")
		{
			alerts.GET("/rules", listAlertRules(db))
			alerts.POST("/rules", createAlertRule(db))
			alerts.GET("/rules/:id", getAlertRule(db))
			alerts.PUT("/rules/:id", updateAlertRule(db))
			alerts.DELETE("/rules/:id", deleteAlertRule(db))
			alerts.GET("/events", listAlertEvents(db))
			alerts.DELETE("/events", deleteAllAlertEvents(db))
			alerts.POST("/test/:id", testAlertRule(db, notifier))
			alerts.POST("/test-webhook", testWebhook(notifier))
		}

		// Widgets
		widgets := v1.Group("/widgets")
		{
			widgets.GET("", listWidgets(db))
			widgets.POST("", createWidget(db))
			widgets.PUT("/:id", updateWidget(db))
			widgets.DELETE("/:id", deleteWidget(db))
			widgets.GET("/:id/data", getWidgetData(db))
			widgets.PUT("/reorder", reorderWidgets(db))
			widgets.PUT("/rename-group", renameWidgetGroup(db))
			widgets.PUT("/move", moveWidget(db))
		}

		// Background images (custom uploads)
		bg := v1.Group("/bg")
		{
			bg.POST("/upload", uploadBgImage(bgDir))
			bg.GET("/list", listBgImages(bgDir))
			bg.DELETE("/:filename", deleteBgImage(bgDir))
		}

		// Settings
		v1.GET("/settings", getSettings(db))
		v1.PUT("/settings", updateSettings(db, notifier))

		// Sessions
		v1.GET("/sessions", sessionsHandler(db))
		v1.DELETE("/sessions/:id", deleteSessionHandler(db))

		// Database
		v1.GET("/database/info", getDatabaseInfo(db))
		v1.DELETE("/database/purge", purgeDatabase(db))
	}

	// WebSocket
	r.GET("/ws", handleWebSocket(hub, db))

	// Agent downloads (binary + install script)
	if downloadFS != nil {
		dlSub, _ := fs.Sub(downloadFS, "downloads")
		r.StaticFS("/downloads", http.FS(dlSub))
	}

	// Custom background images
	if bgDir != "" {
		r.Static("/custom_bg", bgDir)
	}

	// Serve frontend (must be last - catch-all for SPA)
	if frontendFS != nil {
		fileServer := http.FileServer(frontendFS)
		noCacheFileServer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			fileServer.ServeHTTP(w, r)
		})
		r.NoRoute(func(c *gin.Context) {
			reqPath := c.Request.URL.Path
			// Try to serve the actual file first
			f, err := frontendFS.Open(path.Clean(reqPath)[1:]) // strip leading /
			if err == nil {
				f.Close()
				noCacheFileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			// Fall back to index.html for SPA client-side routing
			c.Request.URL.Path = "/"
			noCacheFileServer.ServeHTTP(c.Writer, c.Request)
		})
	}

	return r
}
