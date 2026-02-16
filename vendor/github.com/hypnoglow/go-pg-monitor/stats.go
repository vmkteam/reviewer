package monitor

// Stats represents go-pg database client stats.
type Stats struct {
	Hits     uint32
	Misses   uint32
	Timeouts uint32

	TotalConns uint32
	IdleConns  uint32
	StaleConns uint32
}

// Observer can provide database client stats.
type Observer interface {
	// Observe returns current database client stats.
	Observe() Stats
}
