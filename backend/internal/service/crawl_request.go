package service

import (
	"context"
	"errors"
	"strings"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrInvalidCrawlRequestSource = errors.New("invalid crawl request source")
	ErrInvalidCrawlRequestActor  = errors.New("invalid crawl request actor")
)

type CrawlRequestService struct {
	repository repository.CrawlRequestRepository
}

func NewCrawlRequestService(crawlRequestRepository repository.CrawlRequestRepository) *CrawlRequestService {
	return &CrawlRequestService{repository: crawlRequestRepository}
}

func (s *CrawlRequestService) Request(ctx context.Context, sourceID uuid.UUID, actor string) (model.CrawlRequest, error) {
	if sourceID == uuid.Nil {
		return model.CrawlRequest{}, ErrInvalidCrawlRequestSource
	}
	actor = strings.TrimSpace(actor)
	if actor == "" || len(actor) > 320 || strings.ContainsAny(actor, "\r\n") {
		return model.CrawlRequest{}, ErrInvalidCrawlRequestActor
	}
	return s.repository.EnqueueCrawlRequest(ctx, sourceID, actor)
}

var _ repository.CrawlRequestRepository = (*repository.PostgresCrawlRequestRepository)(nil)
