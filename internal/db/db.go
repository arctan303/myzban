package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

const defaultDBPath = "/etc/pnm/pnm.db"

// DB wraps the SQLite connection
type DB struct {
	conn *sql.DB
	path string
}

// Open opens (or creates) the SQLite database
func Open(path string) (*DB, error) {
	if path == "" {
		path = defaultDBPath
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	d := &DB{conn: conn, path: path}
	if err := d.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return d, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.conn.Close()
}

// Conn returns the raw sql.DB for advanced queries
func (d *DB) Conn() *sql.DB {
	return d.conn
}

func (d *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			uuid TEXT UNIQUE NOT NULL,
			hy2_password TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			traffic_up INTEGER NOT NULL DEFAULT 0,
			traffic_down INTEGER NOT NULL DEFAULT 0,
			traffic_limit INTEGER NOT NULL DEFAULT 0,
			expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS traffic_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			protocol TEXT NOT NULL,
			upload INTEGER NOT NULL DEFAULT 0,
			download INTEGER NOT NULL DEFAULT 0,
			record_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS proxy_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			protocol TEXT UNIQUE NOT NULL,
			port INTEGER NOT NULL DEFAULT 0,
			installed INTEGER NOT NULL DEFAULT 0,
			extra_json TEXT NOT NULL DEFAULT '{}'
		)`,
		`CREATE TABLE IF NOT EXISTS node_info (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			server_ip TEXT NOT NULL DEFAULT '',
			xray_pub_key TEXT NOT NULL DEFAULT '',
			xray_pri_key TEXT NOT NULL DEFAULT '',
			short_id TEXT NOT NULL DEFAULT '',
			hy2_cert TEXT NOT NULL DEFAULT '',
			hy2_key TEXT NOT NULL DEFAULT ''
		)`,
		`INSERT OR IGNORE INTO node_info (id) VALUES (1)`,
		`INSERT OR IGNORE INTO proxy_configs (protocol, port) VALUES ('vless', 443)`,
		`INSERT OR IGNORE INTO proxy_configs (protocol, port) VALUES ('hysteria2', 8443)`,
		`CREATE INDEX IF NOT EXISTS idx_traffic_logs_user ON traffic_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_traffic_logs_time ON traffic_logs(record_at)`,
	}

	for _, m := range migrations {
		if _, err := d.conn.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %s: %w", m[:60], err)
		}
	}
	return nil
}

// --- User CRUD ---

func (d *DB) CreateUser(u *User) error {
	res, err := d.conn.Exec(
		`INSERT INTO users (username, email, uuid, hy2_password, enabled, traffic_limit, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.Username, u.Email, u.UUID, u.Hy2Password, u.Enabled, u.TrafficLimit, u.ExpiresAt,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	u.ID = id
	return nil
}

func (d *DB) GetUser(id int64) (*User, error) {
	u := &User{}
	err := d.conn.QueryRow(
		`SELECT id, username, email, uuid, hy2_password, enabled,
		        traffic_up, traffic_down, traffic_limit, expires_at, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.UUID, &u.Hy2Password, &u.Enabled,
		&u.TrafficUp, &u.TrafficDown, &u.TrafficLimit, &u.ExpiresAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (d *DB) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := d.conn.QueryRow(
		`SELECT id, username, email, uuid, hy2_password, enabled,
		        traffic_up, traffic_down, traffic_limit, expires_at, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.UUID, &u.Hy2Password, &u.Enabled,
		&u.TrafficUp, &u.TrafficDown, &u.TrafficLimit, &u.ExpiresAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (d *DB) ListUsers() ([]*User, error) {
	rows, err := d.conn.Query(
		`SELECT id, username, email, uuid, hy2_password, enabled,
		        traffic_up, traffic_down, traffic_limit, expires_at, created_at, updated_at
		 FROM users ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.UUID, &u.Hy2Password, &u.Enabled,
			&u.TrafficUp, &u.TrafficDown, &u.TrafficLimit, &u.ExpiresAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (d *DB) ListEnabledUsers() ([]*User, error) {
	rows, err := d.conn.Query(
		`SELECT id, username, email, uuid, hy2_password, enabled,
		        traffic_up, traffic_down, traffic_limit, expires_at, created_at, updated_at
		 FROM users WHERE enabled = 1 ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.UUID, &u.Hy2Password, &u.Enabled,
			&u.TrafficUp, &u.TrafficDown, &u.TrafficLimit, &u.ExpiresAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (d *DB) UpdateUser(u *User) error {
	_, err := d.conn.Exec(
		`UPDATE users SET username=?, email=?, uuid=?, hy2_password=?, enabled=?,
		 traffic_limit=?, expires_at=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=?`,
		u.Username, u.Email, u.UUID, u.Hy2Password, u.Enabled,
		u.TrafficLimit, u.ExpiresAt, u.ID,
	)
	return err
}

func (d *DB) SetUserEnabled(id int64, enabled bool) error {
	_, err := d.conn.Exec(
		`UPDATE users SET enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		enabled, id,
	)
	return err
}

func (d *DB) DeleteUser(id int64) error {
	_, err := d.conn.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

func (d *DB) AddTraffic(id int64, up, down int64) error {
	_, err := d.conn.Exec(
		`UPDATE users SET traffic_up = traffic_up + ?, traffic_down = traffic_down + ?,
		 updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		up, down, id,
	)
	return err
}

func (d *DB) ResetTraffic(id int64) error {
	_, err := d.conn.Exec(
		`UPDATE users SET traffic_up = 0, traffic_down = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id,
	)
	return err
}

// --- Traffic Log ---

func (d *DB) InsertTrafficLog(log *TrafficLog) error {
	_, err := d.conn.Exec(
		`INSERT INTO traffic_logs (user_id, protocol, upload, download, record_at)
		 VALUES (?, ?, ?, ?, ?)`,
		log.UserID, log.Protocol, log.Upload, log.Download, log.RecordAt,
	)
	return err
}

func (d *DB) GetTrafficLogs(userID int64, limit int) ([]*TrafficLog, error) {
	rows, err := d.conn.Query(
		`SELECT id, user_id, protocol, upload, download, record_at
		 FROM traffic_logs WHERE user_id = ? ORDER BY record_at DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*TrafficLog
	for rows.Next() {
		l := &TrafficLog{}
		if err := rows.Scan(&l.ID, &l.UserID, &l.Protocol, &l.Upload, &l.Download, &l.RecordAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

// --- Node Info ---

func (d *DB) GetNodeInfo() (*NodeInfo, error) {
	n := &NodeInfo{}
	err := d.conn.QueryRow(
		`SELECT id, server_ip, xray_pub_key, xray_pri_key, short_id, hy2_cert, hy2_key
		 FROM node_info WHERE id = 1`,
	).Scan(&n.ID, &n.ServerIP, &n.XrayPubKey, &n.XrayPriKey, &n.ShortID, &n.Hy2Cert, &n.Hy2Key)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (d *DB) SaveNodeInfo(n *NodeInfo) error {
	_, err := d.conn.Exec(
		`UPDATE node_info SET server_ip=?, xray_pub_key=?, xray_pri_key=?, short_id=?, hy2_cert=?, hy2_key=?
		 WHERE id = 1`,
		n.ServerIP, n.XrayPubKey, n.XrayPriKey, n.ShortID, n.Hy2Cert, n.Hy2Key,
	)
	return err
}

// --- Proxy Config ---

func (d *DB) GetProxyConfig(protocol string) (*ProxyConfig, error) {
	p := &ProxyConfig{}
	err := d.conn.QueryRow(
		`SELECT id, protocol, port, installed, extra_json FROM proxy_configs WHERE protocol = ?`,
		protocol,
	).Scan(&p.ID, &p.Protocol, &p.Port, &p.Installed, &p.ExtraJSON)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (d *DB) SaveProxyConfig(p *ProxyConfig) error {
	_, err := d.conn.Exec(
		`UPDATE proxy_configs SET port=?, installed=?, extra_json=? WHERE protocol=?`,
		p.Port, p.Installed, p.ExtraJSON, p.Protocol,
	)
	return err
}

// FindUserByHy2Credentials looks up a user by hysteria2 username+password for HTTP auth
func (d *DB) FindUserByHy2Credentials(username, password string) (*User, error) {
	u := &User{}
	err := d.conn.QueryRow(
		`SELECT id, username, email, uuid, hy2_password, enabled,
		        traffic_up, traffic_down, traffic_limit, expires_at, created_at, updated_at
		 FROM users WHERE username = ? AND hy2_password = ? AND enabled = 1`, username, password,
	).Scan(&u.ID, &u.Username, &u.Email, &u.UUID, &u.Hy2Password, &u.Enabled,
		&u.TrafficUp, &u.TrafficDown, &u.TrafficLimit, &u.ExpiresAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// FindEnabledUserByHy2Auth is used by the Hy2 HTTP auth endpoint.
// Hy2 sends the password in "auth" field; we use username:password format or just password.
func (d *DB) FindEnabledUserByHy2Auth(authStr string) (*User, error) {
	u := &User{}
	err := d.conn.QueryRow(
		`SELECT id, username, email, uuid, hy2_password, enabled,
		        traffic_up, traffic_down, traffic_limit, expires_at, created_at, updated_at
		 FROM users WHERE hy2_password = ? AND enabled = 1`, authStr,
	).Scan(&u.ID, &u.Username, &u.Email, &u.UUID, &u.Hy2Password, &u.Enabled,
		&u.TrafficUp, &u.TrafficDown, &u.TrafficLimit, &u.ExpiresAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}
