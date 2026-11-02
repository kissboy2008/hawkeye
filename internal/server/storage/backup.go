package storage

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

	// Run first backup immediately
	db.doBackup(cfg)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
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

	// VACUUM INTO 的路径是 SQL 字面量（无法用占位符参数化），所以必须先规范成绝对路径
	// 再转义单引号：否则 backup.dir 里带一个 ' 就能把语句拼坏（最坏情况是执行到别的路径）。
	absDest, err := filepath.Abs(dest)
	if err != nil {
		log.Printf("[backup] resolve path %q failed: %v", dest, err)
		return
	}
	absDir, err := filepath.Abs(cfg.Dir)
	if err != nil || filepath.Dir(absDest) != absDir {
		log.Printf("[backup] refusing to write outside backup dir: %q", absDest)
		return
	}
	if _, statErr := os.Stat(absDest); statErr == nil {
		log.Printf("[backup] target already exists, skipping: %s", absDest)
		return
	}

	start := time.Now()
	// Use VACUUM INTO for a consistent hot backup
	quoted := strings.ReplaceAll(absDest, "'", "''")
	if _, err := db.retryExec(fmt.Sprintf("VACUUM INTO '%s'", quoted)); err != nil {
		log.Printf("[backup] failed: %v", err)
		return
	}
	// VACUUM INTO 期间 SQLite 会持有读事务（WAL 下不阻塞写入，但会拖慢 checkpoint），
	// 记录耗时便于判断是否需要挪到凌晨执行。
	log.Printf("[backup] wrote %s in %v", absDest, time.Since(start).Round(time.Millisecond))

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
		}
	}
}
