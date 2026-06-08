package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
)

type authRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func registerHandler(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only allow registration if no users exist
		has, err := db.HasUsers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "系统错误"})
			return
		}
		if has {
			c.JSON(http.StatusForbidden, gin.H{"error": "已存在用户，不允许重复注册"})
			return
		}

		var req authRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请提供用户名和密码"})
			return
		}

		username := strings.TrimSpace(req.Username)
		if username == "" || len(username) < 2 || len(username) > 32 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "用户名需2-32个字符"})
			return
		}
		for _, ch := range username {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "用户名只能包含字母和数字"})
				return
			}
		}

		if len(req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少6位"})
			return
		}

		user, token, err := db.Register(username, req.Password, c.ClientIP())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败: " + err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":  "注册成功",
			"username": user.Username,
			"token":    token,
		})
	}
}

func loginHandler(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req authRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请提供用户名和密码"})
			return
		}

		user, token, err := db.Login(req.Username, req.Password, c.ClientIP())
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":  "登录成功",
			"username": user.Username,
			"token":    token,
		})
	}
}

func checkHandler(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		has, err := db.HasUsers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "系统错误"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"need_register": !has})
	}
}

// sessionsHandler returns the current user's active sessions.
func sessionsHandler(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := validateTokenFromContext(c, db)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		sessions, err := db.GetSessions(user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取会话列表失败"})
			return
		}
		// Mark the current session
		currentToken := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		type sessionOut struct {
			ID        int64  `json:"id"`
			UserAgent string `json:"user_agent"`
			IPAddress string `json:"ip_address"`
			CreatedAt string `json:"created_at"`
			IsCurrent bool   `json:"is_current"`
		}
		var out []sessionOut
		for _, s := range sessions {
			out = append(out, sessionOut{
				ID:        s.ID,
				UserAgent: s.UserAgent,
				IPAddress: s.IPAddress,
				CreatedAt: s.CreatedAt,
				IsCurrent: s.Token == currentToken,
			})
		}
		c.JSON(http.StatusOK, gin.H{"sessions": out})
	}
}

// deleteSessionHandler deletes a specific session (logout a device).
func deleteSessionHandler(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := validateTokenFromContext(c, db)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		var sessionID int64
		if idStr := c.Param("id"); idStr != "" {
			if _, err := fmt.Sscanf(idStr, "%d", &sessionID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "无效的会话ID"})
				return
			}
		} else {
			var body struct {
				SessionID int64 `json:"session_id"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "请提供会话ID"})
				return
			}
			sessionID = body.SessionID
		}

		// Prevent deleting current session via this endpoint
		currentToken := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if _, sid, err := db.GetTokenUserID(currentToken); err == nil && sid == sessionID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除当前会话，请使用退出登录"})
			return
		}

		if err := db.DeleteSession(sessionID, user.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "删除会话失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "已退出该设备"})
	}
}

// validateTokenFromContext extracts and validates the Bearer token from the request header.
// It returns the User if valid, or an error.
func validateTokenFromContext(c *gin.Context, db *storage.DB) (*storage.User, error) {
	header := c.GetHeader("Authorization")
	if header == "" || !strings.HasPrefix(header, "Bearer ") {
		return nil, errors.New("请先登录")
	}
	token := strings.TrimPrefix(header, "Bearer ")
	return db.ValidateToken(token)
}

// AuthMiddleware validates the Bearer token.
func AuthMiddleware(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
			c.Abort()
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		user, err := db.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
			c.Abort()
			return
		}
		c.Set("user", user)
		c.Next()
	}
}
