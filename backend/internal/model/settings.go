package model

import "time"

// CrawlerSettings is the operator-facing representation of the persisted
// crawler interval. The database stores seconds so the scheduler has one
// unambiguous value; the admin UI receives hours and minutes for editing.
type CrawlerSettings struct {
	IntervalHours      int   `json:"interval_hours"`
	IntervalMinutes    int   `json:"interval_minutes"`
	IntervalSeconds    int64 `json:"interval_seconds"`
	MinIntervalMinutes int   `json:"min_interval_minutes"`
	MaxIntervalMinutes int   `json:"max_interval_minutes"`
}

type CrawlerRuntime struct {
	Status              string     `json:"status"`
	LastHeartbeatAt     *time.Time `json:"last_heartbeat_at,omitempty"`
	LastCycleStartedAt  *time.Time `json:"last_cycle_started_at,omitempty"`
	LastCycleFinishedAt *time.Time `json:"last_cycle_finished_at,omitempty"`
	NextCycleAt         *time.Time `json:"next_cycle_at,omitempty"`
	CurrentSourceKey    *string    `json:"current_source_key,omitempty"`
	LastErrorCode       *string    `json:"last_error_code,omitempty"`
}
