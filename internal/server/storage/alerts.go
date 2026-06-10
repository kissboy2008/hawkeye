package storage

import (
	"database/sql"
	"time"

	"hawkeye/internal/models"
)

var cst = time.FixedZone("CST", 8*3600)

// ========== Web Probes CRUD ==========

func (db *DB) CreateProbe(p *models.WebProbe) (int64, error) {
	result, err := db.Exec(
		`INSERT INTO web_probes (name, url, method, expected_status, timeout_ms, interval_s, enabled, sort_order)
		 VALUES (?, ?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order),0)+1 FROM web_probes))`,
		p.Name, p.URL, p.Method, p.ExpectedStatus, p.TimeoutMs, p.IntervalS, boolToInt(p.Enabled),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) GetProbe(id int64) (*models.WebProbe, error) {
	p := &models.WebProbe{}
	var enabled int
	err := db.QueryRow(
		`SELECT id, name, url, method, expected_status, timeout_ms, interval_s, enabled, sort_order, created_at, updated_at
		 FROM web_probes WHERE id=?`, id,
	).Scan(&p.ID, &p.Name, &p.URL, &p.Method, &p.ExpectedStatus, &p.TimeoutMs, &p.IntervalS, &enabled, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	p.Enabled = intToBool(enabled)
	return p, err
}

func (db *DB) GetAllProbes() ([]models.WebProbe, error) {
	rows, err := db.Query(
		`SELECT id, name, url, method, expected_status, timeout_ms, interval_s, enabled, sort_order, created_at, updated_at
		 FROM web_probes ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var probes []models.WebProbe
	for rows.Next() {
		var p models.WebProbe
		var enabled int
		if err := rows.Scan(&p.ID, &p.Name, &p.URL, &p.Method, &p.ExpectedStatus, &p.TimeoutMs, &p.IntervalS, &enabled, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Enabled = intToBool(enabled)
		probes = append(probes, p)
	}
	return probes, nil
}

func (db *DB) GetEnabledProbes() ([]models.WebProbe, error) {
	rows, err := db.Query(
		`SELECT id, name, url, method, expected_status, timeout_ms, interval_s, enabled, sort_order, created_at, updated_at
		 FROM web_probes WHERE enabled=1 ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var probes []models.WebProbe
	for rows.Next() {
		var p models.WebProbe
		var enabled int
		if err := rows.Scan(&p.ID, &p.Name, &p.URL, &p.Method, &p.ExpectedStatus, &p.TimeoutMs, &p.IntervalS, &enabled, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Enabled = intToBool(enabled)
		probes = append(probes, p)
	}
	return probes, nil
}

func (db *DB) UpdateProbe(p *models.WebProbe) error {
	_, err := db.Exec(
		`UPDATE web_probes SET name=?, url=?, method=?, expected_status=?, timeout_ms=?, interval_s=?, enabled=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=?`,
		p.Name, p.URL, p.Method, p.ExpectedStatus, p.TimeoutMs, p.IntervalS, boolToInt(p.Enabled), p.ID,
	)
	return err
}

func (db *DB) DeleteProbe(id int64) error {
	_, err := db.Exec(`DELETE FROM web_probes WHERE id=?`, id)
	return err
}

// ReorderProbes updates sort_order for all given probe IDs in order.
func (db *DB) ReorderProbes(ids []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE web_probes SET sort_order=? WHERE id=?`, i+1, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ========== Probe Results ==========

func (db *DB) InsertProbeResult(r *models.ProbeResult) error {
	_, err := db.Exec(
		`INSERT INTO probe_results (probe_id, status_code, latency_ms, error, success, cert_issuer, cert_not_after, cert_days_left)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ProbeID, r.StatusCode, r.LatencyMs, r.Error, boolToInt(r.Success),
		r.CertIssuer, r.CertNotAfter, r.CertDaysLeft,
	)
	return err
}

func (db *DB) GetLatestProbeResult(probeID int64) (*models.ProbeResult, error) {
	r := &models.ProbeResult{}
	var success int
	err := db.QueryRow(
		`SELECT id, probe_id, timestamp, status_code, latency_ms, error, success, cert_issuer, cert_not_after, cert_days_left
		 FROM probe_results WHERE probe_id=? ORDER BY timestamp DESC LIMIT 1`, probeID,
	).Scan(&r.ID, &r.ProbeID, &r.Timestamp, &r.StatusCode, &r.LatencyMs, &r.Error, &success,
		&r.CertIssuer, &r.CertNotAfter, &r.CertDaysLeft)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	r.Success = intToBool(success)
	return r, err
}

func (db *DB) GetProbeResults(probeID int64, from, to time.Time, limit int) ([]models.ProbeResult, error) {
	query := `SELECT id, probe_id, timestamp, status_code, latency_ms, error, success, cert_issuer, cert_not_after, cert_days_left
			  FROM probe_results WHERE probe_id=?`
	args := []interface{}{probeID}

	if !from.IsZero() {
		query += ` AND timestamp >= ?`
		args = append(args, from.UTC().Format("2006-01-02 15:04:05"))
	}
	if !to.IsZero() {
		query += ` AND timestamp <= ?`
		args = append(args, to.UTC().Format("2006-01-02 15:04:05"))
	}

	query += ` ORDER BY timestamp DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.ProbeResult
	for rows.Next() {
		var r models.ProbeResult
		var success int
		if err := rows.Scan(&r.ID, &r.ProbeID, &r.Timestamp, &r.StatusCode, &r.LatencyMs, &r.Error, &success,
			&r.CertIssuer, &r.CertNotAfter, &r.CertDaysLeft); err != nil {
			return nil, err
		}
		r.Success = intToBool(success)
		results = append(results, r)
	}
	return results, nil
}

func (db *DB) CleanOldProbeResults(retentionDays int) (int64, error) {
	result, err := db.Exec(
		`DELETE FROM probe_results WHERE timestamp < datetime('now', ? || ' days')`,
		-retentionDays,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (db *DB) DeleteAllProbeResults() (int64, error) {
	result, err := db.Exec(`DELETE FROM probe_results`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ProbeStats holds aggregated statistics for a probe.
type ProbeStats struct {
	ProbeID      int64   `json:"probe_id"`
	TotalChecks  int64   `json:"total_checks"`
	SuccessCount int64   `json:"success_count"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	UpPercent    float64 `json:"up_percent"`
}

func (db *DB) GetProbeStats(probeID int64) (*ProbeStats, error) {
	s := &ProbeStats{ProbeID: probeID}
	err := db.QueryRow(
		`SELECT COALESCE(COUNT(*),0), COALESCE(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END),0), COALESCE(AVG(latency_ms),0)
		 FROM probe_results WHERE probe_id=?`, probeID,
	).Scan(&s.TotalChecks, &s.SuccessCount, &s.AvgLatencyMs)
	if err != nil {
		return nil, err
	}
	if s.TotalChecks > 0 {
		s.UpPercent = float64(s.SuccessCount) / float64(s.TotalChecks) * 100
	}
	return s, nil
}

// GetCertInfo returns the latest certificate info for a probe.
type CertInfo struct {
	Issuer   string `json:"issuer"`
	NotAfter string `json:"not_after"`
	DaysLeft int    `json:"days_left"`
}

func (db *DB) GetLatestCertInfo(probeID int64) (*CertInfo, error) {
	c := &CertInfo{}
	err := db.QueryRow(
		`SELECT cert_issuer, cert_not_after, COALESCE(cert_days_left,0)
		 FROM probe_results WHERE probe_id=? AND cert_issuer != '' ORDER BY timestamp DESC LIMIT 1`, probeID,
	).Scan(&c.Issuer, &c.NotAfter, &c.DaysLeft)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// UptimeBar represents a single bar in the uptime timeline.
type UptimeBar struct {
	Label string `json:"label"`
	Up    *bool  `json:"up"` // nil = no data, true = up, false = down
}

// GetUptimeBars returns exactly 30 bars for a probe.
// bucketSeconds controls the granularity (e.g., 120 for 2-min, 2880 for 48-min).
// Each bar is a time slot; nil Up means no data for that slot.
func (db *DB) GetUptimeBars(probeID int64, hours int, bucketSeconds int) ([]UptimeBar, error) {
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

	query := `
		SELECT
			(CAST(strftime('%s', timestamp) AS INTEGER) / ?) * ? as bucket,
			MIN(CASE WHEN success = 1 THEN 1 ELSE 0 END) as status
		FROM probe_results
		WHERE probe_id = ? AND timestamp >= ?
		GROUP BY bucket
		ORDER BY bucket ASC
	`

	rows, err := db.Query(query, bucketSeconds, bucketSeconds, probeID, since.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Build a map of bucket epoch -> status
	data := make(map[int64]bool)
	for rows.Next() {
		var epoch, status int64
		if err := rows.Scan(&epoch, &status); err != nil {
			return nil, err
		}
		data[epoch] = status == 1
	}

	// Generate exactly 30 bars from the start time
	sinceEpoch := since.Unix() / int64(bucketSeconds) * int64(bucketSeconds)
	bars := make([]UptimeBar, 30)
	for i := 0; i < 30; i++ {
		epoch := sinceEpoch + int64(i)*int64(bucketSeconds)
		t := time.Unix(epoch, 0).In(cst)
		var label string
		switch bucketSeconds {
		case 120:
			label = t.Format("15:04")
		case 2880:
			label = t.Format("01/02 15:04")
		case 20160:
			label = t.Format("01/02 15:04")
		}

		if status, ok := data[epoch]; ok {
			bars[i] = UptimeBar{Label: label, Up: &status}
		} else {
			bars[i] = UptimeBar{Label: label, Up: nil}
		}
	}
	return bars, nil
}

// GetUptimePercent returns uptime percentage for a probe within a time window.
func (db *DB) GetUptimePercent(probeID int64, hours int) (float64, error) {
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	var total, success int64
	err := db.QueryRow(
		`SELECT COALESCE(COUNT(*),0), COALESCE(SUM(CASE WHEN success=1 THEN 1 ELSE 0 END),0)
		 FROM probe_results WHERE probe_id=? AND timestamp>=?`,
		probeID, since.UTC().Format("2006-01-02 15:04:05"),
	).Scan(&total, &success)
	if err != nil {
		return 100, err
	}
	if total == 0 {
		return 100, nil
	}
	return float64(success) / float64(total) * 100, nil
}

// ResponseTimePoint is a single data point for the response time chart.
type ResponseTimePoint struct {
	Timestamp  string  `json:"timestamp"`
	LatencyMs  float64 `json:"latency_ms"`
	StatusCode int     `json:"status_code"`
}

// GetResponseTimeTrend returns averaged response time data points for a chart.
// bucketSeconds controls granularity (e.g., 600 for 10-min buckets).
func (db *DB) GetResponseTimeTrend(probeID int64, hours int, bucketSeconds int) ([]ResponseTimePoint, error) {
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

	query := `
		SELECT
			(CAST(strftime('%s', timestamp) AS INTEGER) / ?) * ? as bucket,
			AVG(latency_ms) as avg_latency,
			MAX(CASE WHEN success=1 THEN status_code ELSE 0 END) as status_code
		FROM probe_results
		WHERE probe_id = ? AND timestamp >= ?
		GROUP BY bucket
		ORDER BY bucket ASC
	`

	rows, err := db.Query(query, bucketSeconds, bucketSeconds, probeID, since.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []ResponseTimePoint
	for rows.Next() {
		var epoch int64
		var avgLat float64
		var statusCode int
		if err := rows.Scan(&epoch, &avgLat, &statusCode); err != nil {
			return nil, err
		}
		t := time.Unix(epoch, 0).UTC()
		points = append(points, ResponseTimePoint{
			Timestamp:  t.Format("2006-01-02 15:04:05"),
			LatencyMs:  avgLat,
			StatusCode: statusCode,
		})
	}
	return points, nil
}

// ========== Alert Rules ==========

func (db *DB) CreateAlertRule(r *models.AlertRule) (int64, error) {
	result, err := db.Exec(
		`INSERT INTO alert_rules (name, description, scope_type, scope_id, metric_type, operator, threshold, duration_s, enabled, wechat_webhook, cooldown_s, repeat_enabled, repeat_interval_s)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Name, r.Description, r.ScopeType, r.ScopeID, r.MetricType, r.Operator, r.Threshold, r.DurationS, boolToInt(r.Enabled), r.WechatWebhook, r.CooldownS, boolToInt(r.RepeatEnabled), r.RepeatIntervalS,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) GetAlertRule(id int64) (*models.AlertRule, error) {
	r := &models.AlertRule{}
	var enabled, repeatEnabled int
	err := db.QueryRow(
		`SELECT id, name, description, scope_type, scope_id, metric_type, operator, threshold, duration_s, enabled, wechat_webhook, cooldown_s, repeat_enabled, repeat_interval_s, created_at, updated_at
		 FROM alert_rules WHERE id=?`, id,
	).Scan(&r.ID, &r.Name, &r.Description, &r.ScopeType, &r.ScopeID, &r.MetricType, &r.Operator, &r.Threshold, &r.DurationS, &enabled, &r.WechatWebhook, &r.CooldownS, &repeatEnabled, &r.RepeatIntervalS, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	r.Enabled = intToBool(enabled)
	r.RepeatEnabled = intToBool(repeatEnabled)
	return r, err
}

func (db *DB) GetAllAlertRules() ([]models.AlertRule, error) {
	rows, err := db.Query(
		`SELECT id, name, description, scope_type, scope_id, metric_type, operator, threshold, duration_s, enabled, wechat_webhook, cooldown_s, repeat_enabled, repeat_interval_s, created_at, updated_at
		 FROM alert_rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.AlertRule
	for rows.Next() {
		var r models.AlertRule
		var enabled, repeatEnabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.ScopeType, &r.ScopeID, &r.MetricType, &r.Operator, &r.Threshold, &r.DurationS, &enabled, &r.WechatWebhook, &r.CooldownS, &repeatEnabled, &r.RepeatIntervalS, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Enabled = intToBool(enabled)
		r.RepeatEnabled = intToBool(repeatEnabled)
		rules = append(rules, r)
	}
	return rules, nil
}

func (db *DB) GetEnabledAlertRules() ([]models.AlertRule, error) {
	rows, err := db.Query(
		`SELECT id, name, description, scope_type, scope_id, metric_type, operator, threshold, duration_s, enabled, wechat_webhook, cooldown_s, repeat_enabled, repeat_interval_s, created_at, updated_at
		 FROM alert_rules WHERE enabled=1 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.AlertRule
	for rows.Next() {
		var r models.AlertRule
		var enabled, repeatEnabled int
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.ScopeType, &r.ScopeID, &r.MetricType, &r.Operator, &r.Threshold, &r.DurationS, &enabled, &r.WechatWebhook, &r.CooldownS, &repeatEnabled, &r.RepeatIntervalS, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Enabled = intToBool(enabled)
		r.RepeatEnabled = intToBool(repeatEnabled)
		rules = append(rules, r)
	}
	return rules, nil
}

func (db *DB) UpdateAlertRule(r *models.AlertRule) error {
	_, err := db.Exec(
		`UPDATE alert_rules SET name=?, description=?, scope_type=?, scope_id=?, metric_type=?, operator=?, threshold=?, duration_s=?, enabled=?, wechat_webhook=?, cooldown_s=?, repeat_enabled=?, repeat_interval_s=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=?`,
		r.Name, r.Description, r.ScopeType, r.ScopeID, r.MetricType, r.Operator, r.Threshold, r.DurationS, boolToInt(r.Enabled), r.WechatWebhook, r.CooldownS, boolToInt(r.RepeatEnabled), r.RepeatIntervalS, r.ID,
	)
	return err
}

func (db *DB) DeleteAlertRule(id int64) error {
	_, err := db.Exec(`DELETE FROM alert_rules WHERE id=?`, id)
	return err
}

// ========== Alert Events ==========

func (db *DB) CreateAlertEvent(e *models.AlertEvent) (int64, error) {
	result, err := db.Exec(
		`INSERT INTO alert_events (rule_id, agent_id, probe_id, severity, message, value)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.RuleID, e.AgentID, e.ProbeID, e.Severity, e.Message, e.Value,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) ResolveAlertEvent(id int64) error {
	_, err := db.Exec(`UPDATE alert_events SET resolved_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}

func (db *DB) ResolveAlertByRule(ruleID int64) error {
	_, err := db.Exec(`UPDATE alert_events SET resolved_at=CURRENT_TIMESTAMP WHERE rule_id=? AND resolved_at IS NULL`, ruleID)
	return err
}

func (db *DB) GetAlertEvents(resolved bool, limit int) ([]models.AlertEvent, error) {
	query := `SELECT id, rule_id, agent_id, probe_id, severity, message, value, fired_at, resolved_at
			  FROM alert_events WHERE 1=1`
	args := []interface{}{}

	if !resolved {
		query += ` AND resolved_at IS NULL`
	}
	query += ` ORDER BY fired_at DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.AlertEvent
	for rows.Next() {
		var e models.AlertEvent
		if err := rows.Scan(&e.ID, &e.RuleID, &e.AgentID, &e.ProbeID, &e.Severity, &e.Message, &e.Value, &e.FiredAt, &e.ResolvedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (db *DB) GetLatestAlertEventByRule(ruleID int64) (*models.AlertEvent, error) {
	e := &models.AlertEvent{}
	err := db.QueryRow(
		`SELECT id, rule_id, agent_id, probe_id, severity, message, value, fired_at, resolved_at
		 FROM alert_events WHERE rule_id=? ORDER BY fired_at DESC LIMIT 1`, ruleID,
	).Scan(&e.ID, &e.RuleID, &e.AgentID, &e.ProbeID, &e.Severity, &e.Message, &e.Value, &e.FiredAt, &e.ResolvedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

func (db *DB) DeleteAllAlertEvents() (int64, error) {
	result, err := db.Exec(`DELETE FROM alert_events`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ========== Helpers ==========

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i == 1
}

// GetUnresolvedAlertEvents returns all alert events that have not been resolved.
func (db *DB) GetUnresolvedAlertEvents() ([]models.AlertEvent, error) {
	rows, err := db.Query(
		`SELECT id, rule_id, agent_id, probe_id, severity, message, value, fired_at, resolved_at
		 FROM alert_events WHERE resolved_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.AlertEvent
	for rows.Next() {
		var e models.AlertEvent
		if err := rows.Scan(&e.ID, &e.RuleID, &e.AgentID, &e.ProbeID, &e.Severity, &e.Message, &e.Value, &e.FiredAt, &e.ResolvedAt); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events, nil
}
