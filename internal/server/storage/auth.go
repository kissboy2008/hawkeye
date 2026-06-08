package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// TokenMaxAge is how long a login token remains valid.
const TokenMaxAge = 7 * 24 * time.Hour // 7 days

// User represents a user record.
type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Token        string `json:"-"`
	CreatedAt    string `json:"created_at"`
}

// Session represents an active login session.
type Session struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"user_id"`
	Token     string `json:"-"`
	UserAgent string `json:"user_agent"`
	IPAddress string `json:"ip_address"`
	CreatedAt string `json:"created_at"`
}

// Register creates a new user with hashed password and creates a session.
func (db *DB) Register(username, password, ipAddress string) (*User, string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.Exec(
		"INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)",
		username, string(hash), now,
	)
	if err != nil {
		return nil, "", err
	}

	userID, _ := result.LastInsertId()

	token, err := generateToken()
	if err != nil {
		return nil, "", err
	}

	_, err = db.Exec(
		"INSERT INTO sessions (user_id, token, ip_address, created_at) VALUES (?, ?, ?, ?)",
		userID, token, ipAddress, now,
	)
	if err != nil {
		return nil, "", err
	}

	return &User{ID: userID, Username: username, Token: token}, token, nil
}

// Login validates credentials and creates a new session (does not invalidate existing sessions).
func (db *DB) Login(username, password, ipAddress string) (*User, string, error) {
	var u User
	err := db.QueryRow(
		"SELECT id, username, password_hash FROM users WHERE username = ?", username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash)
	if err != nil {
		return nil, "", errors.New("账户或密码错误")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, "", errors.New("账户或密码错误")
	}

	token, err := generateToken()
	if err != nil {
		return nil, "", err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec("INSERT INTO sessions (user_id, token, ip_address, created_at) VALUES (?, ?, ?, ?)", u.ID, token, ipAddress, now)
	if err != nil {
		return nil, "", err
	}

	u.Token = token
	return &u, token, nil
}

// ValidateToken checks if a token is valid and not expired by looking it up in the sessions table.
func (db *DB) ValidateToken(token string) (*User, error) {
	var u User
	var sessionID int64
	var sessionCreatedAt string
	err := db.QueryRow(
		`SELECT u.id, u.username, s.id, COALESCE(s.created_at, '')
		 FROM users u
		 JOIN sessions s ON s.user_id = u.id
		 WHERE s.token = ?`, token,
	).Scan(&u.ID, &u.Username, &sessionID, &sessionCreatedAt)
	if err != nil {
		return nil, errors.New("invalid token")
	}

	// Check expiration
	if sessionCreatedAt != "" {
		created, err := time.Parse(time.RFC3339, sessionCreatedAt)
		if err == nil && time.Since(created) > TokenMaxAge {
			db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
			return nil, errors.New("token expired")
		}
	}

	// Renew session timestamp on each successful validation (sliding expiration)
	now := time.Now().UTC().Format(time.RFC3339)
	db.Exec("UPDATE sessions SET created_at = ? WHERE id = ?", now, sessionID)

	return &u, nil
}

// GetSessions returns all active sessions for a user.
func (db *DB) GetSessions(userID int64) ([]Session, error) {
	rows, err := db.Query(
		"SELECT id, user_id, token, COALESCE(user_agent, ''), COALESCE(ip_address, ''), COALESCE(created_at, '') FROM sessions WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.UserID, &s.Token, &s.UserAgent, &s.IPAddress, &s.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// DeleteSession deletes a specific session, ensuring it belongs to the given user.
func (db *DB) DeleteSession(sessionID, userID int64) error {
	_, err := db.Exec("DELETE FROM sessions WHERE id = ? AND user_id = ?", sessionID, userID)
	return err
}

// DeleteOtherSessions deletes all sessions for a user except the one with the given token.
func (db *DB) DeleteOtherSessions(userID int64, keepToken string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE user_id = ? AND token != ?", userID, keepToken)
	return err
}

// HasUsers returns true if at least one user exists.
func (db *DB) HasUsers() (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count > 0, err
}

// CountUsers returns the number of registered users.
func (db *DB) CountUsers() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// GetTokenUserID returns the user ID and session ID for a given token (or error).
func (db *DB) GetTokenUserID(token string) (userID int64, sessionID int64, err error) {
	err = db.QueryRow(
		"SELECT user_id, id FROM sessions WHERE token = ?", token,
	).Scan(&userID, &sessionID)
	return
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
