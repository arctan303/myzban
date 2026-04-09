package proxy

import "github.com/pnm/proxy-node-manager/internal/db"

// Manager is the interface for managing a proxy protocol
type Manager interface {
	// GenerateConfig writes the proxy config file based on current enabled users
	GenerateConfig(users []*db.User) error

	// Start starts the proxy service
	Start() error

	// Stop stops the proxy service
	Stop() error

	// Restart restarts the proxy service
	Restart() error

	// IsRunning checks if the proxy service is active
	IsRunning() bool

	// AddUser adds a user to the running proxy (hot-reload if possible)
	AddUser(user *db.User) error

	// RemoveUser removes a user from the running proxy
	RemoveUser(user *db.User) error

	// GetTrafficStats returns a map of user-identifier -> {upload, download}
	GetTrafficStats(reset bool) (map[string]*TrafficData, error)

	// GenerateClientConfig generates per-user client config YAML
	GenerateClientConfig(user *db.User, nodeInfo *db.NodeInfo) string
}

// TrafficData holds upload/download byte counts
type TrafficData struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}
