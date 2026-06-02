package storage

import "fmt"

// SystemStats holds aggregate system statistics for Homepage widget.
type SystemStats struct {
	OnlineAgents string `json:"online_agents"` // e.g. "3/5"
	OnlineProbes string `json:"online_probes"` // e.g. "3/4"
	ActiveAlerts string `json:"active_alerts"` // e.g. "0"
}

// GetSystemStats returns aggregate statistics for Homepage customapi widget.
func (db *DB) GetSystemStats() (*SystemStats, error) {
	s := &SystemStats{}

	// Agents: total and online
	var totalAgents, onlineAgents int64
	err := db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status='online' THEN 1 ELSE 0 END), 0) FROM agents`,
	).Scan(&totalAgents, &onlineAgents)
	if err != nil {
		return nil, err
	}
	s.OnlineAgents = fmt.Sprintf("%d/%d", onlineAgents, totalAgents)

	// Probes: total and up (based on latest result)
	var totalProbes, probesUp int64
	err = db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN latest.success = 1 THEN 1 ELSE 0 END), 0)
		 FROM web_probes wp
		 LEFT JOIN probe_results latest ON latest.id = (
			 SELECT id FROM probe_results WHERE probe_id = wp.id ORDER BY timestamp DESC LIMIT 1
		 )`,
	).Scan(&totalProbes, &probesUp)
	if err != nil {
		return nil, err
	}
	s.OnlineProbes = fmt.Sprintf("%d/%d", probesUp, totalProbes)

	// Active (unresolved) alerts
	var activeAlerts int64
	err = db.QueryRow(
		`SELECT COUNT(*) FROM alert_events WHERE resolved_at IS NULL`,
	).Scan(&activeAlerts)
	if err != nil {
		return nil, err
	}
	s.ActiveAlerts = fmt.Sprintf("%d", activeAlerts)

	return s, nil
}
