package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"hawkeye/internal/agent"
	"hawkeye/internal/config"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "./configs/agent.yaml", "path to agent config file")
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("hawkeye-agent %s\n", version)
		os.Exit(0)
	}

	// Load config (fallback to defaults if file not found)
	cfg, err := config.LoadAgentConfig(*configPath)
	if err != nil {
		cfg = &config.AgentConfig{}
		log.Printf("[agent] config file not found, using defaults")
	}

	// Create collector
	collector := agent.NewCollector(version)

	// Create HTTP handler
	handler := agent.NewHandler(collector, cfg.Auth.Token, *configPath)

	// Register routes
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create server
	srv := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: mux,
	}

	// Create push reporter
	reporter := agent.NewReporter(collector, cfg, version)

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Start push reporter
	go reporter.Run(ctx)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[agent] shutting down...")
		cancel()
		srv.Close()
	}()

	log.Printf("[agent] hawkeye-agent %s starting on %s", version, cfg.Server.Listen)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[agent] server error: %v", err)
	}
	log.Println("[agent] stopped")
}
