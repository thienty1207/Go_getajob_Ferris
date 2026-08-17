package repository

import (
	"context"
	"errors"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

var ErrJobLinkNotFound = errors.New("job link not found")
var ErrJobLinkConflict = errors.New("job link already exists")

type JobLinkWrite struct {
	ID             uuid.UUID
	SourceKey      string
	DisplayName    string
	BaseURL        string
	SourceType     string
	ApprovalStatus string
	ApprovedAt     *time.Time
	ApprovedBy     *string
}

type JobLinkRepository interface {
	ListJobLinks(context.Context, int, int) (model.JobLinkPage, error)
	CreateJobLink(context.Context, JobLinkWrite) (model.JobLink, error)
	UpdateJobLink(context.Context, JobLinkWrite) (model.JobLink, error)
	SetJobLinkStatus(context.Context, uuid.UUID, string) error
	DeleteJobLink(context.Context, uuid.UUID) error
}
