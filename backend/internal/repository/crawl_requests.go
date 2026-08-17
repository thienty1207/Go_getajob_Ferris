package repository

import (
	"context"
	"errors"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

var (
	ErrCrawlSourceNotFound = errors.New("crawl source not found")
	ErrCrawlSourceInactive = errors.New("crawl source is inactive")
)

type CrawlRequestRepository interface {
	EnqueueCrawlRequest(context.Context, uuid.UUID, string) (model.CrawlRequest, error)
}
