package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"hawkeye/internal/config"
	"hawkeye/internal/models"
	"hawkeye/internal/server/alert"
	"hawkeye/internal/server/api"
	"hawkeye/internal/server/poller"
	"hawkeye/internal/server/probe"
	"hawkeye/internal/server/storage"
	"hawkeye/internal/server"
	"hawkeye/internal/static"
	ver "hawkeye/internal/version"

	"github.com/gin-gonic/gin"
)

var version = ver.Get()

func main() {
	configPath := flag.String("config", "./configs/server.yaml", "path to server config file")
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("hawkeye-server %s\n", version)
		os.Exit(0)
	}

	// Load config (fallback to defaults if file not found)
	cfg, err := config.LoadServerConfig(*configPath)
	if err != nil {
		cfg = &config.ServerConfig{}
		log.Printf("[server] config file not found, using defaults")
	}

	gin.SetMode(cfg.Server.Mode)

	// Open database
	db, err := storage.Open(cfg.Database.Path)
	if err != nil {
		log.Fatalf("[server] failed to open database: %v", err)
	}
	defer db.Close()
	log.Println("[server] database opened:", cfg.Database.Path)

	// WebSocket hub
	hub := api.NewHub()

	// Get global webhook
	globalWebhook := cfg.Alerts.WechatWebhook
	if globalWebhook == "" {
		globalWebhook, _ = db.GetSetting("default_wechat_webhook")
	}

	// Notifier and alert engine
	notifier := alert.NewNotifier(globalWebhook)
	engine := alert.NewEngine(db, notifier, cfg.Alerts.CheckIntervalS)

	// Prepare embedded frontend
	subFS, err := fs.Sub(static.FrontendFS, "dist")
	if err != nil {
		log.Printf("[server] warning: frontend assets not embedded: %v", err)
		subFS = nil
	}

	var httpFS http.FileSystem
	if subFS != nil {
		httpFS = http.FS(subFS)
	}

	// API router (pass onMetrics callback for WebSocket broadcast)
	router := api.Router(db, hub, httpFS, server.DownloadsFS, notifier, func(agentID int64, metrics *models.AgentMetricsResponse) {
		api.BroadcastMetrics(hub, agentID, metrics)
	}, version, cfg.Server.CORSOrigins, cfg.Auth.Disabled, cfg.BgImagesDir)

	// Heartbeat checker (marks agents offline when no contact)
	hc := poller.NewChecker(db, cfg.Poller.IntervalS, cfg.Poller.OfflineTimeoutS)

	// Active poller (pulls metrics from agents via HTTP)
	ap := poller.NewActivePoller(db, func(agentID int64, metrics *models.AgentMetricsResponse) {
		api.BroadcastMetrics(hub, agentID, metrics)
	}, cfg.Poller.IntervalS, cfg.Poller.TimeoutS)

	// Probe scheduler
	ps := probe.NewScheduler(db, cfg.Probes.CheckIntervalS)
	ps.OnResult(func(result *models.ProbeResult) {
		api.BroadcastProbeResult(hub, result)
	})

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// Start background services
	wg.Add(6)
	go func() { defer wg.Done(); hc.Run(ctx) }()
	go func() { defer wg.Done(); ap.Run(ctx) }()
	go func() { defer wg.Done(); ps.Run(ctx) }()
	go func() { defer wg.Done(); engine.Run(ctx) }()
	go func() { defer wg.Done(); runDataCleanup(ctx, db, cfg) }()
	go func() {
		defer wg.Done()
		db.RunBackup(ctx, storage.BackupConfig{
			Enabled:   cfg.Backup.Enabled,
			Dir:       cfg.Backup.Dir,
			MaxKeep:   cfg.Backup.MaxKeep,
			IntervalH: cfg.Backup.IntervalH,
		})
	}()

	// HTTP server
	srv := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: router,
	}

	// Handle shutdown signals
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[server] shutting down...")
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("[server] hawkeye-server %s starting on %s", version, cfg.Server.Listen)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[server] server error: %v", err)
	}

	// Wait for all background goroutines to finish
	wg.Wait()
	log.Println("[server] stopped")
}

// runDataCleanup periodically aggregates and cleans old metrics.
func runDataCleanup(ctx context.Context, db *storage.DB, cfg *config.ServerConfig) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := db.AggregateHourlyMetrics()
			if err != nil {
				log.Printf("[cleanup] aggregation error: %v", err)
			} else if count > 0 {
				log.Printf("[cleanup] aggregated %d hourly records", count)
			}

			deleted, err := db.CleanOldMetrics(cfg.Database.RetentionDays)
			if err != nil {
				log.Printf("[cleanup] cleanup error: %v", err)
			} else if deleted > 0 {
				log.Printf("[cleanup] deleted %d old records (%d-day retention)", deleted, cfg.Database.RetentionDays)
			}

			hDeleted, err := db.CleanOldHourlyMetrics(cfg.Database.HourlyRetentionDays)
			if err != nil {
				log.Printf("[cleanup] hourly cleanup error: %v", err)
			} else if hDeleted > 0 {
				log.Printf("[cleanup] deleted %d old hourly records", hDeleted)
			}
		}
	}
}
