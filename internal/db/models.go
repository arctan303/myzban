package db

import "time"

// User represents a proxy user
type User struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`         // Xray uses email as user identifier
	UUID         string     `json:"uuid"`           // VLESS UUID
	Hy2Password  string     `json:"hy2_password"`   // Hysteria2 password
	SubToken     string     `json:"sub_token"`      // subscription token for fetching config
	Enabled      bool       `json:"enabled"`
	TrafficUp    int64      `json:"traffic_up"`     // cumulative upload bytes
	TrafficDown  int64      `json:"traffic_down"`   // cumulative download bytes
	TrafficLimit int64      `json:"traffic_limit"`  // traffic cap (0 = unlimited)
	ExpiresAt    *time.Time `json:"expires_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TrafficLog stores periodic traffic snapshots per user
type TrafficLog struct {
	ID       int64     `json:"id"`
	UserID   int64     `json:"user_id"`
	Protocol string    `json:"protocol"` // "vless" | "hysteria2"
	Upload   int64     `json:"upload"`
	Download int64     `json:"download"`
	RecordAt time.Time `json:"record_at"`
}

// ProxyConfig stores per-protocol installation and runtime config
type ProxyConfig struct {
	ID        int64  `json:"id"`
	Protocol  string `json:"protocol"` // "vless" | "hysteria2"
	Port      int    `json:"port"`
	Installed bool   `json:"installed"`
	ExtraJSON string `json:"extra_json"` // protocol-specific config as JSON blob
}

// NodeInfo stores this node's identity and key information
type NodeInfo struct {
	ID         int64  `json:"id"`
	ServerIP   string `json:"server_ip"`
	XrayPubKey string `json:"xray_pub_key"` // X25519 public key
	XrayPriKey string `json:"-"`            // X25519 private key (hidden from JSON)
	ShortID    string `json:"short_id"`     // Reality short id
	Hy2Cert    string `json:"hy2_cert"`     // certificate path
	Hy2Key     string `json:"-"`            // key path (hidden from JSON)
	AdminToken string `json:"-"`            // admin API token (hidden from JSON)
}

