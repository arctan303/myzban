package installer

import (
	"fmt"
	"os"
	"strings"

	"github.com/pnm/proxy-node-manager/internal/config"
	"github.com/pnm/proxy-node-manager/internal/db"
)

// XrayInstaller installs and manages Xray core
type XrayInstaller struct {
	cfg *config.Config
	db  *db.DB
}

// NewXrayInstaller creates a new Xray installer
func NewXrayInstaller(cfg *config.Config, database *db.DB) *XrayInstaller {
	return &XrayInstaller{cfg: cfg, db: database}
}

// IsInstalled checks if Xray binary exists
func (x *XrayInstaller) IsInstalled() bool {
	_, err := os.Stat(x.cfg.XrayBinPath)
	return err == nil
}

// Install downloads and installs Xray with Reality support
func (x *XrayInstaller) Install() error {
	if x.IsInstalled() {
		return fmt.Errorf("xray is already installed at %s", x.cfg.XrayBinPath)
	}

	fmt.Println("[1/5] Installing dependencies...")
	if _, err := runShell("apt update -y && apt install -y curl wget sudo openssl"); err != nil {
		return fmt.Errorf("install dependencies: %w", err)
	}

	fmt.Println("[2/5] Installing Xray v1.8.24...")
	script := `bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install -u root --version v1.8.24`
	if _, err := runShell(script); err != nil {
		return fmt.Errorf("install xray: %w", err)
	}

	fmt.Println("[3/5] Generating credentials...")
	// Generate UUID
	uuidOut, err := runCmd(x.cfg.XrayBinPath, "uuid")
	if err != nil {
		return fmt.Errorf("generate uuid: %w", err)
	}
	_ = strings.TrimSpace(uuidOut) // base UUID not stored globally, per-user

	// Generate X25519 key pair
	keysOut, err := runCmd(x.cfg.XrayBinPath, "x25519")
	if err != nil {
		return fmt.Errorf("generate x25519: %w", err)
	}
	privateKey, publicKey := parseXrayKeys(keysOut)
	if privateKey == "" || publicKey == "" {
		return fmt.Errorf("failed to parse x25519 keys from output:\n%s", keysOut)
	}

	// Generate short ID
	shortIDOut, err := runShell("openssl rand -hex 8")
	if err != nil {
		return fmt.Errorf("generate shortid: %w", err)
	}
	shortID := strings.TrimSpace(shortIDOut)

	// Detect server IP
	fmt.Println("[4/5] Detecting server IP...")
	serverIP, err := GetServerIP()
	if err != nil {
		return fmt.Errorf("detect server ip: %w", err)
	}

	// Save node info to DB
	nodeInfo := &db.NodeInfo{
		ServerIP:   serverIP,
		XrayPubKey: publicKey,
		XrayPriKey: privateKey,
		ShortID:    shortID,
	}
	// Preserve existing hy2 fields
	existing, _ := x.db.GetNodeInfo()
	if existing != nil {
		nodeInfo.Hy2Cert = existing.Hy2Cert
		nodeInfo.Hy2Key = existing.Hy2Key
	}
	if err := x.db.SaveNodeInfo(nodeInfo); err != nil {
		return fmt.Errorf("save node info: %w", err)
	}

	// Mark as installed
	proxyConf, _ := x.db.GetProxyConfig("vless")
	if proxyConf == nil {
		proxyConf = &db.ProxyConfig{Protocol: "vless"}
	}
	proxyConf.Port = x.cfg.VLESSPort
	proxyConf.Installed = true
	if err := x.db.SaveProxyConfig(proxyConf); err != nil {
		return fmt.Errorf("save proxy config: %w", err)
	}

	fmt.Println("[5/5] Xray installed successfully!")
	fmt.Printf("  Server IP:   %s\n", serverIP)
	fmt.Printf("  Public Key:  %s\n", publicKey)
	fmt.Printf("  Short ID:    %s\n", shortID)
	fmt.Printf("  Port:        %d\n", x.cfg.VLESSPort)
	return nil
}

// Uninstall removes Xray
func (x *XrayInstaller) Uninstall() error {
	if _, err := runShell("systemctl stop xray 2>/dev/null; systemctl disable xray 2>/dev/null"); err != nil {
		// ignore errors
	}
	script := `bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ remove --purge`
	if _, err := runShell(script); err != nil {
		return fmt.Errorf("uninstall xray: %w", err)
	}

	proxyConf, _ := x.db.GetProxyConfig("vless")
	if proxyConf != nil {
		proxyConf.Installed = false
		x.db.SaveProxyConfig(proxyConf)
	}
	return nil
}

func parseXrayKeys(output string) (privateKey, publicKey string) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(line), "private") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				privateKey = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(strings.ToLower(line), "public") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				publicKey = strings.TrimSpace(parts[1])
			}
		}
	}
	return
}
