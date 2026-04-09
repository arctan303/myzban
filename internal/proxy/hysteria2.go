package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/pnm/proxy-node-manager/internal/config"
	"github.com/pnm/proxy-node-manager/internal/db"
)

// Hy2Manager manages the Hysteria 2 proxy
type Hy2Manager struct {
	cfg *config.Config
	db  *db.DB
}

// NewHy2Manager creates a new Hysteria 2 manager
func NewHy2Manager(cfg *config.Config, database *db.DB) *Hy2Manager {
	return &Hy2Manager{cfg: cfg, db: database}
}

// GenerateConfig writes Hysteria2 config.yaml using HTTP auth mode
func (h *Hy2Manager) GenerateConfig(users []*db.User) error {
	// Use HTTP auth mode — our node manager provides the auth endpoint
	// This way, adding/removing users is instant (no restart needed)
	yamlContent := fmt.Sprintf(`listen: :%d

tls:
  cert: %s
  key: %s

auth:
  type: http
  http:
    url: http://%s/hy2/auth
    insecure: true

masquerade:
  type: proxy
  proxy:
    url: https://www.bing.com/
    rewriteHost: true

quic:
  initStreamReceiveWindow: 8388608
  maxStreamReceiveWindow: 8388608
  initConnReceiveWindow: 20971520
  maxConnReceiveWindow: 20971520

trafficStats:
  listen: %s
  secret: %s
`,
		h.cfg.Hy2Port,
		h.cfg.Hy2CertPath, h.cfg.Hy2KeyPath,
		h.cfg.AuthListenAddr,
		h.cfg.Hy2StatsAddr,
		h.cfg.Hy2StatsSecret,
	)

	dir := "/etc/hysteria"
	os.MkdirAll(dir, 0755)
	return os.WriteFile(h.cfg.Hy2ConfigPath, []byte(yamlContent), 0644)
}

// Start starts the Hysteria2 systemd service
func (h *Hy2Manager) Start() error {
	return exec.Command("systemctl", "start", "hysteria-server").Run()
}

// Stop stops the Hysteria2 systemd service
func (h *Hy2Manager) Stop() error {
	return exec.Command("systemctl", "stop", "hysteria-server").Run()
}

// Restart restarts the Hysteria2 systemd service
func (h *Hy2Manager) Restart() error {
	return exec.Command("systemctl", "restart", "hysteria-server").Run()
}

// IsRunning checks if Hysteria2 service is active
func (h *Hy2Manager) IsRunning() bool {
	out, err := exec.Command("systemctl", "is-active", "hysteria-server").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "active"
}

// AddUser — with HTTP auth, no config change needed, user is looked up in DB on each connection
func (h *Hy2Manager) AddUser(user *db.User) error {
	// No-op: HTTP auth queries the database directly
	return nil
}

// RemoveUser — with HTTP auth, disabling in DB is sufficient.
// Optionally kick the user via the stats API.
func (h *Hy2Manager) RemoveUser(user *db.User) error {
	// Kick the user if currently connected
	return h.kickUser(user.Username)
}

// kickUser disconnects a user via the Hysteria2 kick API
func (h *Hy2Manager) kickUser(username string) error {
	body, err := json.Marshal([]string{username})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/kick", h.cfg.Hy2StatsAddr)
	req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.cfg.Hy2StatsSecret != "" {
		req.Header.Set("Authorization", h.cfg.Hy2StatsSecret)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err // service might not be running
	}
	resp.Body.Close()
	return nil
}

// GetTrafficStats queries the Hysteria2 traffic stats API
func (h *Hy2Manager) GetTrafficStats(reset bool) (map[string]*TrafficData, error) {
	url := fmt.Sprintf("http://%s/traffic", h.cfg.Hy2StatsAddr)
	if reset {
		url += "?clear=1"
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if h.cfg.Hy2StatsSecret != "" {
		req.Header.Set("Authorization", h.cfg.Hy2StatsSecret)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stats request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read stats body: %w", err)
	}

	// Hysteria2 returns: {"user_id": {"tx": 123, "rx": 456}}
	var raw map[string]struct {
		Tx int64 `json:"tx"`
		Rx int64 `json:"rx"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse stats: %w (body: %s)", err, string(body))
	}

	result := make(map[string]*TrafficData)
	for id, data := range raw {
		result[id] = &TrafficData{
			Upload:   data.Tx,
			Download: data.Rx,
		}
	}
	return result, nil
}

// GenerateClientConfig generates Clash YAML config for a specific user
func (h *Hy2Manager) GenerateClientConfig(user *db.User, nodeInfo *db.NodeInfo) string {
	return fmt.Sprintf(`  - name: "%s-UDP"
    type: hysteria2
    server: %s
    port: %d
    password: %s
    up: 50 Mbps
    down: 100 Mbps
    sni: www.bing.com
    skip-cert-verify: true
    alpn:
      - h3`,
		nodeInfo.ServerIP, nodeInfo.ServerIP, h.cfg.Hy2Port,
		user.Hy2Password)
}
