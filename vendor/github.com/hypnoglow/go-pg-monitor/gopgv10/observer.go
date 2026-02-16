package gopgv10

import (
	"github.com/go-pg/pg/v10"

	monitor "github.com/hypnoglow/go-pg-monitor"
)

// Observer is an observer for go-pg v10.
type Observer struct {
	db *pg.DB
}

// NewObserver returns a new Observer for db.
func NewObserver(db *pg.DB) *Observer {
	return &Observer{db: db}
}

func (o *Observer) Observe() monitor.Stats {
	stats := o.db.PoolStats()
	return monitor.Stats{
		Hits:       stats.Hits,
		Misses:     stats.Misses,
		Timeouts:   stats.Timeouts,
		TotalConns: stats.TotalConns,
		IdleConns:  stats.IdleConns,
		StaleConns: stats.StaleConns,
	}
}
