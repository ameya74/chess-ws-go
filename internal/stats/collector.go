package stats

import (
	"log"
	"sync"
	"time"
)

// Stats holds the server statistics
type Stats struct {
	ActiveConnections int
	ActiveGames       int
	TotalRequests     uint64
	StartTime         time.Time
	mu                sync.RWMutex
}

// Collector manages server statistics
type Collector struct {
	stats    *Stats
	interval time.Duration
	getGames func() int // Callback to get current number of games
	getConns func() int // Callback to get current number of connections
}

// NewCollector creates a new statistics collector
func NewCollector(interval time.Duration, getGames, getConns func() int) *Collector {
	return &Collector{
		stats: &Stats{
			StartTime: time.Now(),
		},
		interval: interval,
		getGames: getGames,
		getConns: getConns,
	}
}

// Start begins periodic collection of server statistics
func (c *Collector) Start() {
	ticker := time.NewTicker(c.interval)
	go func() {
		for range ticker.C {
			c.collect()
		}
	}()
}

// collect gathers current statistics
func (c *Collector) collect() {
	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()

	c.stats.ActiveGames = c.getGames()
	c.stats.ActiveConnections = c.getConns()

	// Log current stats
	log.Printf("Server Stats - Active Connections: %d, Active Games: %d, Uptime: %v",
		c.stats.ActiveConnections,
		c.stats.ActiveGames,
		time.Since(c.stats.StartTime))
}

// GetStats returns a copy of current statistics
func (c *Collector) GetStats() Stats {
	c.stats.mu.RLock()
	defer c.stats.mu.RUnlock()
	return *c.stats
}

// IncrementRequests increases the total request counter
func (c *Collector) IncrementRequests() {
	c.stats.mu.Lock()
	c.stats.TotalRequests++
	c.stats.mu.Unlock()
}
