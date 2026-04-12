package api

import (
	"encoding/json"
	"fmt"
	"io"
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
	cfg           *config.Config
	db            *db.DB
	userSvc       *user.Service
	xray          *proxy.XrayManager
	hy2           *proxy.Hy2Manager
	xrayInstaller *installer.XrayInstaller
	hy2Installer  *installer.Hy2Installer
}

func NewServer(cfg *config.Config, db *db.DB, userSvc *user.Service, xray *proxy.XrayManager, hy2 *proxy.Hy2Manager, xi *installer.XrayInstaller, hi *installer.Hy2Installer) *Server {
	return &Server{
		cfg:           cfg,
		db:            db,
		userSvc:       userSvc,
		xray:          xray,
		hy2:           hy2,
		xrayInstaller: xi,
		hy2Installer:  hi,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", s.adminAuth(s.handleStatus))
	mux.HandleFunc("/api/v1/proxy/install", s.adminAuth(s.handleProxyInstall))
	mux.HandleFunc("/api/v1/proxy/start", s.adminAuth(s.handleProxyStart))
	mux.HandleFunc("/api/v1/proxy/stop", s.adminAuth(s.handleProxyStop))
	mux.HandleFunc("/api/v1/users", s.adminAuth(s.handleUsers))
	mux.HandleFunc("/api/v1/users/", s.adminAuth(s.handleUserOps))
	mux.HandleFunc("/api/v1/node", s.adminAuth(s.handleNodeInfo))
	mux.HandleFunc("/hy2/auth", s.handleHy2Auth)

	addr := s.cfg.APIListenAddr
	if addr == ":" { addr = ":9090" }
	log.Printf("[INFO] API server starting on %s", addr)
	return http.ListenAndServe(addr, withCORS(mux))
}

func (s *Server) StartAuthEndpoint() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/hy2/auth", s.handleHy2Auth)
	addr := ":19876"
	log.Printf("[INFO] Auth endpoint starting on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) adminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			token = r.Header.Get("Authorization")
			token = strings.TrimPrefix(token, "Bearer ")
		}
		nodeInfo, _ := s.db.GetNodeInfo()
		if nodeInfo == nil || nodeInfo.AdminToken == "" || token != nodeInfo.AdminToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	nodeInfo, _ := s.db.GetNodeInfo()
	publicIP := ""
	if nodeInfo != nil { publicIP = nodeInfo.ServerIP }
	if publicIP == "" {
		resp, err := http.Get("https://api.ipify.org")
		if err == nil {
			defer resp.Body.Close()
			ip, _ := io.ReadAll(resp.Body)
			publicIP = string(ip)
		}
	}

	vlessConf, _ := s.db.GetProxyConfig("vless")
	hy2Conf, _ := s.db.GetProxyConfig("hysteria2")

	status := map[string]interface{}{
		"version": "1.0.0",
		"time":    time.Now().Format(time.RFC3339),
		"server_ip": publicIP,
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

func (s *Server) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	nodeInfo, err := s.db.GetNodeInfo()
	if err != nil { jsonError(w, 500, err.Error()); return }
	jsonResp(w, http.StatusOK, map[string]interface{}{
		"server_ip": nodeInfo.ServerIP,
		"xray_pub_key": nodeInfo.XrayPubKey,
		"short_id": nodeInfo.ShortID,
		"dest_domain": s.cfg.DestDomain,
	})
}

func (s *Server) handleProxyInstall(w http.ResponseWriter, r *http.Request) {
	var req struct { Protocol string `json:"protocol"` }
	json.NewDecoder(r.Body).Decode(&req)
	var err error
	if req.Protocol == "vless" { err = s.xrayInstaller.Install() } else { err = s.hy2Installer.Install() }
	if err != nil { jsonError(w, 500, err.Error()); return }
	jsonResp(w, 200, map[string]string{"status": "installed"})
}

func (s *Server) handleProxyStart(w http.ResponseWriter, r *http.Request) {
	var req struct { Protocol string `json:"protocol"` }
	json.NewDecoder(r.Body).Decode(&req)
	var err error
	if req.Protocol == "vless" { err = s.xray.Start() } else { err = s.hy2.Restart() }
	if err != nil { jsonError(w, 500, err.Error()); return }
	jsonResp(w, 200, map[string]string{"status": "started"})
}

func (s *Server) handleProxyStop(w http.ResponseWriter, r *http.Request) {
	var req struct { Protocol string `json:"protocol"` }
	json.NewDecoder(r.Body).Decode(&req)
	var err error
	if req.Protocol == "vless" { err = s.xray.Stop() } else { err = s.hy2.Stop() }
	if err != nil { jsonError(w, 500, err.Error()); return }
	jsonResp(w, 200, map[string]string{"status": "stopped"})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		users, _ := s.db.ListUsers()
		jsonResp(w, 200, users)
	case http.MethodPost:
		var req struct { Username string `json:"username"` }
		json.NewDecoder(r.Body).Decode(&req)
		u, err := s.userSvc.CreateUser(req.Username)
		if err != nil { jsonError(w, 500, err.Error()); return }
		jsonResp(w, 201, u)
	}
}

func (s *Server) handleUserOps(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	if username == "" { return }
	parts := strings.Split(username, "/")
	uname := parts[0]
	if r.Method == http.MethodDelete {
		if err := s.userSvc.DeleteUser(uname); err != nil { jsonError(w, 500, err.Error()); return }
		jsonResp(w, 200, map[string]string{"status": "deleted"})
		return
	}
	if len(parts) > 1 && r.Method == http.MethodPost {
		action := parts[1]
		if action == "enable" { s.userSvc.EnableUser(uname) }
		if action == "disable" { s.userSvc.DisableUser(uname) }
		jsonResp(w, 200, map[string]string{"status": "ok"})
	}
}

func (s *Server) handleHy2Auth(w http.ResponseWriter, r *http.Request) {
	var req struct { User, Password string }
	json.NewDecoder(r.Body).Decode(&req)
	u, _ := s.db.GetUserByUsername(req.User)
	if u != nil && u.Enabled && u.Hy2Password == req.Password { w.WriteHeader(200) } else { w.WriteHeader(403) }
}

func FormatBytes(b int64) string { return fmt.Sprintf("%d B", b) }

func jsonResp(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	jsonResp(w, code, map[string]string{"error": msg})
}
