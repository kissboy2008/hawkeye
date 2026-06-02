package storage

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// BackupConfig holds backup settings.
type BackupConfig struct {
	Enabled   bool
	Dir       string
	MaxKeep   int
	IntervalH int
}

// RunBackup starts a periodic backup loop. Blocks until ctx is cancelled.
func (db *DB) RunBackup(ctx context.Context, cfg BackupConfig) {
	if !cfg.Enabled {
		log.Println("[backup] disabled")
		return
	}

	if cfg.Dir == "" {
		cfg.Dir = "./data/backups"
	}
	if cfg.MaxKeep <= 0 {
		cfg.MaxKeep = 7
	}
	if cfg.IntervalH <= 0 {
		cfg.IntervalH = 24
	}

	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		log.Printf("[backup] failed to create backup dir: %v", err)
		return
	}

	interval := time.Duration(cfg.IntervalH) * time.Hour
	log.Printf("[backup] enabled, interval=%v, dir=%s, max_keep=%d", interval, cfg.Dir, cfg.MaxKeep)

	// Run first backup immediately
	db.doBackup(cfg)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[backup] stopped")
			return
		case <-ticker.C:
			db.doBackup(cfg)
		}
	}
}

func (db *DB) doBackup(cfg BackupConfig) {
	ts := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("hawkeye-%s.db", ts)
	dest := filepath.Join(cfg.Dir, filename)

	// Use VACUUM INTO for a consistent hot backup
	_, err := db.Exec(fmt.Sprintf(`VACUUM INTO '%s'`, dest))
	if err != nil {
		log.Printf("[backup] failed: %v", err)
		return
	}

	log.Printf("[backup] created: %s", dest)

	// Cleanup old backups
	db.cleanOldBackups(cfg)
}

func (db *DB) cleanOldBackups(cfg BackupConfig) {
	entries, err := os.ReadDir(cfg.Dir)
	if err != nil {
		return
	}

	// Filter only backup files
	var backups []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".db" {
			backups = append(backups, e)
		}
	}

	if len(backups) <= cfg.MaxKeep {
		return
	}

	// Sort by name (which includes timestamp, so oldest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name() < backups[j].Name()
	})

	// Remove oldest
	toRemove := backups[:len(backups)-cfg.MaxKeep]
	for _, f := range toRemove {
		path := filepath.Join(cfg.Dir, f.Name())
		if err := os.Remove(path); err != nil {
			log.Printf("[backup] failed to remove old backup %s: %v", path, err)
		} else {
			log.Printf("[backup] removed old backup: %s", f.Name())
		}
	}
}
