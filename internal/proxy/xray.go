package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pnm/proxy-node-manager/internal/config"
	"github.com/pnm/proxy-node-manager/internal/db"
)

// XrayManager manages the Xray VLESS Reality proxy
type XrayManager struct {
	cfg *config.Config
	db  *db.DB
}

// NewXrayManager creates a new Xray manager
func NewXrayManager(cfg *config.Config, database *db.DB) *XrayManager {
	return &XrayManager{cfg: cfg, db: database}
}

// xrayConfig represents the full Xray config.json structure
type xrayConfig struct {
	Log       xrayLog       `json:"log"`
	Stats     struct{}      `json:"stats"`
	API       xrayAPI       `json:"api"`
	Policy    xrayPolicy    `json:"policy"`
	Inbounds  []xrayInbound `json:"inbounds"`
	Outbounds []xrayOutbound `json:"outbounds"`
	Routing   xrayRouting   `json:"routing"`
}

type xrayLog struct {
	Loglevel string `json:"loglevel"`
}

type xrayAPI struct {
	Tag      string   `json:"tag"`
	Listen   string   `json:"listen"`
	Services []string `json:"services"`
}

type xrayPolicy struct {
	Levels map[string]xrayPolicyLevel `json:"levels"`
}

type xrayPolicyLevel struct {
	StatsUserUplink   bool `json:"statsUserUplink"`
	StatsUserDownlink bool `json:"statsUserDownlink"`
}

type xrayInbound struct {
	Tag            string          `json:"tag"`
	Listen         string          `json:"listen,omitempty"`
	Port           interface{}     `json:"port"`
	Protocol       string          `json:"protocol"`
	Settings       json.RawMessage `json:"settings"`
	StreamSettings json.RawMessage `json:"streamSettings,omitempty"`
	Sniffing       json.RawMessage `json:"sniffing,omitempty"`
}

type xrayOutbound struct {
	Tag      string `json:"tag"`
	Protocol string `json:"protocol"`
}

type xrayRouting struct {
	Rules []xrayRoutingRule `json:"rules"`
}

type xrayRoutingRule struct {
	Type        string   `json:"type"`
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag"`
}

type vlessClient struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type vlessSettings struct {
	Clients    []vlessClient `json:"clients"`
	Decryption string        `json:"decryption"`
}

type realitySettings struct {
	PrivateKey  string   `json:"privateKey"`
	ServerNames []string `json:"serverNames"`
	ShortIds    []string `json:"shortIds"`
	Dest        string   `json:"dest"`
	Xver        int      `json:"xver"`
}

type streamSettings struct {
	Network         string          `json:"network"`
	Security        string          `json:"security"`
	RealitySettings realitySettings `json:"realitySettings"`
}

type sniffingSettings struct {
	Enabled      bool     `json:"enabled"`
	DestOverride []string `json:"destOverride"`
}

// GenerateConfig writes config.json with all enabled users
func (x *XrayManager) GenerateConfig(users []*db.User) error {
	nodeInfo, err := x.db.GetNodeInfo()
	if err != nil {
		return fmt.Errorf("get node info: %w", err)
	}

	if nodeInfo.XrayPriKey == "" {
		return fmt.Errorf("xray not installed: no private key found")
	}

	// Build client list from enabled users
	var clients []vlessClient
	for _, u := range users {
		if u.Enabled {
			clients = append(clients, vlessClient{
				ID:    u.UUID,
				Email: u.Email,
			})
		}
	}

	// If no users, add a placeholder to keep xray happy
	if len(clients) == 0 {
		clients = []vlessClient{{
			ID:    "00000000-0000-0000-0000-000000000000",
			Email: "placeholder@pnm",
		}}
	}

	vlessSettingsData := vlessSettings{
		Clients:    clients,
		Decryption: "none",
	}
	vlessJSON, _ := json.Marshal(vlessSettingsData)

	streamData := streamSettings{
		Network:  "tcp",
		Security: "reality",
		RealitySettings: realitySettings{
			PrivateKey:  nodeInfo.XrayPriKey,
			ServerNames: []string{x.cfg.DestDomain},
			ShortIds:    []string{nodeInfo.ShortID},
			Dest:        fmt.Sprintf("%s:443", x.cfg.DestDomain),
			Xver:        0,
		},
	}
	streamJSON, _ := json.Marshal(streamData)

	sniffData := sniffingSettings{
		Enabled:      true,
		DestOverride: []string{"http", "tls"},
	}
	sniffJSON, _ := json.Marshal(sniffData)

	// API inbound (dokodemo-door for gRPC)
	apiInboundSettings, _ := json.Marshal(map[string]interface{}{
		"address": "127.0.0.1",
	})

	cfg := xrayConfig{
		Log: xrayLog{Loglevel: "warning"},
		API: xrayAPI{
			Tag:      "api",
			Listen:   x.cfg.XrayAPIAddr,
			Services: []string{"StatsService", "HandlerService"},
		},
		Policy: xrayPolicy{
			Levels: map[string]xrayPolicyLevel{
				"0": {
					StatsUserUplink:   true,
					StatsUserDownlink: true,
				},
			},
		},
		Inbounds: []xrayInbound{
			{
				Tag:            "vless-reality",
				Listen:         "0.0.0.0",
				Port:           x.cfg.VLESSPort,
				Protocol:       "vless",
				Settings:       json.RawMessage(vlessJSON),
				StreamSettings: json.RawMessage(streamJSON),
				Sniffing:       json.RawMessage(sniffJSON),
			},
			{
				Tag:      "api-inbound",
				Listen:   "127.0.0.1",
				Port:     10085,
				Protocol: "dokodemo-door",
				Settings: json.RawMessage(apiInboundSettings),
			},
		},
		Outbounds: []xrayOutbound{
			{Tag: "direct", Protocol: "freedom"},
			{Tag: "block", Protocol: "blackhole"},
		},
		Routing: xrayRouting{
			Rules: []xrayRoutingRule{
				{
					Type:        "field",
					InboundTag:  []string{"api-inbound"},
					OutboundTag: "api",
				},
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	dir := strings.TrimSuffix(x.cfg.XrayConfigPath, "/config.json")
	os.MkdirAll(dir, 0755)

	return os.WriteFile(x.cfg.XrayConfigPath, data, 0644)
}

// Start starts the Xray systemd service
func (x *XrayManager) Start() error {
	return exec.Command("systemctl", "start", "xray").Run()
}

// Stop stops the Xray systemd service
func (x *XrayManager) Stop() error {
	return exec.Command("systemctl", "stop", "xray").Run()
}

// Restart restarts the Xray systemd service
func (x *XrayManager) Restart() error {
	return exec.Command("systemctl", "restart", "xray").Run()
}

// IsRunning checks if the Xray service is active
func (x *XrayManager) IsRunning() bool {
	out, err := exec.Command("systemctl", "is-active", "xray").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "active"
}

// AddUser adds a user to the running Xray via config regeneration + restart
// (gRPC HandlerService could be used for hot-reload, but config regen is more reliable)
func (x *XrayManager) AddUser(user *db.User) error {
	users, err := x.db.ListEnabledUsers()
	if err != nil {
		return err
	}
	if err := x.GenerateConfig(users); err != nil {
		return err
	}
	if x.IsRunning() {
		return x.Restart()
	}
	return nil
}

// RemoveUser removes a user from Xray via config regeneration + restart
func (x *XrayManager) RemoveUser(user *db.User) error {
	users, err := x.db.ListEnabledUsers()
	if err != nil {
		return err
	}
	if err := x.GenerateConfig(users); err != nil {
		return err
	}
	if x.IsRunning() {
		return x.Restart()
	}
	return nil
}

// GetTrafficStats queries Xray stats API via `xray api statsquery`
func (x *XrayManager) GetTrafficStats(reset bool) (map[string]*TrafficData, error) {
	args := []string{"api", "statsquery", "--server=" + x.cfg.XrayAPIAddr}
	if reset {
		args = append(args, "-reset=true")
	}

	cmd := exec.Command(x.cfg.XrayBinPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("statsquery: %w: %s", err, string(out))
	}

	return parseXrayStats(string(out)), nil
}

// parseXrayStats parses the xray api statsquery output
// Format: stat: { name: "user>>>email>>>traffic>>>uplink" value: 12345 }
func parseXrayStats(output string) map[string]*TrafficData {
	result := make(map[string]*TrafficData)

	// Try JSON parsing first
	var jsonResp struct {
		Stat []struct {
			Name  string `json:"name"`
			Value int64  `json:"value"`
		} `json:"stat"`
	}
	if err := json.Unmarshal([]byte(output), &jsonResp); err == nil {
		for _, s := range jsonResp.Stat {
			parts := strings.Split(s.Name, ">>>")
			if len(parts) == 4 && parts[0] == "user" {
				email := parts[1]
				direction := parts[3] // "uplink" or "downlink"
				if _, ok := result[email]; !ok {
					result[email] = &TrafficData{}
				}
				if direction == "uplink" {
					result[email].Upload = s.Value
				} else if direction == "downlink" {
					result[email].Download = s.Value
				}
			}
		}
	}

	return result
}

// GenerateClientConfig generates Clash YAML config for a specific user
func (x *XrayManager) GenerateClientConfig(user *db.User, nodeInfo *db.NodeInfo) string {
	return fmt.Sprintf(`  - name: "%s-TCP"
    type: vless
    server: %s
    port: %d
    uuid: %s
    udp: true
    tls: true
    network: tcp
    servername: %s
    client-fingerprint: chrome
    reality-opts:
      public-key: %s
      short-id: %s`,
		nodeInfo.ServerIP, nodeInfo.ServerIP, x.cfg.VLESSPort,
		user.UUID, x.cfg.DestDomain,
		nodeInfo.XrayPubKey, nodeInfo.ShortID)
}
