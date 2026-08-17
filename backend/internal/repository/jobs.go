package repository

import (
	"context"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

type AdminJobFilter struct {
	LocationID         *uuid.UUID
	UnresolvedLocation bool
	Search             string
}

// JobRepository exposes only the structured operational view of cached jobs.
// Public client reads use the source-approved active_job_cache view instead.
type JobRepository interface {
	ListAdminJobs(context.Context, int, int, AdminJobFilter) (model.AdminJobPage, error)
}
