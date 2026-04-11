package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/pnm/proxy-node-manager/internal/config"
	"github.com/pnm/proxy-node-manager/internal/db"
	"github.com/pnm/proxy-node-manager/internal/installer"
	"github.com/pnm/proxy-node-manager/internal/proxy"
	"github.com/pnm/proxy-node-manager/internal/user"
)

type Server struct {
	cfg          *config.Config
	db           *db.DB
	userSvc      *user.Service
	xray         *proxy.XrayManager
	hy2          *proxy.Hy2Manager
	xrayInstaller *installer.XrayInstaller
	hy2Installer *installer.Hy2Installer
}

func NewServer(cfg *config.Config, db *db.DB, userSvc *user.Service, xray *proxy.XrayManager, hy2 *proxy.Hy2Manager, xi *installer.XrayInstaller, hi *installer.Hy2Installer) *Server {
	return &Server{
		cfg:          cfg,
		db:           db,
		userSvc:      userSvc,
		xray:         xray,
		hy2:          hy2,
		xrayInstaller: xi,
		hy2Installer: hi,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/proxy/install", s.handleProxyInstall)
	mux.HandleFunc("/api/v1/proxy/start", s.handleProxyStart)
	mux.HandleFunc("/api/v1/proxy/stop", s.handleProxyStop)
	mux.HandleFunc("/api/v1/users", s.handleUsers)
	mux.HandleFunc("/hy2/auth", s.handleHy2Auth)
	addr := ":9090"
	log.Printf("[INFO] API server starting on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) StartAuthEndpoint() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/hy2/auth", s.handleHy2Auth)
	addr := ":19876"
	log.Printf("[INFO] Auth endpoint starting on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"version": "1.0.0",
		"time":    time.Now().Format(time.RFC3339),
	}
	jsonResp(w, http.StatusOK, status)
}

func (s *Server) handleProxyInstall(w http.ResponseWriter, r *http.Request) {
	var req struct { Protocol string  }
	json.NewDecoder(r.Body).Decode(&req)
	var err error
	if req.Protocol == "vless" { err = s.xrayInstaller.Install() } else { err = s.hy2Installer.Install() }
	if err != nil { jsonError(w, 500, err.Error()); return }
	jsonResp(w, 200, map[string]string{"status": "installed"})
}

func (s *Server) handleProxyStart(w http.ResponseWriter, r *http.Request) {
	var req struct { Protocol string  }
	json.NewDecoder(r.Body).Decode(&req)
	var err error
	if req.Protocol == "vless" { 
		u, _ := s.db.ListEnabledUsers()
		s.xray.GenerateConfig(u)
		err = s.xray.Start() 
	} else { 
		u, _ := s.db.ListEnabledUsers()
		s.hy2.GenerateConfig(u)
		err = s.hy2.Start() 
	}
	if err != nil { jsonError(w, 500, err.Error()); return }
	jsonResp(w, 200, map[string]string{"status": "started"})
}

func (s *Server) handleProxyStop(w http.ResponseWriter, r *http.Request) {
	var req struct { Protocol string  }
	json.NewDecoder(r.Body).Decode(&req)
	var err error
	if req.Protocol == "vless" { err = s.xray.Stop() } else { err = s.hy2.Stop() }
	if err != nil { jsonError(w, 500, err.Error()); return }
	jsonResp(w, 200, map[string]string{"status": "stopped"})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	users, _ := s.db.ListUsers()
	jsonResp(w, 200, users)
}

func (s *Server) handleHy2Auth(w http.ResponseWriter, r *http.Request) {
	var req struct { User, Password string }
	json.NewDecoder(r.Body).Decode(&req)
	u, _ := s.db.GetUserByUsername(req.User)
	if u != nil && u.Enabled && u.Hy2Password == req.Password { w.WriteHeader(200) } else { w.WriteHeader(403) }
}

func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit { return fmt.Sprintf("%d B", b) }
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func jsonResp(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	jsonResp(w, code, map[string]string{"error": msg})
}
