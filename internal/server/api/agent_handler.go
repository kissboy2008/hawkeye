package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"hawkeye/internal/models"
	"hawkeye/internal/server/poller"
	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
)

// pushLogged tracks which agents have already logged their first successful push.
var pushLogged sync.Map

var agentHTTPClient = &http.Client{Timeout: 10 * time.Second}

func listAgents(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		agents, err := db.GetAllAgents()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if agents == nil {
			agents = []models.Agent{}
		}
		c.JSON(http.StatusOK, agents)
	}
}

func createAgent(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var agent models.Agent
		if err := c.ShouldBindJSON(&agent); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		agent.Status = "unknown"
		id, err := db.CreateAgent(&agent)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		agent.ID = id
		c.JSON(http.StatusCreated, agent)
	}
}

func getAgent(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		agent, err := db.GetAgent(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if agent == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusOK, agent)
	}
}

func updateAgent(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var agent models.Agent
		if err := c.ShouldBindJSON(&agent); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		agent.ID = id
		if err := db.UpdateAgent(&agent); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, agent)
	}
}

func deleteAgent(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		if err := db.DeleteAgent(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}

func testAgent(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		agent, err := db.GetAgent(id)
		if agent == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Test connectivity
		resp, err := agentHTTPClient.Get(agent.Address + "/health")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
			return
		}
		defer resp.Body.Close()

		c.JSON(http.StatusOK, gin.H{
			"success":     resp.StatusCode == http.StatusOK,
			"status_code": resp.StatusCode,
		})
	}
}

func deleteAgentMetrics(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		n, err := db.DeleteAgentMetrics(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted", "count": n})
	}
}

// reorderAgentsRequest is the body for PUT /api/v1/agents/reorder
type reorderAgentsRequest struct {
	IDs []int64 `json:"ids"`
}

// pushServerURLRequest is the body for PUT /api/v1/agents/:id/server-url
type pushServerURLRequest struct {
	ServerURL string `json:"server_url"`
}

// pushAuthTokenRequest is the body for PUT /api/v1/agents/:id/auth-token
type pushAuthTokenRequest struct {
	AuthToken string `json:"auth_token"`
	OldToken  string `json:"old_token"` // optional: old token for auth (frontend provides it since DB may already be updated)
}

// pushServerURL saves server_url to DB and pushes it to the agent via its /api/v1/config endpoint.
func pushServerURL(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var req pushServerURLRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.ServerURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "server_url required"})
			return
		}

		agent, err := db.GetAgent(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if agent == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}

		// 1. Save to DB
		agent.ServerURL = req.ServerURL
		if err := db.UpdateAgent(agent); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save: " + err.Error()})
			return
		}

		// 2. Push to agent
		body, _ := json.Marshal(map[string]string{"server_url": req.ServerURL})
		pushBody := bytes.NewReader(body)
		agentConfigURL := strings.TrimRight(agent.Address, "/") + "/api/v1/config"

		httpReq, err := http.NewRequest(http.MethodPut, agentConfigURL, pushBody)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"status":    "partial",
				"message":   "已保存到数据库，但构建请求到 agent 失败: " + err.Error(),
				"db_saved":  true,
				"agent_url": agentConfigURL,
			})
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if agent.AuthToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+agent.AuthToken)
		}

		resp, err := agentHTTPClient.Do(httpReq)
		if err != nil {
			log.Printf("[api] push server_url to agent %s failed: %v", agent.Name, err)
			c.JSON(http.StatusOK, gin.H{
				"status":    "partial",
				"message":   "已保存到数据库，但推送到 agent 失败: " + err.Error(),
				"db_saved":  true,
				"agent_url": agentConfigURL,
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[api] pushed server_url=%s to agent %s (%s) — OK", req.ServerURL, agent.Name, agent.Address)
			c.JSON(http.StatusOK, gin.H{
				"status":    "ok",
				"message":   "已保存并在 agent 本地写入 server 地址，agent 将开始推送",
				"db_saved":  true,
				"agent_url": agentConfigURL,
			})
		} else {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("[api] push server_url to agent %s rejected: status %d: %s", agent.Name, resp.StatusCode, string(body))
			c.JSON(http.StatusOK, gin.H{
				"status":           "partial",
				"message":          fmt.Sprintf("已保存到数据库，但 agent 返回状态 %d: %s", resp.StatusCode, string(body)),
				"db_saved":         true,
				"agent_url":        agentConfigURL,
				"agent_statuscode": resp.StatusCode,
			})
		}
	}
}

// pushAuthToken saves auth_token to DB and pushes it to the agent via its /api/v1/config endpoint.
// Uses the OLD token for authentication (since the agent still has the old one in memory).
func pushAuthToken(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var req pushAuthTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.AuthToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "auth_token required"})
			return
		}

		// Get CURRENT agent info from DB (before update) — we need the OLD token
		agent, err := db.GetAgent(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if agent == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}

		// Determine the old token: prefer client-provided old_token (since DB may already
		// be updated by updateAgent), fall back to DB value.
		oldToken := req.OldToken
		if oldToken == "" {
			oldToken = agent.AuthToken
		}

		// 1. Save new token to DB
		agent.AuthToken = req.AuthToken
		if err := db.UpdateAgent(agent); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save: " + err.Error()})
			return
		}

		// 2. Push to agent (with OLD token for auth, new token in body)
		body, _ := json.Marshal(map[string]string{"auth_token": req.AuthToken})
		pushBody := bytes.NewReader(body)
		agentConfigURL := strings.TrimRight(agent.Address, "/") + "/api/v1/config"

		httpReq, err := http.NewRequest(http.MethodPut, agentConfigURL, pushBody)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"status":   "partial",
				"message":  "已保存到数据库，但构建请求到 agent 失败: " + err.Error(),
				"db_saved": true,
			})
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		// Use OLD token — the agent still expects the old one
		if oldToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+oldToken)
		}

		resp, err := agentHTTPClient.Do(httpReq)
		if err != nil {
			log.Printf("[api] push auth_token to agent %s failed: %v", agent.Name, err)
			c.JSON(http.StatusOK, gin.H{
				"status":   "partial",
				"message":  "已保存到数据库，但推送到 agent 失败: " + err.Error(),
				"db_saved": true,
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[api] pushed auth_token to agent %s (%s) — OK, agent will restart", agent.Name, agent.Address)
			c.JSON(http.StatusOK, gin.H{
				"status":   "ok",
				"message":  "已保存并在 agent 本地写入新 token，agent 正在重启",
				"db_saved": true,
			})
		} else {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("[api] push auth_token to agent %s rejected: status %d: %s", agent.Name, resp.StatusCode, string(body))
			// If push failed (e.g. old token mismatch), revert DB to old token
			agent.AuthToken = oldToken
			_ = db.UpdateAgent(agent)
			c.JSON(http.StatusOK, gin.H{
				"status":           "failed",
				"message":          "推送失败（旧 token 可能不匹配），已回滚数据库。状态 " + fmt.Sprintf("%d", resp.StatusCode),
				"db_saved":         false,
				"agent_statuscode": resp.StatusCode,
			})
		}
	}
}

func reorderAgents(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req reorderAgentsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(req.IDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ids required"})
			return
		}
		if err := db.ReorderAgents(req.IDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "reordered"})
	}
}

// PushMetricsRequest is the body for POST /api/v1/agents/push
type PushMetricsRequest struct {
	Token     string               `json:"token"`
	Hostname  string               `json:"hostname"`
	Version   string               `json:"version"`
	Timestamp time.Time            `json:"timestamp"`
	CPU       models.CpuMetrics    `json:"cpu"`
	Memory    models.MemoryMetrics `json:"memory"`
	UptimeS   uint64               `json:"uptime_seconds"`
}

// pushMetrics handles agent push. Agents use their own auth_token to identify themselves.
func pushMetrics(db *storage.DB, onMetrics func(int64, *models.AgentMetricsResponse)) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req PushMetricsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		if req.Token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
			return
		}

		// Look up agent by token
		agent, err := db.GetAgentByToken(req.Token)
		if err != nil {
			log.Printf("[push] db error looking up token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
			return
		}
		if agent == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		// Mark online (updates last_seen)
		if err := db.UpdateAgentStatus(agent.ID, "online"); err != nil {
			log.Printf("[push] error updating status for agent %d: %v", agent.ID, err)
		}

		// Update agent version if changed
		if req.Version != "" {
			if err := db.UpdateAgentVersion(agent.ID, req.Version); err != nil {
				log.Printf("[push] error updating version for agent %d: %v", agent.ID, err)
			}
		}

		// Build AgentMetricsResponse from the push data
		metrics := &models.AgentMetricsResponse{
			Hostname:  req.Hostname,
			Timestamp: req.Timestamp,
			CPU:       req.CPU,
			Memory:    req.Memory,
			UptimeS:   req.UptimeS,
		}

		// Store metrics (reuse the same logic as poller)
		poller.StoreMetrics(db, agent.ID, metrics)

		// Notify WebSocket clients
		if onMetrics != nil {
			onMetrics(agent.ID, metrics)
		}

		// Log only first successful push per agent after startup
		if _, ok := pushLogged.Load(agent.ID); !ok {
			log.Printf("[push] received metrics from %s (agent %d)", agent.Name, agent.ID)
			pushLogged.Store(agent.ID, true)
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
