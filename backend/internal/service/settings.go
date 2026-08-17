package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/repository"
)

const (
	DefaultCrawlerIntervalSeconds int64         = 6 * 60 * 60
	MinCrawlerIntervalMinutes     int           = 15
	MaxCrawlerIntervalMinutes     int           = 7 * 24 * 60
	CrawlerHeartbeatTimeout       time.Duration = 15 * time.Second
)

var (
	ErrInvalidCrawlerInterval = errors.New("invalid crawler interval")
	ErrInvalidSettingsActor   = errors.New("invalid settings actor")
)

type SettingsService struct {
	repository repository.SettingsRepository
}

func NewSettingsService(settingsRepository repository.SettingsRepository) *SettingsService {
	return &SettingsService{repository: settingsRepository}
}

func (s *SettingsService) GetCrawlerSettings(ctx context.Context) (model.CrawlerSettings, error) {
	seconds, err := s.repository.GetCrawlerInterval(ctx)
	if errors.Is(err, repository.ErrSettingNotFound) {
		seconds = DefaultCrawlerIntervalSeconds
	} else if err != nil {
		return model.CrawlerSettings{}, err
	}
	if !validCrawlerIntervalSeconds(seconds) {
		return model.CrawlerSettings{}, ErrInvalidCrawlerInterval
	}
	return CrawlerSettingsFromSeconds(seconds), nil
}

func (s *SettingsService) GetCrawlerRuntime(ctx context.Context) (model.CrawlerRuntime, error) {
	runtime, err := s.repository.GetCrawlerRuntime(ctx)
	if errors.Is(err, repository.ErrSettingNotFound) {
		return model.CrawlerRuntime{Status: "OFFLINE"}, nil
	}
	if err != nil {
		return model.CrawlerRuntime{}, err
	}
	if runtime.LastHeartbeatAt == nil || time.Since(runtime.LastHeartbeatAt.UTC()) > CrawlerHeartbeatTimeout {
		runtime.Status = "OFFLINE"
	}
	return runtime, nil
}

func (s *SettingsService) UpdateCrawlerSettings(ctx context.Context, hours, minutes int, actor string) (model.CrawlerSettings, error) {
	seconds, err := NormalizeCrawlerInterval(hours, minutes)
	if err != nil {
		return model.CrawlerSettings{}, err
	}
	actor = strings.TrimSpace(actor)
	if actor == "" || len(actor) > 320 || strings.ContainsAny(actor, "\r\n") {
		return model.CrawlerSettings{}, ErrInvalidSettingsActor
	}
	if err := s.repository.SaveCrawlerInterval(ctx, seconds, actor); err != nil {
		return model.CrawlerSettings{}, err
	}
	return CrawlerSettingsFromSeconds(seconds), nil
}

func NormalizeCrawlerInterval(hours, minutes int) (int64, error) {
	if hours < 0 || minutes < 0 || minutes > 59 {
		return 0, ErrInvalidCrawlerInterval
	}
	if hours > math.MaxInt64/3600 {
		return 0, ErrInvalidCrawlerInterval
	}
	seconds := int64(hours)*3600 + int64(minutes)*60
	minimum := int64(MinCrawlerIntervalMinutes) * 60
	maximum := int64(MaxCrawlerIntervalMinutes) * 60
	if seconds < minimum || seconds > maximum {
		return 0, ErrInvalidCrawlerInterval
	}
	return seconds, nil
}

func CrawlerSettingsFromSeconds(seconds int64) model.CrawlerSettings {
	return model.CrawlerSettings{
		IntervalHours:      int(seconds / 3600),
		IntervalMinutes:    int((seconds % 3600) / 60),
		IntervalSeconds:    seconds,
		MinIntervalMinutes: MinCrawlerIntervalMinutes,
		MaxIntervalMinutes: MaxCrawlerIntervalMinutes,
	}
}

func validCrawlerIntervalSeconds(seconds int64) bool {
	minimum := int64(MinCrawlerIntervalMinutes) * 60
	maximum := int64(MaxCrawlerIntervalMinutes) * 60
	return seconds >= minimum && seconds <= maximum
}

var _ repository.SettingsRepository = (*repository.PostgresSettingsRepository)(nil)
