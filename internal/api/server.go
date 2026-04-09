package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/pnm/proxy-node-manager/internal/config"
	"github.com/pnm/proxy-node-manager/internal/db"
	"github.com/pnm/proxy-node-manager/internal/installer"
	"github.com/pnm/proxy-node-manager/internal/proxy"
	"github.com/pnm/proxy-node-manager/internal/user"
)

// Server holds all dependencies for the HTTP API
type Server struct {
	cfg          *config.Config
	db           *db.DB
	userService  *user.Service
	xray         *proxy.XrayManager
	hy2          *proxy.Hy2Manager
	xrayInstaller *installer.XrayInstaller
	hy2Installer  *installer.Hy2Installer
}

// NewServer creates a new API server
func NewServer(
	cfg *config.Config,
	database *db.DB,
	userSvc *user.Service,
	xray *proxy.XrayManager,
	hy2 *proxy.Hy2Manager,
	xrayInst *installer.XrayInstaller,
	hy2Inst *installer.Hy2Installer,
) *Server {
	return &Server{
		cfg:          cfg,
		db:           database,
		userService:  userSvc,
		xray:         xray,
		hy2:          hy2,
		xrayInstaller: xrayInst,
		hy2Installer:  hy2Inst,
	}
}

// Start starts the HTTP API server (blocking)
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// System
	mux.HandleFunc("/api/v1/status", s.handleStatus)

	// Proxy management
	mux.HandleFunc("/api/v1/proxy/install", s.handleProxyInstall)
	mux.HandleFunc("/api/v1/proxy/start", s.handleProxyStart)
	mux.HandleFunc("/api/v1/proxy/stop", s.handleProxyStop)

	// User management
	mux.HandleFunc("/api/v1/users", s.handleUsers)
	mux.HandleFunc("/api/v1/users/", s.handleUserByPath)

	log.Printf("[api] listening on %s", s.cfg.APIListenAddr)
	return http.ListenAndServe(s.cfg.APIListenAddr, mux)
}

// StartAuthEndpoint starts the Hysteria2 HTTP auth endpoint (blocking)
func (s *Server) StartAuthEndpoint() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/hy2/auth", s.handleHy2Auth)

	log.Printf("[auth] hy2 auth endpoint on %s", s.cfg.AuthListenAddr)
	return http.ListenAndServe(s.cfg.AuthListenAddr, mux)
}

// --- Handlers ---

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	users, _ := s.db.ListUsers()
	enabledCount := 0
	for _, u := range users {
		if u.Enabled {
			enabledCount++
		}
	}

	vlessConf, _ := s.db.GetProxyConfig("vless")
	hy2Conf, _ := s.db.GetProxyConfig("hysteria2")
	nodeInfo, _ := s.db.GetNodeInfo()

	status := map[string]interface{}{
		"server_ip":     nodeInfo.ServerIP,
		"total_users":   len(users),
		"enabled_users": enabledCount,
		"vless": map[string]interface{}{
			"installed": vlessConf != nil && vlessConf.Installed,
			"running":   s.xray.IsRunning(),
			"port":      s.cfg.VLESSPort,
		},
		"hysteria2": map[string]interface{}{
			"installed": hy2Conf != nil && hy2Conf.Installed,
			"running":   s.hy2.IsRunning(),
			"port":      s.cfg.Hy2Port,
		},
	}

	jsonResp(w, http.StatusOK, status)
}

func (s *Server) handleProxyInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Protocol string `json:"protocol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch req.Protocol {
	case "vless":
		if err := s.xrayInstaller.Install(); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	case "hysteria2":
		if err := s.hy2Installer.Install(); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	default:
		jsonError(w, http.StatusBadRequest, "unknown protocol: "+req.Protocol)
		return
	}

	jsonResp(w, http.StatusOK, map[string]string{"status": "installed", "protocol": req.Protocol})
}

func (s *Server) handleProxyStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Protocol string `json:"protocol"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var err error
	switch req.Protocol {
	case "vless":
		// Regenerate config with current users before starting
		users, _ := s.db.ListEnabledUsers()
		s.xray.GenerateConfig(users)
		err = s.xray.Start()
	case "hysteria2":
		users, _ := s.db.ListEnabledUsers()
		s.hy2.GenerateConfig(users)
		err = s.hy2.Start()
	default:
		jsonError(w, http.StatusBadRequest, "unknown protocol")
		return
	}

	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) handleProxyStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Protocol string `json:"protocol"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var err error
	switch req.Protocol {
	case "vless":
		err = s.xray.Stop()
	case "hysteria2":
		err = s.hy2.Stop()
	default:
		jsonError(w, http.StatusBadRequest, "unknown protocol")
		return
	}

	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResp(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		users, err := s.userService.ListUsers()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResp(w, http.StatusOK, users)

	case http.MethodPost:
		var req struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid body")
			return
		}
		user, err := s.userService.CreateUser(req.Username)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResp(w, http.StatusCreated, user)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUserByPath(w http.ResponseWriter, r *http.Request) {
	// Extract username from path: /api/v1/users/{username}[/action]
	path := r.URL.Path
	prefix := "/api/v1/users/"
	if len(path) <= len(prefix) {
		jsonError(w, http.StatusBadRequest, "username required")
		return
	}

	remaining := path[len(prefix):]
	parts := splitPath(remaining)
	username := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case action == "" && r.Method == http.MethodGet:
		// GET /api/v1/users/{username}
		user, err := s.userService.GetUser(username)
		if err != nil {
			jsonError(w, http.StatusNotFound, err.Error())
			return
		}
		jsonResp(w, http.StatusOK, user)

	case action == "" && r.Method == http.MethodDelete:
		// DELETE /api/v1/users/{username}
		if err := s.userService.DeleteUser(username); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResp(w, http.StatusOK, map[string]string{"status": "deleted"})

	case action == "enable" && r.Method == http.MethodPost:
		if err := s.userService.EnableUser(username); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResp(w, http.StatusOK, map[string]string{"status": "enabled"})

	case action == "disable" && r.Method == http.MethodPost:
		if err := s.userService.DisableUser(username); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResp(w, http.StatusOK, map[string]string{"status": "disabled"})

	case action == "config" && r.Method == http.MethodGet:
		// GET /api/v1/users/{username}/config — per-user client YAML
		cfg, err := s.userService.GetClientConfig(username)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/yaml")
		w.Write([]byte(cfg))

	case action == "traffic" && r.Method == http.MethodGet:
		logs, err := s.userService.GetTrafficLogs(username, 100)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResp(w, http.StatusOK, logs)

	case action == "reset-traffic" && r.Method == http.MethodPost:
		if err := s.userService.ResetTraffic(username); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonResp(w, http.StatusOK, map[string]string{"status": "traffic_reset"})

	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// handleHy2Auth handles Hysteria2 HTTP auth requests
// Hy2 sends POST with: {"addr": "...", "auth": "password_string", "tx": 0}
func (s *Server) handleHy2Auth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Addr string `json:"addr"`
		Auth string `json:"auth"`
		Tx   int64  `json:"tx"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false})
		return
	}

	// Look up user by hy2_password
	user, err := s.db.FindEnabledUserByHy2Auth(req.Auth)
	if err != nil || user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false})
		return
	}

	// Check traffic limit
	if user.TrafficLimit > 0 && (user.TrafficUp+user.TrafficDown) >= user.TrafficLimit {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false})
		return
	}

	// Auth success — return username as the ID for traffic tracking
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok": true,
		"id": user.Username,
	})
}

// --- Helpers ---

func jsonResp(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	jsonResp(w, status, map[string]string{"error": msg})
}

func splitPath(path string) []string {
	var parts []string
	for _, p := range splitString(path, '/') {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitString(s string, sep byte) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			if i > start {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

// FormatBytes formats bytes into human-readable form
func FormatBytes(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
