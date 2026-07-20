package agent

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

// Handler serves the agent HTTP API.
type Handler struct {
	collector  *Collector
	authToken  string
	configPath string
}

func NewHandler(c *Collector, authToken string, configPath string) *Handler {
	return &Handler{collector: c, authToken: authToken, configPath: configPath}
}

// RegisterRoutes registers agent API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Own API
	mux.HandleFunc("/api/v1/metrics", h.auth(h.handleMetrics))
	mux.HandleFunc("/api/v1/info", h.auth(h.handleInfo))
	mux.HandleFunc("/api/v1/homepage", h.auth(h.handleHomepage))
	mux.HandleFunc("/api/v1/config", h.auth(h.handleConfig))
	mux.HandleFunc("/health", h.handleHealth)


}

// auth is middleware that checks Bearer token if configured.
func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.authToken == "" {
			next(w, r)
			return
		}
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		if token != h.authToken {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	metrics, err := h.collector.CollectMetrics()
	if err != nil {
		log.Printf("[agent] error collecting metrics: %v", err)
		http.Error(w, `{"error":"failed to collect metrics"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (h *Handler) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	info, err := h.collector.CollectInfo()
	if err != nil {
		log.Printf("[agent] error collecting info: %v", err)
		http.Error(w, `{"error":"failed to collect info"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func (h *Handler) handleHomepage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	data, err := h.collector.CollectHomepage()
	if err != nil {
		log.Printf("[agent] error collecting homepage data: %v", err)
		http.Error(w, `{"error":"failed to collect homepage data"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// configRequest is the body for PUT /api/v1/config.
type configRequest struct {
	ServerURL string `json:"server_url"`
	AuthToken string `json:"auth_token"`
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req configRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if req.ServerURL == "" && req.AuthToken == "" {
		http.Error(w, `{"error":"server_url or auth_token required"}`, http.StatusBadRequest)
		return
	}

	// Read existing config
	data, err := os.ReadFile(h.configPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("[agent] config: failed to read %s: %v", h.configPath, err)
		http.Error(w, `{"error":"failed to read config"}`, http.StatusInternalServerError)
		return
	}

	// Parse or create empty map
	var cfg map[string]interface{}
	if data != nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			cfg = make(map[string]interface{})
		}
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	updatedFields := []string{}

	// Set server.url if provided
	if req.ServerURL != "" {
		serverMap, ok := cfg["server"].(map[string]interface{})
		if !ok {
			serverMap = make(map[string]interface{})
			cfg["server"] = serverMap
		}
		serverMap["url"] = req.ServerURL
		updatedFields = append(updatedFields, "server.url="+req.ServerURL)
	}

	// Set auth.token if provided
	if req.AuthToken != "" {
		authMap, ok := cfg["auth"].(map[string]interface{})
		if !ok {
			authMap = make(map[string]interface{})
			cfg["auth"] = authMap
		}
		authMap["token"] = req.AuthToken
		updatedFields = append(updatedFields, "auth.token=***")
	}

	// Write back
	out, err := yaml.Marshal(cfg)
	if err != nil {
		log.Printf("[agent] config: failed to marshal: %v", err)
		http.Error(w, `{"error":"failed to marshal config"}`, http.StatusInternalServerError)
		return
	}

	// Ensure directory exists
	if err := os.MkdirAll(strings.Replace(h.configPath, "/agent.yaml", "", 1), 0755); err != nil {
		// try writing anyway
	}

	if err := os.WriteFile(h.configPath, out, 0644); err != nil {
		log.Printf("[agent] config: failed to write %s: %v", h.configPath, err)
		http.Error(w, `{"error":"failed to write config"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("[agent] config: updated %v in %s", updatedFields, h.configPath)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "updated": strings.Join(updatedFields, ", ")})

	// Self-restart to pick up the new config (uses exec so PID stays same)
	go func() {
		time.Sleep(500 * time.Millisecond)
		log.Println("[agent] restarting to apply new config...")
		exe, err := os.Executable()
		if err != nil {
			log.Printf("[agent] cannot find self: %v", err)
			os.Exit(1)
		}
		if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
			log.Printf("[agent] exec failed: %v", err)
			os.Exit(1)
		}
	}()
}




// writeJSON is a helper to write JSON responses.
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
