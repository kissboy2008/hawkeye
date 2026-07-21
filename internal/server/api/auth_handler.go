package api

import (
	"errors"
	"log"
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
			masked := token
			if len(token) > 8 {
				masked = token[:4] + "..." + token[len(token)-4:]
			}
			log.Printf("[auth] REST rejected: %v (token=%s)", err, masked)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已过期，请重新登录"})
			c.Abort()
			return
		}
		c.Set("user", user)
		c.Next()
	}
}
