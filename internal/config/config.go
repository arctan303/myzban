package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	DefaultConfigDir  = "/etc/pnm"
	DefaultDBPath     = "/etc/pnm/pnm.db"
	DefaultAPIAddr    = "127.0.0.1:9090"
	DefaultAuthAddr   = "127.0.0.1:19876" // Hy2 HTTP auth endpoint
	DefaultVLESSPort  = 443
	DefaultHy2Port    = 8443
	DefaultCollectSec = 60 // traffic collection interval
)

// Config holds global configuration for the node manager
type Config struct {
	DBPath          string `json:"db_path"`
	APIListenAddr   string `json:"api_listen_addr"`
	AuthListenAddr  string `json:"auth_listen_addr"`
	VLESSPort       int    `json:"vless_port"`
	Hy2Port         int    `json:"hy2_port"`
	CollectInterval int    `json:"collect_interval"`

	// Xray paths
	XrayBinPath    string `json:"xray_bin_path"`
	XrayConfigPath string `json:"xray_config_path"`
	XrayAPIAddr    string `json:"xray_api_addr"`

	// Hysteria2 paths
	Hy2BinPath    string `json:"hy2_bin_path"`
	Hy2ConfigPath string `json:"hy2_config_path"`
	Hy2StatsAddr  string `json:"hy2_stats_addr"`
	Hy2StatsSecret string `json:"hy2_stats_secret"`

	// TLS for Hy2
	Hy2CertPath string `json:"hy2_cert_path"`
	Hy2KeyPath  string `json:"hy2_key_path"`

	// VLESS Reality
	DestDomain string `json:"dest_domain"`
}

// DefaultConfig returns a config with sane defaults
func DefaultConfig() *Config {
	return &Config{
		DBPath:          DefaultDBPath,
		APIListenAddr:   DefaultAPIAddr,
		AuthListenAddr:  DefaultAuthAddr,
		VLESSPort:       DefaultVLESSPort,
		Hy2Port:         DefaultHy2Port,
		CollectInterval: DefaultCollectSec,

		XrayBinPath:    "/usr/local/bin/xray",
		XrayConfigPath: "/usr/local/etc/xray/config.json",
		XrayAPIAddr:    "127.0.0.1:10085",

		Hy2BinPath:    "/usr/local/bin/hysteria",
		Hy2ConfigPath: "/etc/hysteria/config.yaml",
		Hy2StatsAddr:  "127.0.0.1:25413",
		Hy2StatsSecret: "",

		Hy2CertPath: "/etc/hysteria/server.crt",
		Hy2KeyPath:  "/etc/hysteria/server.key",

		DestDomain: "www.cloudflare.com",
	}
}

// ConfigFilePath returns the path to the config file
func ConfigFilePath() string {
	return filepath.Join(DefaultConfigDir, "config.json")
}

// Load loads config from disk; returns default if file doesn't exist
func Load() (*Config, error) {
	path := ConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save persists the config to disk
func (c *Config) Save() error {
	path := ConfigFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
