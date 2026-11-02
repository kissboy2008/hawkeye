package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// TokenMaxAge is how long a login token remains valid without activity.
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
// On each successful validation, the session timestamp is renewed (sliding expiration).
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
			db.retryExec("DELETE FROM sessions WHERE id = ?", sessionID)
			return nil, errors.New("token expired")
		}
	}

	// 滑动过期：只在剩余有效期已用掉一半时才续期。
	// 之前每次校验都 UPDATE，等于把只读的鉴权路径变成写路径（每个 API 请求 + 每次 WS 鉴权
	// 都要抢一次 SQLite 写锁），是 SQLITE_BUSY 的主要来源之一。
	// 现在每个会话最多 12 小时写一次，行为对用户完全一致。
	if created, perr := time.Parse(time.RFC3339, sessionCreatedAt); perr == nil {
		if time.Since(created) > TokenMaxAge/2 {
			now := time.Now().UTC().Format(time.RFC3339)
			if _, err := db.retryExec("UPDATE sessions SET created_at = ? WHERE id = ?", now, sessionID); err != nil {
				log.Printf("[auth] renew session %d failed: %v", sessionID, err)
			}
		}
	} else {
		// created_at 不是 RFC3339（历史数据）→ 补一次规范化写入，之后走正常路径
		now := time.Now().UTC().Format(time.RFC3339)
		db.retryExec("UPDATE sessions SET created_at = ? WHERE id = ?", now, sessionID)
	}

	return &u, nil
}

// DeleteOtherSessions deletes all sessions for a user except the one with the given token.
func (db *DB) DeleteOtherSessions(userID int64, keepToken string) error {
	_, err := db.retryExec("DELETE FROM sessions WHERE user_id = ? AND token != ?", userID, keepToken)
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

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
