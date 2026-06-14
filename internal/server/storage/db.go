package storage

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection.
type DB struct {
	*sql.DB
	path string
}

// Open creates/opens the SQLite database and runs migrations.
func Open(dbPath string) (*DB, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_loc=UTC&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Performance settings for SQLite
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-64000",  // 64MB cache
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			log.Printf("[db] warning: pragma failed: %s: %v", p, err)
		}
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &DB{db, dbPath}, nil
}

func migrate(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT    NOT NULL,
			address     TEXT    NOT NULL UNIQUE,
			server_url  TEXT    DEFAULT '',
			auth_token  TEXT    DEFAULT '',
			tags        TEXT    DEFAULT '',
			status      TEXT    DEFAULT 'unknown',
			last_seen   DATETIME DEFAULT NULL,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS metrics (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id    INTEGER NOT NULL,
			metric_type TEXT    NOT NULL,
			timestamp   DATETIME DEFAULT CURRENT_TIMESTAMP,
			data        TEXT    NOT NULL,
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_agent_type_time ON metrics(agent_id, metric_type, timestamp)`,

		`CREATE TABLE IF NOT EXISTS metrics_hourly (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id    INTEGER NOT NULL,
			metric_type TEXT    NOT NULL,
			hour_start  DATETIME NOT NULL,
			avg_value   REAL,
			max_value   REAL,
			min_value   REAL,
			data        TEXT    DEFAULT '{}',
			FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_hourly_agent_type_hour ON metrics_hourly(agent_id, metric_type, hour_start)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_metrics_hourly_unique ON metrics_hourly(agent_id, metric_type, hour_start)`,

		`CREATE TABLE IF NOT EXISTS web_probes (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			name            TEXT    NOT NULL,
			url             TEXT    NOT NULL,
			method          TEXT    DEFAULT 'GET',
			expected_status INTEGER DEFAULT 200,
			timeout_ms      INTEGER DEFAULT 5000,
			interval_s      INTEGER DEFAULT 60,
			enabled         INTEGER DEFAULT 1,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS probe_results (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			probe_id    INTEGER NOT NULL,
			timestamp   DATETIME DEFAULT CURRENT_TIMESTAMP,
			status_code INTEGER,
			latency_ms  REAL,
			error       TEXT    DEFAULT '',
			success     INTEGER DEFAULT 1,
			FOREIGN KEY (probe_id) REFERENCES web_probes(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_probe_results_probe_time ON probe_results(probe_id, timestamp)`,

		`CREATE TABLE IF NOT EXISTS alert_rules (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			name           TEXT    NOT NULL,
			description    TEXT    DEFAULT '',
			scope_type     TEXT    NOT NULL,
			scope_id       INTEGER DEFAULT NULL,
			metric_type    TEXT    NOT NULL,
			operator       TEXT    NOT NULL,
			threshold      REAL    NOT NULL,
			duration_s     INTEGER DEFAULT 0,
			enabled        INTEGER DEFAULT 1,
			wechat_webhook TEXT    DEFAULT '',
			cooldown_s         INTEGER DEFAULT 300,
			repeat_enabled     INTEGER DEFAULT 0,
			repeat_interval_s  INTEGER DEFAULT 300,
			created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS alert_events (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_id     INTEGER NOT NULL,
			agent_id    INTEGER DEFAULT NULL,
			probe_id    INTEGER DEFAULT NULL,
			severity    TEXT    DEFAULT 'warning',
			message     TEXT    NOT NULL,
			value       REAL,
			fired_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME DEFAULT NULL,
			FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_events_rule_time ON alert_events(rule_id, fired_at)`,

		`CREATE TABLE IF NOT EXISTS settings (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT    NOT NULL UNIQUE,
			password_hash TEXT    NOT NULL,
			token         TEXT    DEFAULT '',
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS sessions (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id     INTEGER NOT NULL,
			token       TEXT    NOT NULL UNIQUE,
			user_agent  TEXT    DEFAULT '',
			ip_address  TEXT    DEFAULT '',
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS widgets (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT    NOT NULL,
			type        TEXT    NOT NULL,
			url         TEXT    NOT NULL,
			api_token   TEXT    DEFAULT '',
			node        TEXT    DEFAULT '',
			config      TEXT    DEFAULT '{}',
			sort_order  INTEGER DEFAULT 0,
			enabled     INTEGER DEFAULT 1,
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

	}

	// Columns to add (table, column, ALTER statement)
	type addColumn struct {
		table  string
		column string
		sql    string
	}
	alterColumns := []addColumn{
		{"probe_results", "cert_issuer", `ALTER TABLE probe_results ADD COLUMN cert_issuer TEXT DEFAULT ''`},
		{"probe_results", "cert_not_after", `ALTER TABLE probe_results ADD COLUMN cert_not_after DATETIME DEFAULT NULL`},
		{"probe_results", "cert_days_left", `ALTER TABLE probe_results ADD COLUMN cert_days_left INTEGER DEFAULT NULL`},
		{"alert_rules", "repeat_enabled", `ALTER TABLE alert_rules ADD COLUMN repeat_enabled INTEGER DEFAULT 0`},
		{"alert_rules", "repeat_interval_s", `ALTER TABLE alert_rules ADD COLUMN repeat_interval_s INTEGER DEFAULT 300`},
		{"agents", "agent_version", `ALTER TABLE agents ADD COLUMN agent_version TEXT DEFAULT ''`},
		{"agents", "mode", `ALTER TABLE agents ADD COLUMN mode TEXT DEFAULT 'push'`},
		{"agents", "sort_order", `ALTER TABLE agents ADD COLUMN sort_order INTEGER DEFAULT 0`},
		{"agents", "server_url", `ALTER TABLE agents ADD COLUMN server_url TEXT DEFAULT ''`},
		{"web_probes", "sort_order", `ALTER TABLE web_probes ADD COLUMN sort_order INTEGER DEFAULT 0`},
		{"widgets", "widget_group", `ALTER TABLE widgets ADD COLUMN widget_group TEXT DEFAULT ''`},
		{"widgets", "description", `ALTER TABLE widgets ADD COLUMN description TEXT DEFAULT ''`},
		{"users", "token_created_at", `ALTER TABLE users ADD COLUMN token_created_at DATETIME DEFAULT ''`},
		{"agents", "intranet_url", `ALTER TABLE agents ADD COLUMN intranet_url TEXT DEFAULT ''`},
		{"agents", "extranet_url", `ALTER TABLE agents ADD COLUMN extranet_url TEXT DEFAULT ''`},
		{"sessions", "ip_address", `ALTER TABLE sessions ADD COLUMN ip_address TEXT DEFAULT ''`},
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	// Add columns only if they don't already exist
	for _, ac := range alterColumns {
		if !columnExists(db, ac.table, ac.column) {
			if _, err := db.Exec(ac.sql); err != nil {
				return fmt.Errorf("migration add column %s.%s failed: %w", ac.table, ac.column, err)
			}
		}
	}

	// Seed default settings
	defaultSettings := map[string]string{
		"default_wechat_webhook":  "",
		"poll_interval_s":         "30",
		"data_retention_days":     "3",
		"hourly_retention_days":   "30",
	}
	for key, value := range defaultSettings {
		db.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)", key, value)
	}

	return nil
}

// DatabaseSize returns the total size of the SQLite database files (db + wal + shm) in bytes.
func (db *DB) DatabaseSize() (int64, error) {
	var total int64
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if info, err := os.Stat(db.path + suffix); err == nil {
			total += info.Size()
		}
	}
	return total, nil
}

// PurgeAllData deletes all data from metrics, metrics_hourly, probe_results,
// alert_events tables while keeping structure, agents, probes, rules, settings, and users.
func (db *DB) PurgeAllData() (int64, error) {
	var total int64
	tables := []string{"metrics", "metrics_hourly", "probe_results", "alert_events"}
	for _, t := range tables {
		result, err := db.Exec("DELETE FROM " + t)
		if err != nil {
			return total, err
		}
		n, _ := result.RowsAffected()
		total += n
	}
	return total, nil
}

// columnExists checks if a column exists in a table using PRAGMA table_info.
func columnExists(db *sql.DB, table, column string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

// isBusyError checks if the error is a SQLITE_BUSY error.
func isBusyError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "database is locked")
}

// retryExec executes a SQL statement with retries on SQLITE_BUSY.
// It retries up to 3 times with a 100ms backoff each time.
func (db *DB) retryExec(query string, args ...interface{}) (sql.Result, error) {
	var (
		result sql.Result
		err    error
	)
	for i := 0; i < 3; i++ {
		result, err = db.Exec(query, args...)
		if !isBusyError(err) {
			return result, err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}
