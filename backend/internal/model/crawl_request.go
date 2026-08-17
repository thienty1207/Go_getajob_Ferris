package model

import (
	"time"

	"github.com/google/uuid"
)

// CrawlRequest represents one durable manual-crawl request. It deliberately
// contains only queue metadata; crawl output remains in source_crawl_runs and
// job_cache, where the existing retention and privacy rules apply.
type CrawlRequest struct {
	ID          uuid.UUID
	SourceID    uuid.UUID
	Status      string
	RequestedBy string
	RequestedAt time.Time
	StartedAt   *time.Time
	FinishedAt  *time.Time
	SourceRunID *uuid.UUID
	ErrorCode   *string
}
