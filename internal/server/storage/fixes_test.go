package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// 验证 #7：ValidateToken 不再每次校验都 UPDATE sessions。
// 旧实现每次请求都会写一次库；现在只在有效期过半后写，所以刚创建的会话
// 校验前后 created_at 必须完全一致。
func TestValidateTokenDoesNotWriteEveryTime(t *testing.T) {
	db := openTestDB(t)

	_, token, err := db.Register("tony", "password123", "127.0.0.1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	var before string
	if err := db.QueryRow(`SELECT created_at FROM sessions WHERE token = ?`, token).Scan(&before); err != nil {
		t.Fatalf("read session: %v", err)
	}

	// 连续校验三次（模拟三个 API 请求）
	for i := 0; i < 3; i++ {
		if _, err := db.ValidateToken(token); err != nil {
			t.Fatalf("validate: %v", err)
		}
	}

	var after string
	if err := db.QueryRow(`SELECT created_at FROM sessions WHERE token = ?`, token).Scan(&after); err != nil {
		t.Fatalf("read session: %v", err)
	}
	if before != after {
		t.Fatalf("session was rewritten on every validation: %q -> %q", before, after)
	}

	// 把会话时间改成"已过 6 天"（超过 TokenMaxAge/2），校验应触发一次续期
	old := time.Now().Add(-6 * 24 * time.Hour).UTC().Format(time.RFC3339)
	if _, err := db.Exec(`UPDATE sessions SET created_at = ? WHERE token = ?`, old, token); err != nil {
		t.Fatalf("age session: %v", err)
	}
	if _, err := db.ValidateToken(token); err != nil {
		t.Fatalf("validate aged: %v", err)
	}
	var renewed string
	db.QueryRow(`SELECT created_at FROM sessions WHERE token = ?`, token).Scan(&renewed)
	if renewed == old {
		t.Fatal("aged session was not renewed")
	}
}

// 验证 #8：清理 probe_results 走分批删除，且 timestamp 单列索引存在。
func TestCleanOldProbeResultsBatchesAndIndexExists(t *testing.T) {
	db := openTestDB(t)

	var idx int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_probe_results_time'`).Scan(&idx); err != nil {
		t.Fatalf("check index: %v", err)
	}
	if idx != 1 {
		t.Fatal("idx_probe_results_time missing")
	}

	res, err := db.Exec(`INSERT INTO web_probes (name, url) VALUES ('t', 'http://127.0.0.1')`)
	if err != nil {
		t.Fatalf("insert probe: %v", err)
	}
	probeID, _ := res.LastInsertId()

	// 6000 行老数据（跨越一个 5000 的批次）+ 100 行新数据
	for i := 0; i < 6000; i++ {
		if _, err := db.Exec(`INSERT INTO probe_results (probe_id, timestamp, success) VALUES (?, datetime('now','-10 days'), 1)`, probeID); err != nil {
			t.Fatalf("insert old: %v", err)
		}
	}
	for i := 0; i < 100; i++ {
		if _, err := db.Exec(`INSERT INTO probe_results (probe_id, timestamp, success) VALUES (?, datetime('now'), 1)`, probeID); err != nil {
			t.Fatalf("insert new: %v", err)
		}
	}

	n, err := db.CleanOldProbeResults(3)
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if n != 6000 {
		t.Fatalf("expected 6000 deleted, got %d", n)
	}
	var left int
	db.QueryRow(`SELECT COUNT(*) FROM probe_results`).Scan(&left)
	if left != 100 {
		t.Fatalf("expected 100 rows left, got %d", left)
	}
}

// 验证 #24：备份写到目录内的绝对路径，目录名里带单引号也不会把 SQL 拼坏。
func TestBackupEscapesQuotesInDir(t *testing.T) {
	db := openTestDB(t)
	dir := filepath.Join(t.TempDir(), "it's a dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	db.doBackup(BackupConfig{Enabled: true, Dir: dir, MaxKeep: 3, IntervalH: 24})

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 backup file, got %d", len(entries))
	}
	info, err := entries[0].Info()
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("backup file is empty")
	}
}
