package storage

import (
	"database/sql"
	"fmt"
	"time"

	"hawkeye/internal/models"
)

// migrateAlertV2 建 v2 告警相关的表并迁移旧规则。
//
// 设计要点：
//   - alert_rule_targets：一条规则 × 一个目标一行，每台机器/每个网站独立阈值
//   - alert_rule_state：运行时状态落库，first_true_at 用墙钟，重启不清零
//   - alert_outbox：通知失败兜底队列，进程重启不丢
//   - 不建渠道/路由/静默表：单用户自用，webhook 走 settings + 规则可选覆盖
func migrateAlertV2(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS alert_rule_targets (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_id        INTEGER NOT NULL,
			target_type    TEXT    NOT NULL,
			target_id      INTEGER NOT NULL,
			enabled        INTEGER DEFAULT 1,
			operator       TEXT    DEFAULT '',
			threshold      REAL    DEFAULT NULL,
			for_s          INTEGER DEFAULT NULL,
			silenced_until DATETIME DEFAULT NULL,
			created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(rule_id, target_type, target_id),
			FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_targets_rule ON alert_rule_targets(rule_id)`,

		`CREATE TABLE IF NOT EXISTS alert_rule_state (
			rule_id          INTEGER NOT NULL,
			target_type      TEXT    NOT NULL,
			target_id        INTEGER NOT NULL,
			status           TEXT    DEFAULT 'ok',
			first_true_at    DATETIME DEFAULT NULL,
			last_eval_at     DATETIME DEFAULT NULL,
			last_value       REAL    DEFAULT 0,
			fired_at         DATETIME DEFAULT NULL,
			last_notify_at   DATETIME DEFAULT NULL,
			notify_count     INTEGER DEFAULT 0,
			suppressed_count INTEGER DEFAULT 0,
			PRIMARY KEY (rule_id, target_type, target_id)
		)`,

		`CREATE TABLE IF NOT EXISTS alert_outbox (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_id       INTEGER NOT NULL,
			payload       TEXT    NOT NULL,
			attempts      INTEGER DEFAULT 0,
			next_retry_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			status        TEXT    DEFAULT 'pending',
			last_error    TEXT    DEFAULT '',
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_outbox_status ON alert_outbox(status, next_retry_at)`,

		// 清理历史遗留的孤儿行：旧版删除规则时没有级联，targets/state 会残留在库里
		`DELETE FROM alert_rule_targets WHERE rule_id NOT IN (SELECT id FROM alert_rules)`,
		`DELETE FROM alert_rule_state WHERE rule_id NOT IN (SELECT id FROM alert_rules)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("alert v2 migration: %w", err)
		}
	}

	alterColumns := []struct{ table, column, sql string }{
		{"alert_rules", "target_class", `ALTER TABLE alert_rules ADD COLUMN target_class TEXT DEFAULT ''`},
		{"alert_rules", "kind", `ALTER TABLE alert_rules ADD COLUMN kind TEXT DEFAULT ''`},
		{"alert_rules", "group_wait_s", `ALTER TABLE alert_rules ADD COLUMN group_wait_s INTEGER DEFAULT 0`},
		{"alert_rules", "silenced_until", `ALTER TABLE alert_rules ADD COLUMN silenced_until DATETIME DEFAULT NULL`},
	}
	for _, ac := range alterColumns {
		if !columnExists(db, ac.table, ac.column) {
			if _, err := db.Exec(ac.sql); err != nil {
				return fmt.Errorf("alert v2 add column %s.%s: %w", ac.table, ac.column, err)
			}
		}
	}

	// 回填 target_class / kind（只填空值，不覆盖用户后来改的）
	if _, err := db.Exec(`
		UPDATE alert_rules SET target_class = CASE
			WHEN metric_type LIKE 'probe%' OR metric_type = 'cert_expiry' THEN 'probe'
			ELSE 'agent' END
		WHERE target_class IS NULL OR target_class = ''`); err != nil {
		return fmt.Errorf("alert v2 backfill target_class: %w", err)
	}
	if _, err := db.Exec(`
		UPDATE alert_rules SET kind = CASE
			WHEN metric_type = 'agent_offline' THEN 'availability'
			ELSE 'threshold' END
		WHERE kind IS NULL OR kind = ''`); err != nil {
		return fmt.Errorf("alert v2 backfill kind: %w", err)
	}

	return expandLegacyRuleTargets(db)
}

// expandLegacyRuleTargets 把还没有目标行的旧规则展开成「每个目标一行」。
//
// 旧模型是 scope_id 为空 = 覆盖全部对象，没有逐台配置能力；
// v2 必须显式列目标，所以这里一次性把现有机器/网站列进去，
// 参数留空（= 继承规则默认值），语义不变但从此可以逐台调。
func expandLegacyRuleTargets(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT r.id, r.target_class, r.scope_id
		FROM alert_rules r
		WHERE NOT EXISTS (SELECT 1 FROM alert_rule_targets t WHERE t.rule_id = r.id)`)
	if err != nil {
		return fmt.Errorf("query legacy rules: %w", err)
	}
	type legacy struct {
		id    int64
		class string
		scope *int64
	}
	var rules []legacy
	for rows.Next() {
		var l legacy
		var scope sql.NullInt64
		if err := rows.Scan(&l.id, &l.class, &scope); err != nil {
			rows.Close()
			return err
		}
		if scope.Valid {
			v := scope.Int64
			l.scope = &v
		}
		rules = append(rules, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, l := range rules {
		class := l.class
		if class == "" {
			class = "agent"
		}
		var ids []int64
		if l.scope != nil {
			ids = []int64{*l.scope}
		} else if class == "probe" {
			rs, err := db.Query(`SELECT id FROM web_probes`)
			if err != nil {
				return err
			}
			for rs.Next() {
				var id int64
				if err := rs.Scan(&id); err != nil {
					rs.Close()
					return err
				}
				ids = append(ids, id)
			}
			rs.Close()
		} else {
			rs, err := db.Query(`SELECT id FROM agents`)
			if err != nil {
				return err
			}
			for rs.Next() {
				var id int64
				if err := rs.Scan(&id); err != nil {
					rs.Close()
					return err
				}
				ids = append(ids, id)
			}
			rs.Close()
		}

		for _, id := range ids {
			if _, err := db.Exec(`
				INSERT OR IGNORE INTO alert_rule_targets (rule_id, target_type, target_id, enabled)
				VALUES (?, ?, ?, 1)`, l.id, class, id); err != nil {
				return fmt.Errorf("expand rule %d target %d: %w", l.id, id, err)
			}
		}
	}
	return nil
}

// ---------- alert_rule_targets ----------

const targetCols = `id, rule_id, target_type, target_id, enabled, operator, threshold, for_s, silenced_until, created_at`

func scanTarget(sc interface{ Scan(...interface{}) error }) (*models.AlertRuleTarget, error) {
	var t models.AlertRuleTarget
	var enabled int
	var operator sql.NullString
	var threshold sql.NullFloat64
	var forS sql.NullInt64
	var silenced sql.NullTime
	if err := sc.Scan(&t.ID, &t.RuleID, &t.TargetType, &t.TargetID, &enabled,
		&operator, &threshold, &forS, &silenced, &t.CreatedAt); err != nil {
		return nil, err
	}
	t.Enabled = enabled == 1
	t.Operator = operator.String
	if threshold.Valid {
		v := threshold.Float64
		t.Threshold = &v
	}
	if forS.Valid {
		v := int(forS.Int64)
		t.ForS = &v
	}
	if silenced.Valid {
		v := silenced.Time
		t.SilencedUntil = &v
	}
	return &t, nil
}

func (db *DB) GetAlertRuleTargets(ruleID int64) ([]models.AlertRuleTarget, error) {
	rows, err := db.Query(`SELECT `+targetCols+` FROM alert_rule_targets WHERE rule_id = ? ORDER BY target_type, target_id`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.AlertRuleTarget
	for rows.Next() {
		t, err := scanTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (db *DB) GetAlertRuleTarget(ruleID int64, targetType string, targetID int64) (*models.AlertRuleTarget, error) {
	row := db.QueryRow(`SELECT `+targetCols+` FROM alert_rule_targets WHERE rule_id = ? AND target_type = ? AND target_id = ?`,
		ruleID, targetType, targetID)
	t, err := scanTarget(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (db *DB) UpsertAlertRuleTarget(t *models.AlertRuleTarget) (int64, error) {
	enabled := 0
	if t.Enabled {
		enabled = 1
	}
	if _, err := db.Exec(`
		INSERT INTO alert_rule_targets (rule_id, target_type, target_id, enabled, operator, threshold, for_s, silenced_until)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rule_id, target_type, target_id) DO UPDATE SET
			enabled = excluded.enabled,
			operator = excluded.operator,
			threshold = excluded.threshold,
			for_s = excluded.for_s,
			silenced_until = excluded.silenced_until`,
		t.RuleID, t.TargetType, t.TargetID, enabled, t.Operator,
		nullableFloat(t.Threshold), nullableInt(t.ForS), nullableTime(t.SilencedUntil)); err != nil {
		return 0, err
	}
	got, err := db.GetAlertRuleTarget(t.RuleID, t.TargetType, t.TargetID)
	if err != nil || got == nil {
		return 0, err
	}
	return got.ID, nil
}

func (db *DB) DeleteAlertRuleTarget(id int64) error {
	_, err := db.Exec(`DELETE FROM alert_rule_targets WHERE id = ?`, id)
	return err
}

func (db *DB) DeleteAlertRuleTargetsByRule(ruleID int64) error {
	_, err := db.Exec(`DELETE FROM alert_rule_targets WHERE rule_id = ?`, ruleID)
	return err
}

func nullableFloat(v *float64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func nullableInt(v *int) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func nullableTime(v *time.Time) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

// ---------- alert_rule_state ----------

func (db *DB) GetAlertRuleState(ruleID int64, targetType string, targetID int64) (*models.AlertRuleState, error) {
	row := db.QueryRow(`
		SELECT rule_id, target_type, target_id, status, first_true_at, last_eval_at, last_value,
		       fired_at, last_notify_at, notify_count, suppressed_count
		FROM alert_rule_state WHERE rule_id = ? AND target_type = ? AND target_id = ?`,
		ruleID, targetType, targetID)
	var st models.AlertRuleState
	var first, lastEval, fired, lastNotify sql.NullTime
	err := row.Scan(&st.RuleID, &st.TargetType, &st.TargetID, &st.Status, &first, &lastEval,
		&st.LastValue, &fired, &lastNotify, &st.NotifyCount, &st.SuppressedCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if first.Valid {
		v := first.Time
		st.FirstTrueAt = &v
	}
	if lastEval.Valid {
		v := lastEval.Time
		st.LastEvalAt = &v
	}
	if fired.Valid {
		v := fired.Time
		st.FiredAt = &v
	}
	if lastNotify.Valid {
		v := lastNotify.Time
		st.LastNotifyAt = &v
	}
	return &st, nil
}

// GetAlertStatesByRule 一次取一条规则下所有目标的状态（面板用，避免 N 次查询）。
func (db *DB) GetAlertStatesByRule(ruleID int64) (map[string]*models.AlertRuleState, error) {
	rows, err := db.Query(`
		SELECT rule_id, target_type, target_id, status, first_true_at, last_eval_at, last_value,
		       fired_at, last_notify_at, notify_count, suppressed_count
		FROM alert_rule_state WHERE rule_id = ?`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]*models.AlertRuleState)
	for rows.Next() {
		var st models.AlertRuleState
		var first, lastEval, fired, lastNotify sql.NullTime
		if err := rows.Scan(&st.RuleID, &st.TargetType, &st.TargetID, &st.Status, &first, &lastEval,
			&st.LastValue, &fired, &lastNotify, &st.NotifyCount, &st.SuppressedCount); err != nil {
			return nil, err
		}
		if first.Valid {
			v := first.Time
			st.FirstTrueAt = &v
		}
		if lastEval.Valid {
			v := lastEval.Time
			st.LastEvalAt = &v
		}
		if fired.Valid {
			v := fired.Time
			st.FiredAt = &v
		}
		if lastNotify.Valid {
			v := lastNotify.Time
			st.LastNotifyAt = &v
		}
		s := st
		out[targetKey(st.TargetType, st.TargetID)] = &s
	}
	return out, rows.Err()
}

func targetKey(targetType string, targetID int64) string {
	return fmt.Sprintf("%s/%d", targetType, targetID)
}

func (db *DB) SaveAlertRuleState(st *models.AlertRuleState) error {
	_, err := db.Exec(`
		INSERT INTO alert_rule_state (rule_id, target_type, target_id, status, first_true_at, last_eval_at,
			last_value, fired_at, last_notify_at, notify_count, suppressed_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rule_id, target_type, target_id) DO UPDATE SET
			status = excluded.status,
			first_true_at = excluded.first_true_at,
			last_eval_at = excluded.last_eval_at,
			last_value = excluded.last_value,
			fired_at = excluded.fired_at,
			last_notify_at = excluded.last_notify_at,
			notify_count = excluded.notify_count,
			suppressed_count = excluded.suppressed_count`,
		st.RuleID, st.TargetType, st.TargetID, st.Status, nullableTime(st.FirstTrueAt),
		nullableTime(st.LastEvalAt), st.LastValue, nullableTime(st.FiredAt),
		nullableTime(st.LastNotifyAt), st.NotifyCount, st.SuppressedCount)
	return err
}

func (db *DB) DeleteAlertRuleState(ruleID int64, targetType string, targetID int64) error {
	_, err := db.Exec(`DELETE FROM alert_rule_state WHERE rule_id = ? AND target_type = ? AND target_id = ?`,
		ruleID, targetType, targetID)
	return err
}

func (db *DB) DeleteAlertStatesByRule(ruleID int64) error {
	_, err := db.Exec(`DELETE FROM alert_rule_state WHERE rule_id = ?`, ruleID)
	return err
}

// CountAlertStates 统计各状态数量（健康面板用）。
func (db *DB) CountAlertStates() (map[string]int, error) {
	rows, err := db.Query(`SELECT status, COUNT(*) FROM alert_rule_state GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[s] = n
	}
	return out, rows.Err()
}

// ---------- alert_outbox ----------

func (db *DB) EnqueueOutbox(ruleID int64, payload string) error {
	_, err := db.Exec(`INSERT INTO alert_outbox (rule_id, payload, next_retry_at) VALUES (?, ?, ?)`,
		ruleID, payload, time.Now())
	return err
}

func (db *DB) GetDueOutbox(limit int) ([]models.AlertOutbox, error) {
	rows, err := db.Query(`
		SELECT id, rule_id, payload, attempts, next_retry_at, status, last_error, created_at
		FROM alert_outbox WHERE status = 'pending' AND next_retry_at <= ? ORDER BY id LIMIT ?`,
		time.Now(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.AlertOutbox
	for rows.Next() {
		var o models.AlertOutbox
		if err := rows.Scan(&o.ID, &o.RuleID, &o.Payload, &o.Attempts, &o.NextRetryAt,
			&o.Status, &o.LastError, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (db *DB) MarkOutboxSent(id int64) error {
	_, err := db.Exec(`UPDATE alert_outbox SET status = 'sent', last_error = '' WHERE id = ?`, id)
	return err
}

func (db *DB) MarkOutboxRetry(id int64, attempts int, nextRetry time.Time, lastErr string) error {
	_, err := db.Exec(`UPDATE alert_outbox SET attempts = ?, next_retry_at = ?, last_error = ? WHERE id = ?`,
		attempts, nextRetry, lastErr, id)
	return err
}

func (db *DB) MarkOutboxFailed(id int64, lastErr string) error {
	_, err := db.Exec(`UPDATE alert_outbox SET status = 'failed', last_error = ? WHERE id = ?`, lastErr, id)
	return err
}

func (db *DB) CountOutboxByStatus() (map[string]int, error) {
	rows, err := db.Query(`SELECT status, COUNT(*) FROM alert_outbox GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[s] = n
	}
	return out, rows.Err()
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
