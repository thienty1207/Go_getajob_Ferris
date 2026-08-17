package model

import (
	"time"

	"github.com/google/uuid"
)

type JobLink struct {
	ID                       uuid.UUID
	SourceKey                string
	DisplayName              string
	URL                      string
	ApprovalStatus           string
	ApprovedAt               *time.Time
	ApprovedBy               *string
	CreatedAt                time.Time
	UpdatedAt                time.Time
	LastCrawlStatus          *string
	LastCrawlAt              *time.Time
	ActiveCrawlRequestID     *uuid.UUID
	ActiveCrawlRequestStatus *string
	LastCrawlPages           int
	LastCrawlJobs            int
	LastCrawlCreated         int
	LastCrawlUpdated         int
	LastCrawlMissing         int
	LastCrawlErrorCode       *string
}

type JobLinkPage struct {
	Items    []JobLink
	Page     int
	PageSize int
	Total    int
}
