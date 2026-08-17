package repository

import (
	"context"
	"errors"

	"github.com/gogetsomefoodferris/backend/internal/model"
)

var ErrSettingNotFound = errors.New("setting not found")

// SettingsRepository owns the typed persistence boundary for global product
// settings. A new setting should get its own service validation rather than
// allowing arbitrary JSON writes from the admin API.
type SettingsRepository interface {
	GetCrawlerInterval(context.Context) (int64, error)
	SaveCrawlerInterval(context.Context, int64, string) error
	GetCrawlerRuntime(context.Context) (model.CrawlerRuntime, error)
}
