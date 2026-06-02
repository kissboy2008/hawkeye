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

// Register creates a new user with hashed password and returns a token.
func (db *DB) Register(username, password string) (*User, string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}

	token, err := generateToken()
	if err != nil {
		return nil, "", err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.Exec(
		"INSERT INTO users (username, password_hash, token, token_created_at) VALUES (?, ?, ?, ?)",
		username, string(hash), token, now,
	)
	if err != nil {
		return nil, "", err
	}

	id, _ := result.LastInsertId()
	return &User{ID: id, Username: username, Token: token}, token, nil
}

// Login validates credentials and returns a new token.
func (db *DB) Login(username, password string) (*User, string, error) {
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
	_, err = db.Exec("UPDATE users SET token = ?, token_created_at = ? WHERE id = ?", token, now, u.ID)
	if err != nil {
		return nil, "", err
	}

	u.Token = token
	return &u, token, nil
}

// ValidateToken checks if a token is valid and not expired.
func (db *DB) ValidateToken(token string) (*User, error) {
	var u User
	var tokenCreatedAt string
	err := db.QueryRow(
		"SELECT id, username, COALESCE(token_created_at, '') FROM users WHERE token = ? AND token != ''", token,
	).Scan(&u.ID, &u.Username, &tokenCreatedAt)
	if err != nil {
		return nil, errors.New("invalid token")
	}

	// Check expiration
	if tokenCreatedAt != "" {
		created, err := time.Parse(time.RFC3339, tokenCreatedAt)
		if err == nil && time.Since(created) > TokenMaxAge {
			// Token expired — clear it
			db.Exec("UPDATE users SET token = '' WHERE id = ?", u.ID)
			return nil, errors.New("token expired")
		}
	}

	return &u, nil
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
