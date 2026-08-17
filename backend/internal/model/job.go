package model

import (
	"time"

	"github.com/google/uuid"
)

// AdminJob is a structured job-cache row for the operational console. Raw
// descriptions are intentionally absent because the database stores metadata
// and parsed fields only.
type AdminJob struct {
	ID                   uuid.UUID
	SourceKey            string
	SourceName           string
	SourceApprovalStatus string
	Title                string
	Company              string
	Location             string
	LocationID           *uuid.UUID
	Role                 string
	RequiredSkills       []string
	PreferredSkills      []string
	Seniority            string
	MinimumExperience    *float64
	Domains              []string
	EmploymentType       string
	WorkMode             string
	Status               string
	OriginalURL          string
	ContentHash          string
	LastSeenAt           time.Time
	UpdatedAt            time.Time
}

// AdminJobPage makes pagination explicit so the frontend never has to infer
// whether an empty response means no jobs or a failed request.
type AdminJobPage struct {
	Items    []AdminJob
	Page     int
	PageSize int
	Total    int
}
