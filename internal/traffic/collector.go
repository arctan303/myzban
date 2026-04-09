package traffic

import (
	"fmt"
	"log"
	"time"

	"github.com/pnm/proxy-node-manager/internal/db"
	"github.com/pnm/proxy-node-manager/internal/proxy"
)

// Collector periodically polls proxy stats APIs and accumulates traffic data
type Collector struct {
	db       *db.DB
	xray     *proxy.XrayManager
	hy2      *proxy.Hy2Manager
	interval time.Duration
	stopCh   chan struct{}
}

// NewCollector creates a new traffic collector
func NewCollector(database *db.DB, xray *proxy.XrayManager, hy2 *proxy.Hy2Manager, intervalSec int) *Collector {
	return &Collector{
		db:       database,
		xray:     xray,
		hy2:      hy2,
		interval: time.Duration(intervalSec) * time.Second,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic collection loop in a goroutine
func (c *Collector) Start() {
	go c.loop()
	log.Printf("[traffic] collector started (interval: %s)", c.interval)
}

// Stop signals the collector to stop
func (c *Collector) Stop() {
	close(c.stopCh)
}

func (c *Collector) loop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			log.Println("[traffic] collector stopped")
			return
		case <-ticker.C:
			c.collect()
		}
	}
}

func (c *Collector) collect() {
	// Build email->user and hy2pass->user maps
	users, err := c.db.ListUsers()
	if err != nil {
		log.Printf("[traffic] list users error: %v", err)
		return
	}

	emailMap := make(map[string]*db.User)
	hy2Map := make(map[string]*db.User)
	for _, u := range users {
		emailMap[u.Email] = u
		hy2Map[u.Username] = u // Hy2 HTTP auth returns username as ID
	}

	// Collect Xray stats
	if c.xray != nil && c.xray.IsRunning() {
		c.collectXray(emailMap)
	}

	// Collect Hy2 stats
	if c.hy2 != nil && c.hy2.IsRunning() {
		c.collectHy2(hy2Map)
	}

	// Check for over-limit users
	c.checkLimits(users)
}

func (c *Collector) collectXray(emailMap map[string]*db.User) {
	stats, err := c.xray.GetTrafficStats(true)
	if err != nil {
		log.Printf("[traffic] xray stats error: %v", err)
		return
	}

	now := time.Now()
	for email, data := range stats {
		user, ok := emailMap[email]
		if !ok || (data.Upload == 0 && data.Download == 0) {
			continue
		}

		// Add to cumulative total
		if err := c.db.AddTraffic(user.ID, data.Upload, data.Download); err != nil {
			log.Printf("[traffic] add traffic for %s: %v", user.Username, err)
			continue
		}

		// Write log entry
		c.db.InsertTrafficLog(&db.TrafficLog{
			UserID:   user.ID,
			Protocol: "vless",
			Upload:   data.Upload,
			Download: data.Download,
			RecordAt: now,
		})
	}
}

func (c *Collector) collectHy2(hy2Map map[string]*db.User) {
	stats, err := c.hy2.GetTrafficStats(true)
	if err != nil {
		log.Printf("[traffic] hy2 stats error: %v", err)
		return
	}

	now := time.Now()
	for id, data := range stats {
		user, ok := hy2Map[id]
		if !ok || (data.Upload == 0 && data.Download == 0) {
			continue
		}

		if err := c.db.AddTraffic(user.ID, data.Upload, data.Download); err != nil {
			log.Printf("[traffic] add traffic for %s: %v", user.Username, err)
			continue
		}

		c.db.InsertTrafficLog(&db.TrafficLog{
			UserID:   user.ID,
			Protocol: "hysteria2",
			Upload:   data.Upload,
			Download: data.Download,
			RecordAt: now,
		})
	}
}

func (c *Collector) checkLimits(users []*db.User) {
	for _, u := range users {
		if !u.Enabled {
			continue
		}

		// Check traffic limit
		if u.TrafficLimit > 0 {
			total := u.TrafficUp + u.TrafficDown
			if total >= u.TrafficLimit {
				log.Printf("[traffic] user %s exceeded traffic limit (%s >= %s), disabling",
					u.Username, formatBytes(total), formatBytes(u.TrafficLimit))
				c.db.SetUserEnabled(u.ID, false)
				// Remove from proxies
				if c.xray != nil {
					c.xray.RemoveUser(u)
				}
				if c.hy2 != nil {
					c.hy2.RemoveUser(u)
				}
			}
		}

		// Check expiration
		if u.ExpiresAt != nil && time.Now().After(*u.ExpiresAt) {
			log.Printf("[traffic] user %s expired, disabling", u.Username)
			c.db.SetUserEnabled(u.ID, false)
			if c.xray != nil {
				c.xray.RemoveUser(u)
			}
			if c.hy2 != nil {
				c.hy2.RemoveUser(u)
			}
		}
	}
}

func formatBytes(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
