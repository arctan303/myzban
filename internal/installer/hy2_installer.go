package installer

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"time"

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
	if _, err := runCmd("apt", "update", "-y"); err == nil {
		runCmd("apt", "install", "-y", "wget", "curl")
	}

	fmt.Println("[2/5] Downloading Hysteria 2...")
	// Get latest version securely via native HTTP
	version := "v2.2.4" // fallback
	resp, err := http.Get("https://api.github.com/repos/apernet/hysteria/releases/latest")
	if err == nil {
		defer resp.Body.Close()
		var release struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&release); err == nil && release.TagName != "" {
			version = release.TagName
		}
	}
	fmt.Printf("  Latest version: %s\n", version)

	downloadURL := fmt.Sprintf("https://github.com/apernet/hysteria/releases/download/%s/hysteria-linux-%s", version, arch)
	if _, err := runCmd("wget", "-O", h.cfg.Hy2BinPath, downloadURL); err != nil {
		return fmt.Errorf("download hysteria2: %w", err)
	}
	if _, err := runCmd("chmod", "+x", h.cfg.Hy2BinPath); err != nil {
		return fmt.Errorf("chmod hysteria2: %w", err)
	}

	fmt.Println("[3/5] Generating self-signed certificate...")
	if err := os.MkdirAll("/etc/hysteria", 0755); err != nil {
		return fmt.Errorf("create cert dir: %w", err)
	}
	
	if err := generateSelfSignedCert(h.cfg.Hy2CertPath, h.cfg.Hy2KeyPath); err != nil {
		return fmt.Errorf("generate certificate natively: %w", err)
	}

	fmt.Println("[4/5] Detecting server IP...")
	serverIP, err := GetServerIP()
	if err != nil {
		return fmt.Errorf("detect server ip: %w", err)
	}

	// Generate API secret for traffic stats securely via crypto/rand
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("generate api secret: %w", err)
	}
	apiSecret := hex.EncodeToString(b)

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

	if _, err := runCmd("systemctl", "daemon-reload"); err != nil {
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
	runCmd("systemctl", "stop", "hysteria-server")
	runCmd("systemctl", "disable", "hysteria-server")

	os.Remove(h.cfg.Hy2BinPath)
	os.Remove("/etc/systemd/system/hysteria-server.service")
	runCmd("systemctl", "daemon-reload")

	proxyConf, _ := h.db.GetProxyConfig("hysteria2")
	if proxyConf != nil {
		proxyConf.Installed = false
		h.db.SaveProxyConfig(proxyConf)
	}
	return nil
}

func generateSelfSignedCert(certPath, keyPath string) error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "www.bing.com",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(3650 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}
	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}
	keyOut, err := os.Create(keyPath)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return err
	}
	return nil
}
