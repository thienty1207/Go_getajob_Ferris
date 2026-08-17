package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const crawlerIntervalSettingKey = "crawler.interval_seconds"

type PostgresSettingsRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresSettingsRepository(pool *pgxpool.Pool) *PostgresSettingsRepository {
	return &PostgresSettingsRepository{pool: pool}
}

func (r *PostgresSettingsRepository) GetCrawlerInterval(ctx context.Context) (int64, error) {
	var raw string
	err := r.pool.QueryRow(ctx, `
		SELECT setting_value #>> '{}'
		FROM public.app_settings
		WHERE setting_key = $1`, crawlerIntervalSettingKey).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrSettingNotFound
	}
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seconds <= 0 {
		return 0, errors.New("crawler interval setting is not a positive integer")
	}
	return seconds, nil
}

func (r *PostgresSettingsRepository) SaveCrawlerInterval(ctx context.Context, seconds int64, actor string) error {
	value, err := json.Marshal(seconds)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO public.app_settings (setting_key, setting_group, setting_value, updated_by)
		VALUES ($1, 'crawler', $2::jsonb, $3)
		ON CONFLICT (setting_key) DO UPDATE
		SET setting_group = EXCLUDED.setting_group,
		    setting_value = EXCLUDED.setting_value,
		    updated_at = now(),
		    updated_by = EXCLUDED.updated_by`, crawlerIntervalSettingKey, string(value), actor)
	return err
}

func (r *PostgresSettingsRepository) GetCrawlerRuntime(ctx context.Context) (model.CrawlerRuntime, error) {
	var runtime model.CrawlerRuntime
	err := r.pool.QueryRow(ctx, `
		SELECT status,
		       last_heartbeat_at,
		       last_cycle_started_at,
		       last_cycle_finished_at,
		       next_cycle_at,
		       current_source_key,
		       last_error_code
		FROM public.crawler_runtime
		WHERE runtime_key = 'default'`).Scan(
		&runtime.Status,
		&runtime.LastHeartbeatAt,
		&runtime.LastCycleStartedAt,
		&runtime.LastCycleFinishedAt,
		&runtime.NextCycleAt,
		&runtime.CurrentSourceKey,
		&runtime.LastErrorCode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.CrawlerRuntime{}, ErrSettingNotFound
	}
	return runtime, err
}

var _ SettingsRepository = (*PostgresSettingsRepository)(nil)
