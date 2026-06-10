package storage

import (
	"database/sql"
	"fmt"
	"time"

	"hawkeye/internal/models"
)

// ========== Agent CRUD ==========

func (db *DB) CreateAgent(a *models.Agent) (int64, error) {
	if a.Mode == "" {
		a.Mode = "push"
	}
	result, err := db.Exec(
		`INSERT INTO agents (name, address, server_url, auth_token, tags, mode, status, intranet_url, extranet_url, sort_order)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order),0)+1 FROM agents))`,
		a.Name, a.Address, a.ServerURL, a.AuthToken, a.Tags, a.Mode, a.Status, a.IntranetURL, a.ExtranetURL,
	)
	if err != nil {
		return 0, err
	}
	id, _ := result.LastInsertId()
	return id, nil
}

func (db *DB) GetAgent(id int64) (*models.Agent, error) {
	a := &models.Agent{}
	err := db.QueryRow(
		`SELECT id, name, address, server_url, auth_token, tags, mode, status, agent_version, intranet_url, extranet_url, sort_order, last_seen, created_at, updated_at
		 FROM agents WHERE id = ?`, id,
	).Scan(&a.ID, &a.Name, &a.Address, &a.ServerURL, &a.AuthToken, &a.Tags, &a.Mode, &a.Status, &a.AgentVersion, &a.IntranetURL, &a.ExtranetURL, &a.SortOrder, &a.LastSeen, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (db *DB) GetAllAgents() ([]models.Agent, error) {
	rows, err := db.Query(
		`SELECT id, name, address, server_url, auth_token, tags, mode, status, agent_version, intranet_url, extranet_url, sort_order, last_seen, created_at, updated_at
		 FROM agents ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.Address, &a.ServerURL, &a.AuthToken, &a.Tags, &a.Mode, &a.Status, &a.AgentVersion, &a.IntranetURL, &a.ExtranetURL, &a.SortOrder, &a.LastSeen, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func (db *DB) GetAgentByToken(token string) (*models.Agent, error) {
	a := &models.Agent{}
	err := db.QueryRow(
		`SELECT id, name, address, server_url, auth_token, tags, mode, status, agent_version, intranet_url, extranet_url, sort_order, last_seen, created_at, updated_at
		 FROM agents WHERE auth_token = ?`, token,
	).Scan(&a.ID, &a.Name, &a.Address, &a.ServerURL, &a.AuthToken, &a.Tags, &a.Mode, &a.Status, &a.AgentVersion, &a.IntranetURL, &a.ExtranetURL, &a.SortOrder, &a.LastSeen, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (db *DB) UpdateAgent(a *models.Agent) error {
	if a.Mode == "" {
		a.Mode = "push"
	}
	_, err := db.Exec(
		`UPDATE agents SET name=?, address=?, server_url=?, auth_token=?, tags=?, mode=?, status=?, intranet_url=?, extranet_url=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=?`,
		a.Name, a.Address, a.ServerURL, a.AuthToken, a.Tags, a.Mode, a.Status, a.IntranetURL, a.ExtranetURL, a.ID,
	)
	return err
}

func (db *DB) DeleteAgent(id int64) error {
	_, err := db.Exec(`DELETE FROM agents WHERE id=?`, id)
	return err
}

func (db *DB) UpdateAgentStatus(id int64, status string) error {
	if status == "online" {
		_, err := db.Exec(
			`UPDATE agents SET status=?, last_seen=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			status, id,
		)
		return err
	}
	_, err := db.Exec(
		`UPDATE agents SET status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		status, id,
	)
	return err
}

func (db *DB) GetAgentsByStatus(status string) ([]models.Agent, error) {
	rows, err := db.Query(
		`SELECT id, name, address, server_url, auth_token, tags, mode, status, agent_version, intranet_url, extranet_url, sort_order, last_seen, created_at, updated_at
		 FROM agents WHERE status=? ORDER BY sort_order ASC, id ASC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.Address, &a.ServerURL, &a.AuthToken, &a.Tags, &a.Mode, &a.Status, &a.AgentVersion, &a.IntranetURL, &a.ExtranetURL, &a.SortOrder, &a.LastSeen, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func (db *DB) UpdateAgentVersion(id int64, version string) error {
	_, err := db.Exec(
		`UPDATE agents SET agent_version=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		version, id,
	)
	return err
}

// ReorderAgents updates sort_order for all given agent IDs in order.
func (db *DB) ReorderAgents(ids []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE agents SET sort_order=? WHERE id=?`, i+1, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ========== Settings ==========

func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=?`, key, value, value)
	return err
}

func (db *DB) GetAllSettings() ([]models.Setting, error) {
	rows, err := db.Query(`SELECT key, value FROM settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []models.Setting
	for rows.Next() {
		var s models.Setting
		if err := rows.Scan(&s.Key, &s.Value); err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, nil
}

// ========== Metrics ==========

func (db *DB) InsertMetric(agentID int64, metricType string, data string) error {
	_, err := db.Exec(
		`INSERT INTO metrics (agent_id, metric_type, data) VALUES (?, ?, ?)`,
		agentID, metricType, data,
	)
	return err
}

func (db *DB) GetLatestMetric(agentID int64, metricType string) (string, error) {
	var data string
	err := db.QueryRow(
		`SELECT data FROM metrics WHERE agent_id=? AND metric_type=? ORDER BY timestamp DESC LIMIT 1`,
		agentID, metricType,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return data, err
}

func (db *DB) GetMetricsTimeRange(agentID int64, metricType string, from, to time.Time) ([]models.MetricPointResult, error) {
	rows, err := db.Query(
		`SELECT timestamp, data FROM metrics
		 WHERE agent_id=? AND metric_type=? AND timestamp BETWEEN ? AND ?
		 ORDER BY timestamp`,
		agentID, metricType, from.UTC().Format("2006-01-02 15:04:05"), to.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.MetricPointResult
	for rows.Next() {
		var ts time.Time
		var data string
		if err := rows.Scan(&ts, &data); err != nil {
			return nil, err
		}
		value := extractMetricValueFromJSON(data)
		points = append(points, models.MetricPointResult{
			Timestamp: ts,
			Value:     value,
			Labels:    data,
		})
	}
	return points, nil
}

// extractMetricValueFromJSON extracts usage_percent from metric JSON.
func extractMetricValueFromJSON(jsonStr string) float64 {
	return models.ExtractMetricValue(jsonStr, "usage_percent")
}

func (db *DB) GetHourlyMetrics(agentID int64, metricType string, from, to time.Time) ([]models.MetricPointResult, error) {
	rows, err := db.Query(
		`SELECT hour_start, avg_value, max_value, min_value FROM metrics_hourly
		 WHERE agent_id=? AND metric_type=? AND hour_start BETWEEN ? AND ?
		 ORDER BY hour_start`,
		agentID, metricType, from.UTC().Format("2006-01-02 15:04:05"), to.UTC().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []models.MetricPointResult
	for rows.Next() {
		var ts time.Time
		var avg, max, min float64
		if err := rows.Scan(&ts, &avg, &max, &min); err != nil {
			return nil, err
		}
		// Store avg as the main value, put max/min in labels as JSON
		labels := fmt.Sprintf(`{"avg":%.2f,"max":%.2f,"min":%.2f}`, avg, max, min)
		points = append(points, models.MetricPointResult{
			Timestamp: ts,
			Value:     avg,
			Labels:    labels,
		})
	}
	return points, nil
}

// ========== Data Retention ==========

func (db *DB) CleanOldMetrics(retentionDays int) (int64, error) {
	result, err := db.Exec(
		`DELETE FROM metrics WHERE timestamp < datetime('now', ? || ' days')`,
		-retentionDays,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (db *DB) CleanOldHourlyMetrics(retentionDays int) (int64, error) {
	result, err := db.Exec(
		`DELETE FROM metrics_hourly WHERE hour_start < datetime('now', ? || ' days')`,
		-retentionDays,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (db *DB) DeleteAgentMetrics(agentID int64) (int64, error) {
	r1, err := db.Exec(`DELETE FROM metrics WHERE agent_id = ?`, agentID)
	if err != nil {
		return 0, err
	}
	n1, _ := r1.RowsAffected()
	r2, err := db.Exec(`DELETE FROM metrics_hourly WHERE agent_id = ?`, agentID)
	if err != nil {
		return n1, err
	}
	n2, _ := r2.RowsAffected()
	return n1 + n2, nil
}

func (db *DB) AggregateHourlyMetrics() (int64, error) {
	// Determine the start of the aggregation window.
	// Only aggregate metrics from the last tracked timestamp onward to avoid rescanning all data.
	from, _ := db.GetSetting("last_aggregated_at")
	if from == "" {
		from = "1970-01-01 00:00:00"
	}

	// INSERT OR IGNORE is safe because of the unique index on (agent_id, metric_type, hour_start).
	result, err := db.Exec(`
		INSERT OR IGNORE INTO metrics_hourly (agent_id, metric_type, hour_start, avg_value, max_value, min_value)
		SELECT m.agent_id, m.metric_type,
			strftime('%Y-%m-%d %H:00:00', m.timestamp) as hour,
			AVG(json_extract(m.data, '$.usage_percent')),
			MAX(json_extract(m.data, '$.usage_percent')),
			MIN(json_extract(m.data, '$.usage_percent'))
		FROM metrics m
		WHERE m.metric_type IN ('cpu', 'memory')
		  AND m.timestamp >= ?
		  AND m.timestamp < datetime('now', '-1 hour')
		GROUP BY m.agent_id, m.metric_type, strftime('%Y-%m-%d %H', m.timestamp)
	`, from)
	if err != nil {
		return 0, err
	}

	n, _ := result.RowsAffected()

	// Advance the cursor so the next run only scans new metrics.
	// Store the upper bound of the window we just processed as a computed timestamp.
	if _, err := db.Exec(
		`INSERT INTO settings (key, value) VALUES ('last_aggregated_at', datetime('now', '-1 hour'))
		 ON CONFLICT(key) DO UPDATE SET value = datetime('now', '-1 hour')`,
	); err != nil {
		return n, err
	}

	return n, nil
}
