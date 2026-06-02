package storage

import (
	"database/sql"
	"time"
)

// Widget represents a dashboard widget configuration.
type Widget struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // proxmox, pbs, unraid
	URL         string `json:"url"`
	APIToken    string `json:"api_token,omitempty"`
	Node        string `json:"node,omitempty"`
	Config      string `json:"config,omitempty"`
	Description string `json:"description,omitempty"`
	WidgetGroup string `json:"widget_group"`
	SortOrder   int    `json:"sort_order"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// ListWidgets returns all widgets ordered by sort_order.
func (db *DB) ListWidgets() ([]Widget, error) {
	rows, err := db.Query(`SELECT id, name, type, url, api_token, node, config, description, sort_order, enabled, created_at, updated_at, COALESCE(widget_group, '') FROM widgets ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var widgets []Widget
	for rows.Next() {
		var w Widget
		var enabled int
		if err := rows.Scan(&w.ID, &w.Name, &w.Type, &w.URL, &w.APIToken, &w.Node, &w.Config, &w.Description, &w.SortOrder, &enabled, &w.CreatedAt, &w.UpdatedAt, &w.WidgetGroup); err != nil {
			return nil, err
		}
		w.Enabled = enabled == 1
		widgets = append(widgets, w)
	}
	return widgets, nil
}

// GetWidget returns a single widget by ID.
func (db *DB) GetWidget(id int64) (*Widget, error) {
	var w Widget
	var enabled int
	err := db.QueryRow(`SELECT id, name, type, url, api_token, node, config, description, sort_order, enabled, created_at, updated_at, COALESCE(widget_group, '') FROM widgets WHERE id = ?`, id).
		Scan(&w.ID, &w.Name, &w.Type, &w.URL, &w.APIToken, &w.Node, &w.Config, &w.Description, &w.SortOrder, &enabled, &w.CreatedAt, &w.UpdatedAt, &w.WidgetGroup)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	w.Enabled = enabled == 1
	return &w, nil
}

// CreateWidget inserts a new widget. sort_order is auto-assigned to the end.
func (db *DB) CreateWidget(w *Widget) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	result, err := db.Exec(`INSERT INTO widgets (name, type, url, api_token, node, config, description, widget_group, sort_order, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order),0)+1 FROM widgets), ?, ?, ?)`,
		w.Name, w.Type, w.URL, w.APIToken, w.Node, w.Config, w.Description, w.WidgetGroup, boolToInt(w.Enabled), now, now)
	if err != nil {
		return err
	}
	w.ID, _ = result.LastInsertId()
	w.CreatedAt = now
	w.UpdatedAt = now
	return nil
}

// UpdateWidget updates an existing widget. sort_order is preserved (not changed by edit).
func (db *DB) UpdateWidget(w *Widget) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`UPDATE widgets SET name=?, type=?, url=?, api_token=?, node=?, config=?, description=?, widget_group=?, enabled=?, updated_at=? WHERE id=?`,
		w.Name, w.Type, w.URL, w.APIToken, w.Node, w.Config, w.Description, w.WidgetGroup, boolToInt(w.Enabled), now, w.ID)
	if err != nil {
		return err
	}
	w.UpdatedAt = now
	return nil
}

// DeleteWidget removes a widget by ID.
func (db *DB) DeleteWidget(id int64) error {
	_, err := db.Exec(`DELETE FROM widgets WHERE id = ?`, id)
	return err
}

// ReorderWidgets updates sort_order for widgets based on the given ID order.
func (db *DB) ReorderWidgets(ids []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE widgets SET sort_order = ? WHERE id = ?`, i+1, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpdateWidgetGroup updates the widget_group for a single widget.
func (db *DB) UpdateWidgetGroup(id int64, group string) error {
	_, err := db.Exec(`UPDATE widgets SET widget_group = ?, updated_at = ? WHERE id = ?`, group, time.Now().Format("2006-01-02 15:04:05"), id)
	return err
}

// GroupExists checks whether a widget_group name already exists in the table.
func (db *DB) GroupExists(name string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM widgets WHERE widget_group = ?`, name).Scan(&count)
	return count > 0, err
}

// RenameWidgetGroup renames a group across all widgets that belong to it.
func (db *DB) RenameWidgetGroup(oldName, newName string) error {
	_, err := db.Exec(`UPDATE widgets SET widget_group = ?, updated_at = ? WHERE widget_group = ?`, newName, time.Now().Format("2006-01-02 15:04:05"), oldName)
	return err
}
