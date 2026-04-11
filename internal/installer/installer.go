package installer

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Installer is the interface for installing proxy software
type Installer interface {
	Install() error
	IsInstalled() bool
	Uninstall() error
}

// getArch returns the download architecture string
func getArch() (string, error) {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		// fallback: try uname
		out, err := exec.Command("uname", "-m").Output()
		if err != nil {
			return "", fmt.Errorf("unsupported architecture: %s", arch)
		}
		uname := strings.TrimSpace(string(out))
		switch uname {
		case "x86_64":
			return "amd64", nil
		case "aarch64", "arm64":
			return "arm64", nil
		default:
			return "", fmt.Errorf("unsupported architecture: %s", uname)
		}
	}
}

// runCmd executes a shell command and returns combined output
func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// GetServerIP attempts to detect the public IPv4 address securely natively
func GetServerIP() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	
	resp, err := client.Get("https://api.ipify.org")
	if err == nil {
		defer resp.Body.Close()
		if b, err := io.ReadAll(resp.Body); err == nil {
			ip := strings.TrimSpace(string(b))
			if ip != "" {
				return ip, nil
			}
		}
	}

	resp2, err2 := client.Get("https://ifconfig.me")
	if err2 == nil {
		defer resp2.Body.Close()
		if b, err := io.ReadAll(resp2.Body); err == nil {
			ip := strings.TrimSpace(string(b))
			if ip != "" {
				return ip, nil
			}
		}
	}

	return "", fmt.Errorf("failed to detect public IP natively")
}
