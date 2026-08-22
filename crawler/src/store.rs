use chrono::{DateTime, Utc};
use thiserror::Error;
use tokio_postgres::{Client, Error as PostgresError, GenericClient};
use uuid::Uuid;

use crate::crawl::{CrawlReport, StructuredJobObservation};
use crate::location::normalize_location_text;
use crate::reconcile::RunStatus;
use crate::scope::SourceScope;

#[derive(Debug, Clone)]
pub struct ActiveSource {
    pub id: Uuid,
    pub source_key: String,
    pub base_url: String,
    pub robots_url: Option<String>,
}

#[derive(Debug, Clone)]
pub struct ClaimedCrawlRequest {
    pub id: Uuid,
    pub source: ActiveSource,
}

#[derive(Debug, Clone, Copy)]
pub struct CrawlRun {
    pub id: Uuid,
    pub started_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct DeltaCounts {
    pub jobs_created: i32,
    pub jobs_updated: i32,
}

#[derive(Debug, Error)]
pub enum StorageError {
    #[error(transparent)]
    Postgres(#[from] PostgresError),
    #[error("structured observation violates the approved source scope")]
    ObservationOutsideScope,
    #[error("structured observation is incomplete or invalid")]
    InvalidObservation,
    #[error("healthy reconciliation requires an authoritative observation batch")]
    ReconciliationNotAuthorized,
    #[error("approved Job Link changed or is no longer ACTIVE during crawl persistence")]
    SourceSnapshotChanged,
    #[error("structured observation has an invalid location text")]
    InvalidLocationText,
}

pub struct AuthoritativeObservationBatch {
    observations: Vec<StructuredJobObservation>,
}

impl AuthoritativeObservationBatch {
    pub fn from_report(report: &mut CrawlReport) -> Result<Self, StorageError> {
        if report.run_status != RunStatus::Healthy || !report.reconciliation_safe {
            return Err(StorageError::ReconciliationNotAuthorized);
        }
        Ok(Self {
            observations: std::mem::take(&mut report.observations),
        })
    }
}

pub async fn load_active_sources(client: &Client) -> Result<Vec<ActiveSource>, PostgresError> {
    let rows = client
        .query(
            "SELECT id, source_key, base_url, robots_url FROM public.job_sources WHERE approval_status = 'ACTIVE' ORDER BY source_key",
            &[],
        )
        .await?;
    Ok(rows
        .into_iter()
        .map(|row| ActiveSource {
            id: row.get("id"),
            source_key: row.get("source_key"),
            base_url: row.get("base_url"),
            robots_url: row.get("robots_url"),
        })
        .collect())
}

pub async fn load_crawler_interval_setting(
    client: &Client,
) -> Result<Option<String>, PostgresError> {
    let row = client
        .query_opt(
            "SELECT setting_value #>> '{}' FROM public.app_settings WHERE setting_key = 'crawler.interval_seconds'",
            &[],
        )
        .await?;
    Ok(row.map(|row| row.get(0)))
}

pub async fn heartbeat_runtime<C>(client: &C) -> Result<(), PostgresError>
where
    C: GenericClient + Sync,
{
    client
        .execute(
            "UPDATE public.crawler_runtime
			 SET last_heartbeat_at = now(),
			     status = CASE WHEN status = 'OFFLINE' THEN 'IDLE' ELSE status END,
			     updated_at = now()
			 WHERE runtime_key = 'default'",
            &[],
        )
        .await?;
    Ok(())
}

pub async fn mark_runtime_cycle_started<C>(client: &C) -> Result<(), PostgresError>
where
    C: GenericClient + Sync,
{
    client
        .execute(
            "UPDATE public.crawler_runtime
			 SET status = 'RUNNING',
			     last_heartbeat_at = now(),
			     last_cycle_started_at = now(),
			     next_cycle_at = NULL,
			     current_source_key = NULL,
			     last_error_code = NULL,
			     updated_at = now()
			 WHERE runtime_key = 'default'",
            &[],
        )
        .await?;
    Ok(())
}

pub async fn mark_runtime_source<C>(client: &C, source_key: &str) -> Result<(), PostgresError>
where
    C: GenericClient + Sync,
{
    client
        .execute(
            "UPDATE public.crawler_runtime
			 SET status = 'RUNNING',
			     last_heartbeat_at = now(),
			     current_source_key = $1,
			     updated_at = now()
			 WHERE runtime_key = 'default'",
            &[&source_key],
        )
        .await?;
    Ok(())
}

pub async fn mark_runtime_cycle_finished<C>(
    client: &C,
    next_cycle_at: Option<DateTime<Utc>>,
) -> Result<(), PostgresError>
where
    C: GenericClient + Sync,
{
    let status = "IDLE";
    let error_code: Option<&str> = None;
    client
        .execute(
            "UPDATE public.crawler_runtime
			 SET status = $1,
			     last_heartbeat_at = now(),
			     last_cycle_finished_at = now(),
			     next_cycle_at = $2,
			     current_source_key = NULL,
			     last_error_code = $3,
			     updated_at = now()
			 WHERE runtime_key = 'default'",
            &[&status, &next_cycle_at, &error_code],
        )
        .await?;
    Ok(())
}

pub async fn mark_runtime_error<C>(
    client: &C,
    next_cycle_at: Option<DateTime<Utc>>,
    error_code: &str,
) -> Result<(), PostgresError>
where
    C: GenericClient + Sync,
{
    let status = "ERROR";
    client
        .execute(
            "UPDATE public.crawler_runtime
			 SET status = $1,
			     last_heartbeat_at = now(),
			     last_cycle_finished_at = now(),
			     next_cycle_at = $2,
			     current_source_key = NULL,
			     last_error_code = $3,
			     updated_at = now()
			 WHERE runtime_key = 'default'",
            &[&status, &next_cycle_at, &error_code],
        )
        .await?;
    Ok(())
}

pub async fn mark_inactive_crawl_requests(client: &Client) -> Result<u64, PostgresError> {
    client
        .execute(
            "UPDATE public.crawl_requests AS requests
             SET status = 'FAILED', finished_at = now(), error_code = 'source_inactive'
             WHERE requests.status = 'PENDING'
               AND NOT EXISTS (
                   SELECT 1 FROM public.job_sources AS sources
                   WHERE sources.id = requests.source_id
                     AND sources.approval_status = 'ACTIVE'
               )",
            &[],
        )
        .await
}

pub async fn claim_next_crawl_request(
    client: &Client,
) -> Result<Option<ClaimedCrawlRequest>, PostgresError> {
    let row = client
        .query_opt(
            "WITH candidate AS (
                 SELECT requests.id
                 FROM public.crawl_requests AS requests
                 JOIN public.job_sources AS sources ON sources.id = requests.source_id
                 WHERE requests.status = 'PENDING'
                   AND sources.approval_status = 'ACTIVE'
                 ORDER BY requests.requested_at, requests.id
                 FOR UPDATE OF requests SKIP LOCKED
                 LIMIT 1
             )
             UPDATE public.crawl_requests AS requests
             SET status = 'RUNNING', started_at = now()
             FROM candidate, public.job_sources AS sources
             WHERE requests.id = candidate.id
               AND sources.id = requests.source_id
             RETURNING requests.id, requests.source_id, sources.source_key, sources.base_url, sources.robots_url",
            &[],
        )
        .await?;
    Ok(row.map(|row| ClaimedCrawlRequest {
        id: row.get("id"),
        source: ActiveSource {
            id: row.get("source_id"),
            source_key: row.get("source_key"),
            base_url: row.get("base_url"),
            robots_url: row.get("robots_url"),
        },
    }))
}

pub async fn attach_crawl_request_run(
    client: &Client,
    request_id: Uuid,
    run_id: Uuid,
) -> Result<(), PostgresError> {
    client
        .execute(
            "UPDATE public.crawl_requests
             SET source_run_id = $2
             WHERE id = $1 AND status = 'RUNNING'",
            &[&request_id, &run_id],
        )
        .await?;
    Ok(())
}

pub async fn complete_crawl_request(
    client: &Client,
    request_id: Uuid,
    run_id: Uuid,
) -> Result<(), PostgresError> {
    client
        .execute(
            "UPDATE public.crawl_requests
             SET status = 'COMPLETED', finished_at = now(), source_run_id = $2
             WHERE id = $1 AND status = 'RUNNING'",
            &[&request_id, &run_id],
        )
        .await?;
    Ok(())
}

pub async fn fail_crawl_request(
    client: &Client,
    request_id: Uuid,
    run_id: Option<Uuid>,
    error_code: &str,
) -> Result<(), PostgresError> {
    client
        .execute(
            "UPDATE public.crawl_requests
             SET status = 'FAILED', finished_at = now(), source_run_id = COALESCE($2, source_run_id), error_code = $3
             WHERE id = $1 AND status IN ('PENDING', 'RUNNING')",
            &[&request_id, &run_id, &error_code],
        )
        .await?;
    Ok(())
}

pub async fn begin_crawl_run<C>(client: &C, source_id: Uuid) -> Result<CrawlRun, PostgresError>
where
    C: GenericClient + Sync,
{
    let row = client
        .query_one(
            "INSERT INTO public.source_crawl_runs (source_id, run_status, error_code) VALUES ($1, 'ANOMALY', 'crawl_started') RETURNING id, started_at",
            &[&source_id],
        )
        .await?;
    Ok(CrawlRun {
        id: row.get("id"),
        started_at: row.get("started_at"),
    })
}

pub async fn finish_crawl_run_if_unfinished<C>(
    client: &C,
    run_id: Uuid,
    report: &CrawlReport,
) -> Result<bool, PostgresError>
where
    C: GenericClient + Sync,
{
    let finished_at: DateTime<Utc> = Utc::now();
    let affected = client
        .execute(
            "UPDATE public.source_crawl_runs SET run_status = $2, finished_at = $3, pages_seen = $4, jobs_seen = $5, jobs_created = $6, jobs_updated = $7, jobs_missing = $8, error_code = $9 WHERE id = $1 AND finished_at IS NULL",
            &[
                &run_id,
                &run_status_name(report.run_status),
                &finished_at,
                &report.pages_seen,
                &report.jobs_seen,
                &report.jobs_created,
                &report.jobs_updated,
                &report.jobs_missing,
                &report.error_code,
            ],
        )
        .await?;
    Ok(affected == 1)
}

pub async fn crawl_run_is_finished(
    client: &Client,
    run_id: Uuid,
) -> Result<Option<bool>, PostgresError> {
    let row = client
        .query_opt(
            "SELECT finished_at IS NOT NULL FROM public.source_crawl_runs WHERE id = $1",
            &[&run_id],
        )
        .await?;
    Ok(row.map(|row| row.get(0)))
}

pub async fn persist_healthy_observations<C>(
    client: &C,
    source_id: Uuid,
    expected_base_url: &str,
    scope: &SourceScope,
    batch: &AuthoritativeObservationBatch,
) -> Result<DeltaCounts, StorageError>
where
    C: GenericClient + Sync,
{
    lock_active_source_snapshot(client, source_id, expected_base_url).await?;
    let mut counts = DeltaCounts::default();
    for observation in &batch.observations {
        if !scope.allows(&observation.original_url) {
            return Err(StorageError::ObservationOutsideScope);
        }
        observation
            .validate()
            .map_err(|_| StorageError::InvalidObservation)?;
        let normalized_location = normalize_location_text(&observation.location_text)
            .map_err(|_| StorageError::InvalidLocationText)?;
        let location_id = resolve_location_id(client, &normalized_location).await?;
        let existing = client
            .query_opt(
                "SELECT content_hash FROM public.job_cache WHERE source_id = $1 AND source_job_key = $2",
                &[&source_id, &observation.source_job_key],
            )
            .await?;
        let is_new = existing.is_none();
        let changed = existing
            .as_ref()
            .and_then(|row| row.try_get::<_, String>("content_hash").ok())
            .map(|hash| hash != observation.content_hash)
            .unwrap_or(true);
        let status = if observation.source_declared_closed {
            "CLOSED"
        } else {
            "ACTIVE"
        };
        let missing_cycles: i16 = if observation.source_declared_closed {
            2
        } else {
            0
        };
        client
            .execute(
                UPSERT_JOB_CACHE_QUERY,
                &[
                    &source_id,
                    &observation.source_job_key,
                    &observation.content_hash,
                    &observation.title,
                    &observation.company,
                    &observation.location_text,
                    &location_id,
                    &observation.latitude,
                    &observation.longitude,
                    &observation.role,
                    &observation.required_skills,
                    &observation.preferred_skills,
                    &observation.seniority,
                    &observation.minimum_experience_years,
                    &observation.domains,
                    &observation.employment_type,
                    &observation.salary_min,
                    &observation.salary_max,
                    &observation.salary_currency,
                    &observation.salary_period,
                    &observation.salary_source_text,
                    &observation.original_url,
                    &observation.work_mode,
                    &status,
                    &missing_cycles,
                ],
            )
            .await?;
        if is_new {
            counts.jobs_created += 1;
        } else if changed {
            counts.jobs_updated += 1;
        }
    }
    Ok(counts)
}

pub async fn reconcile_missing_jobs<C>(
    client: &C,
    source_id: Uuid,
    run_started_at: DateTime<Utc>,
    expected_base_url: &str,
    _batch: &AuthoritativeObservationBatch,
) -> Result<i32, StorageError>
where
    C: GenericClient + Sync,
{
    lock_active_source_snapshot(client, source_id, expected_base_url).await?;
    let rows = client
        .query(RECONCILE_MISSING_JOBS_QUERY, &[&source_id, &run_started_at])
        .await?;
    Ok(i32::try_from(rows.len()).unwrap_or(i32::MAX))
}

async fn lock_active_source_snapshot<C>(
    client: &C,
    source_id: Uuid,
    expected_base_url: &str,
) -> Result<(), StorageError>
where
    C: GenericClient + Sync,
{
    let row = client
        .query_opt(
            "SELECT base_url, approval_status FROM public.job_sources WHERE id = $1 FOR UPDATE",
            &[&source_id],
        )
        .await?;
    let Some(row) = row else {
        return Err(StorageError::SourceSnapshotChanged);
    };
    let base_url: String = row.get("base_url");
    let approval_status: String = row.get("approval_status");
    if approval_status != "ACTIVE" || base_url != expected_base_url {
        return Err(StorageError::SourceSnapshotChanged);
    }
    Ok(())
}

async fn resolve_location_id<C>(
    client: &C,
    normalized_location: &str,
) -> Result<Option<Uuid>, StorageError>
where
    C: GenericClient + Sync,
{
    let row = client
        .query_opt(
            "SELECT aliases.location_id FROM public.location_aliases AS aliases JOIN public.locations AS locations ON locations.id = aliases.location_id WHERE aliases.normalized_text = $1 AND locations.is_active = true UNION ALL SELECT id AS location_id FROM public.locations WHERE canonical_key = $1 AND is_active = true LIMIT 1",
            &[&normalized_location],
        )
        .await?;
    Ok(row.map(|row| row.get("location_id")))
}

pub const UPSERT_JOB_CACHE_QUERY: &str = r#"
INSERT INTO public.job_cache (
    source_id,
    source_job_key,
    content_hash,
    title,
    company,
    location_text,
    location_id,
    latitude,
    longitude,
    role,
    required_skills,
    preferred_skills,
    seniority,
    minimum_experience_years,
    domains,
    employment_type,
    work_mode,
    salary_min,
    salary_max,
    salary_currency,
    salary_period,
    salary_source_text,
    original_url,
    status,
    missing_healthy_cycles,
    location_assignment_source,
    last_seen_at,
    updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8::double precision, $9::double precision, $10, $11, $12, $13, $14::double precision, $15, $16, $23, $17::double precision, $18::double precision, $19, $20, $21, $22, $24, $25, 'AUTO', now(), now())
ON CONFLICT (source_id, source_job_key)
DO UPDATE SET
    content_hash = EXCLUDED.content_hash,
    title = EXCLUDED.title,
    company = EXCLUDED.company,
    location_text = EXCLUDED.location_text,
    -- Location ownership invariant:
    --  - ADMIN rows keep the canonical location the admin chose; the crawler
    --    must never overwrite or clear it (including on unresolved NULL).
    --  - AUTO rows accept crawler resolution, but an unresolved (NULL) result
    --    must not clobber an already-resolved value with NULL.
    location_id = CASE
        WHEN job_cache.location_assignment_source = 'ADMIN' THEN job_cache.location_id
        WHEN EXCLUDED.location_id IS NULL THEN job_cache.location_id
        ELSE EXCLUDED.location_id
    END,
    -- The crawler never flips an ADMIN row back to AUTO, and a new insert is
    -- always AUTO by construction (see the literal above).
    location_assignment_source = job_cache.location_assignment_source,
    latitude = EXCLUDED.latitude,
    longitude = EXCLUDED.longitude,
    role = EXCLUDED.role,
    required_skills = EXCLUDED.required_skills,
    preferred_skills = EXCLUDED.preferred_skills,
    seniority = EXCLUDED.seniority,
    minimum_experience_years = EXCLUDED.minimum_experience_years,
    domains = EXCLUDED.domains,
    employment_type = EXCLUDED.employment_type,
    work_mode = EXCLUDED.work_mode,
    salary_min = EXCLUDED.salary_min,
    salary_max = EXCLUDED.salary_max,
    salary_currency = EXCLUDED.salary_currency,
    salary_period = EXCLUDED.salary_period,
    salary_source_text = EXCLUDED.salary_source_text,
    original_url = EXCLUDED.original_url,
    status = EXCLUDED.status,
    missing_healthy_cycles = EXCLUDED.missing_healthy_cycles,
    last_seen_at = now(),
    updated_at = now()"#;

const RECONCILE_MISSING_JOBS_QUERY: &str = r#"
UPDATE public.job_cache
SET missing_healthy_cycles = LEAST(missing_healthy_cycles + 1, 2),
    status = CASE WHEN missing_healthy_cycles + 1 >= 2 THEN 'CLOSED' ELSE 'VERIFYING' END,
    updated_at = now()
WHERE source_id = $1
  AND status IN ('ACTIVE', 'VERIFYING')
  AND last_seen_at < $2
RETURNING id"#;

fn run_status_name(status: RunStatus) -> &'static str {
    match status {
        RunStatus::Healthy => "HEALTHY",
        RunStatus::SourceError => "SOURCE_ERROR",
        RunStatus::ParserError => "PARSER_ERROR",
        RunStatus::Anomaly => "ANOMALY",
    }
}

#[cfg(test)]
mod tests {
    use super::{RECONCILE_MISSING_JOBS_QUERY, UPSERT_JOB_CACHE_QUERY};

    // Guards the location-ownership invariant of the crawler upsert. The crawler
    // must never blind-overwrite job_cache.location_id because that erases the
    // canonical location an admin assigned in the Job Cache console.
    #[test]
    fn upsert_must_not_blind_overwrite_location_id() {
        assert!(
            !UPSERT_JOB_CACHE_QUERY.contains("location_id = EXCLUDED.location_id"),
            "crawler upsert must not blind-overwrite location_id"
        );
    }

    #[test]
    fn upsert_must_preserve_admin_location_and_reject_null_overwrite() {
        // The DO UPDATE branch must (a) keep the current value on ADMIN rows and
        // (b) never let an unresolved (NULL) crawler result NULL-out a resolved one.
        assert!(
            UPSERT_JOB_CACHE_QUERY.contains("job_cache.location_assignment_source = 'ADMIN'"),
            "upsert must guard ADMIN-owned locations"
        );
        assert!(
            UPSERT_JOB_CACHE_QUERY
                .contains("WHEN EXCLUDED.location_id IS NULL THEN job_cache.location_id"),
            "upsert must not NULL-out an existing location when the crawler cannot resolve one"
        );
        assert!(
            UPSERT_JOB_CACHE_QUERY
                .contains("location_assignment_source = job_cache.location_assignment_source"),
            "upsert must never change the assignment source on an existing row"
        );
    }

    #[test]
    fn reconcile_missing_jobs_must_not_touch_location() {
        // Location assignment is canonical metadata and must survive lifecycle
        // transitions (ACTIVE -> VERIFYING -> CLOSED) driven by reconciliation.
        assert!(!RECONCILE_MISSING_JOBS_QUERY.contains("location"));
    }
}
