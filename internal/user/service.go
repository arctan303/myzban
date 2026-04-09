package user

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/pnm/proxy-node-manager/internal/config"
	"github.com/pnm/proxy-node-manager/internal/db"
	"github.com/pnm/proxy-node-manager/internal/proxy"
)

// Service provides user management business logic
type Service struct {
	db   *db.DB
	cfg  *config.Config
	xray *proxy.XrayManager
	hy2  *proxy.Hy2Manager
}

// NewService creates a new user service
func NewService(database *db.DB, cfg *config.Config, xray *proxy.XrayManager, hy2 *proxy.Hy2Manager) *Service {
	return &Service{
		db:   database,
		cfg:  cfg,
		xray: xray,
		hy2:  hy2,
	}
}

// CreateUser creates a new user with auto-generated credentials
func (s *Service) CreateUser(username string) (*db.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}

	existing, _ := s.db.GetUserByUsername(username)
	if existing != nil {
		return nil, fmt.Errorf("user '%s' already exists", username)
	}

	user := &db.User{
		Username:    username,
		Email:       fmt.Sprintf("%s@pnm", username),
		UUID:        uuid.New().String(),
		Hy2Password: generateToken(16),
		SubToken:    generateToken(24), // longer token for subscription URLs
		Enabled:     true,
	}

	if err := s.db.CreateUser(user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	s.syncProxies()
	return user, nil
}

// GetUser retrieves a user by username
func (s *Service) GetUser(identifier string) (*db.User, error) {
	user, err := s.db.GetUserByUsername(identifier)
	if err == nil {
		return user, nil
	}
	return nil, fmt.Errorf("user '%s' not found", identifier)
}

// ListUsers returns all users
func (s *Service) ListUsers() ([]*db.User, error) {
	return s.db.ListUsers()
}

// EnableUser enables a user and syncs proxies
func (s *Service) EnableUser(username string) error {
	user, err := s.db.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("user '%s' not found", username)
	}
	if user.Enabled {
		return fmt.Errorf("user '%s' is already enabled", username)
	}
	if err := s.db.SetUserEnabled(user.ID, true); err != nil {
		return err
	}
	s.syncProxies()
	return nil
}

// DisableUser disables a user and removes from proxies
func (s *Service) DisableUser(username string) error {
	user, err := s.db.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("user '%s' not found", username)
	}
	if !user.Enabled {
		return fmt.Errorf("user '%s' is already disabled", username)
	}
	if err := s.db.SetUserEnabled(user.ID, false); err != nil {
		return err
	}
	if s.xray != nil {
		s.xray.RemoveUser(user)
	}
	if s.hy2 != nil {
		s.hy2.RemoveUser(user)
	}
	return nil
}

// DeleteUser removes a user completely
func (s *Service) DeleteUser(username string) error {
	user, err := s.db.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("user '%s' not found", username)
	}
	if s.xray != nil {
		s.xray.RemoveUser(user)
	}
	if s.hy2 != nil {
		s.hy2.RemoveUser(user)
	}
	return s.db.DeleteUser(user.ID)
}

// ResetTraffic resets a user's traffic counters
func (s *Service) ResetTraffic(username string) error {
	user, err := s.db.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("user '%s' not found", username)
	}
	return s.db.ResetTraffic(user.ID)
}

// GetTrafficLogs returns traffic history for a user
func (s *Service) GetTrafficLogs(username string, limit int) ([]*db.TrafficLog, error) {
	user, err := s.db.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("user '%s' not found", username)
	}
	return s.db.GetTrafficLogs(user.ID, limit)
}

// GetClientConfig generates per-user Clash YAML for all installed protocols
func (s *Service) GetClientConfig(username string) (string, error) {
	user, err := s.db.GetUserByUsername(username)
	if err != nil {
		return "", fmt.Errorf("user '%s' not found", username)
	}

	nodeInfo, err := s.db.GetNodeInfo()
	if err != nil {
		return "", fmt.Errorf("get node info: %w", err)
	}

	var parts []string

	vlessConf, _ := s.db.GetProxyConfig("vless")
	if vlessConf != nil && vlessConf.Installed {
		parts = append(parts, s.xray.GenerateClientConfig(user, nodeInfo))
	}

	hy2Conf, _ := s.db.GetProxyConfig("hysteria2")
	if hy2Conf != nil && hy2Conf.Installed {
		parts = append(parts, s.hy2.GenerateClientConfig(user, nodeInfo))
	}

	if len(parts) == 0 {
		return "", fmt.Errorf("no proxy protocols installed")
	}

	return strings.Join(parts, "\n"), nil
}

// GetSubscriptionURL returns the subscription URL for a user
func (s *Service) GetSubscriptionURL(username string) (string, error) {
	user, err := s.db.GetUserByUsername(username)
	if err != nil {
		return "", fmt.Errorf("user '%s' not found", username)
	}
	nodeInfo, _ := s.db.GetNodeInfo()
	ip := nodeInfo.ServerIP
	if ip == "" {
		ip = "YOUR_SERVER_IP"
	}
	return fmt.Sprintf("http://%s:9090/sub/%s", ip, user.SubToken), nil
}

// syncProxies regenerates configs and restarts if needed
func (s *Service) syncProxies() {
	users, err := s.db.ListEnabledUsers()
	if err != nil {
		return
	}

	vlessConf, _ := s.db.GetProxyConfig("vless")
	if vlessConf != nil && vlessConf.Installed && s.xray != nil {
		if err := s.xray.GenerateConfig(users); err == nil {
			if s.xray.IsRunning() {
				s.xray.Restart()
			}
		}
	}
}

func generateToken(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
