package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pnm/proxy-node-manager/internal/config"
	"github.com/pnm/proxy-node-manager/internal/db"
)

// Hy2Installer installs and manages Hysteria 2
type Hy2Installer struct {
	cfg *config.Config
	db  *db.DB
}

// NewHy2Installer creates a new Hysteria 2 installer
func NewHy2Installer(cfg *config.Config, database *db.DB) *Hy2Installer {
	return &Hy2Installer{cfg: cfg, db: database}
}

// IsInstalled checks if Hysteria2 binary exists
func (h *Hy2Installer) IsInstalled() bool {
	_, err := os.Stat(h.cfg.Hy2BinPath)
	return err == nil
}

// Install downloads and sets up Hysteria 2
func (h *Hy2Installer) Install() error {
	if h.IsInstalled() {
		return fmt.Errorf("hysteria2 is already installed at %s", h.cfg.Hy2BinPath)
	}

	arch, err := getArch()
	if err != nil {
		return err
	}

	fmt.Println("[1/5] Installing dependencies...")
	if _, err := runShell("apt update -y && apt install -y wget openssl curl"); err != nil {
		return fmt.Errorf("install dependencies: %w", err)
	}

	fmt.Println("[2/5] Downloading Hysteria 2...")
	// Get latest version
	verScript := `curl -s https://api.github.com/repos/apernet/hysteria/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'`
	verOut, err := runShell(verScript)
	version := strings.TrimSpace(verOut)
	if err != nil || version == "" {
		version = "v2.2.4" // fallback
	}
	fmt.Printf("  Latest version: %s\n", version)

	downloadURL := fmt.Sprintf("https://github.com/apernet/hysteria/releases/download/%s/hysteria-linux-%s", version, arch)
	downloadScript := fmt.Sprintf("wget -O %s '%s' && chmod +x %s", h.cfg.Hy2BinPath, downloadURL, h.cfg.Hy2BinPath)
	if _, err := runShell(downloadScript); err != nil {
		return fmt.Errorf("download hysteria2: %w", err)
	}

	fmt.Println("[3/5] Generating self-signed certificate...")
	if _, err := runShell("mkdir -p /etc/hysteria"); err != nil {
		return fmt.Errorf("create cert dir: %w", err)
	}
	certScript := fmt.Sprintf(
		`openssl req -x509 -nodes -newkey rsa:2048 -keyout %s -out %s -days 3650 -subj "/CN=www.bing.com"`,
		h.cfg.Hy2KeyPath, h.cfg.Hy2CertPath,
	)
	if _, err := runShell(certScript); err != nil {
		return fmt.Errorf("generate certificate: %w", err)
	}

	fmt.Println("[4/5] Detecting server IP...")
	serverIP, err := GetServerIP()
	if err != nil {
		return fmt.Errorf("detect server ip: %w", err)
	}

	// Generate API secret for traffic stats
	secretOut, err := runShell("openssl rand -hex 16")
	if err != nil {
		return fmt.Errorf("generate api secret: %w", err)
	}
	apiSecret := strings.TrimSpace(secretOut)

	// Save node info
	nodeInfo, _ := h.db.GetNodeInfo()
	if nodeInfo == nil {
		nodeInfo = &db.NodeInfo{}
	}
	nodeInfo.ServerIP = serverIP
	nodeInfo.Hy2Cert = h.cfg.Hy2CertPath
	nodeInfo.Hy2Key = h.cfg.Hy2KeyPath
	if err := h.db.SaveNodeInfo(nodeInfo); err != nil {
		return fmt.Errorf("save node info: %w", err)
	}

	// Save proxy config with the API secret
	proxyConf, _ := h.db.GetProxyConfig("hysteria2")
	if proxyConf == nil {
		proxyConf = &db.ProxyConfig{Protocol: "hysteria2"}
	}
	proxyConf.Port = h.cfg.Hy2Port
	proxyConf.Installed = true
	extra := map[string]string{
		"version":    version,
		"api_secret": apiSecret,
	}
	extraJSON, _ := json.Marshal(extra)
	proxyConf.ExtraJSON = string(extraJSON)
	if err := h.db.SaveProxyConfig(proxyConf); err != nil {
		return fmt.Errorf("save proxy config: %w", err)
	}

	// Update config with the secret
	h.cfg.Hy2StatsSecret = apiSecret

	fmt.Println("[5/5] Creating systemd service...")
	serviceContent := fmt.Sprintf(`[Unit]
Description=Hysteria 2 Server
After=network.target

[Service]
Type=simple
User=root
ExecStart=%s server -c %s
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`, h.cfg.Hy2BinPath, h.cfg.Hy2ConfigPath)

	if err := os.WriteFile("/etc/systemd/system/hysteria-server.service", []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}

	if _, err := runShell("systemctl daemon-reload"); err != nil {
		return fmt.Errorf("daemon reload: %w", err)
	}

	fmt.Println("Hysteria 2 installed successfully!")
	fmt.Printf("  Server IP:    %s\n", serverIP)
	fmt.Printf("  Port:         %d (UDP)\n", h.cfg.Hy2Port)
	fmt.Printf("  Version:      %s\n", version)
	fmt.Println("  ⚠  Remember to open UDP port in firewall!")
	return nil
}

// Uninstall removes Hysteria 2
func (h *Hy2Installer) Uninstall() error {
	if _, err := runShell("systemctl stop hysteria-server 2>/dev/null; systemctl disable hysteria-server 2>/dev/null"); err != nil {
		// ignore
	}
	os.Remove(h.cfg.Hy2BinPath)
	os.Remove("/etc/systemd/system/hysteria-server.service")
	runShell("systemctl daemon-reload")

	proxyConf, _ := h.db.GetProxyConfig("hysteria2")
	if proxyConf != nil {
		proxyConf.Installed = false
		h.db.SaveProxyConfig(proxyConf)
	}
	return nil
}
