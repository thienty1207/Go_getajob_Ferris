use chrono::Utc;
use ferris_crawler::crawl::{CrawlReport, StructuredJobObservation};
use ferris_crawler::reconcile::RunStatus;
use ferris_crawler::scope::SourceScope;
use ferris_crawler::store::{
    begin_crawl_run, finish_crawl_run_if_unfinished, persist_healthy_observations,
    AuthoritativeObservationBatch, DeltaCounts,
};
use tokio_postgres::NoTls;
use uuid::Uuid;

#[tokio::test]
#[ignore = "requires a migrated PostgreSQL database and is run as an explicit smoke gate"]
async fn crawler_storage_contract_uses_the_migrated_schema_atomically() {
    let database_url = std::env::var("DATABASE_URL")
        .expect("DATABASE_URL must be set for the PostgreSQL crawler contract test");
    let (mut client, connection) = tokio_postgres::connect(&database_url, NoTls)
        .await
        .expect("connect to PostgreSQL");
    tokio::spawn(async move {
        connection
            .await
            .expect("PostgreSQL connection remains healthy");
    });

    let transaction = client.transaction().await.expect("begin test transaction");
    let source_id = Uuid::new_v4();
    let source_key = format!("baron-runtime-contract-{source_id}");
    let base_url = "https://example.com/careers/";
    let approved_at = Utc::now();
    let approved_by = "baron-runtime-contract";
    transaction
        .execute(
            "INSERT INTO public.job_sources (id, source_key, display_name, base_url, source_type, approval_status, approved_at, approved_by) VALUES ($1, $2, $3, $4, 'EXPLICIT_PERMISSION', 'ACTIVE', $5, $6)",
            &[&source_id, &source_key, &"Runtime contract", &base_url, &approved_at, &approved_by],
        )
        .await
        .expect("insert temporary active source");

    let location_id = Uuid::new_v4();
    transaction
        .execute(
            "INSERT INTO public.locations (id, display_name, province, country, canonical_key) VALUES ($1, $2, $3, $4, $5)",
            &[&location_id, &"Hồ Chí Minh", &"Hồ Chí Minh", &"Vietnam", &"ho chi minh"],
        )
        .await
        .expect("insert temporary canonical location");
    transaction
        .execute(
            "INSERT INTO public.location_aliases (location_id, normalized_text, alias_text) VALUES ($1, $2, $3)",
            &[&location_id, &"ho chi minh city", &"Ho Chi Minh City"],
        )
        .await
        .expect("insert temporary location alias");

    let run = begin_crawl_run(&transaction, source_id)
        .await
        .expect("begin crawl run satisfies the run-status constraint");
    let scope = SourceScope::new(base_url).expect("test source scope");
    let observation = StructuredJobObservation {
        source_job_key: "runtime-contract-job".to_string(),
        content_hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
            .to_string(),
        title: "Runtime contract job".to_string(),
        company: "Runtime contract".to_string(),
        location_text: "Ho Chi Minh City".to_string(),
        latitude: None,
        longitude: None,
        role: "Software Engineer".to_string(),
        required_skills: vec!["Go".to_string()],
        preferred_skills: vec!["PostgreSQL".to_string()],
        seniority: "MID".to_string(),
        minimum_experience_years: Some(2.0),
        domains: vec!["software".to_string()],
        employment_type: "FULL_TIME".to_string(),
        work_mode: "HYBRID".to_string(),
        salary_min: None,
        salary_max: None,
        salary_currency: None,
        salary_period: None,
        salary_source_text: None,
        original_url: "https://example.com/careers/runtime-contract-job".to_string(),
        source_declared_closed: false,
    };
    let mut report = CrawlReport {
        run_status: RunStatus::Healthy,
        pages_seen: 1,
        jobs_seen: 1,
        jobs_created: 0,
        jobs_updated: 0,
        jobs_missing: 0,
        error_code: None,
        reconciliation_safe: true,
        observations: vec![observation.clone()],
    };
    let batch = AuthoritativeObservationBatch::from_report(&mut report)
        .expect("healthy report creates an authoritative observation batch");
    let delta = persist_healthy_observations(&transaction, source_id, base_url, &scope, &batch)
        .await
        .expect("persist structured observation");
    assert_eq!(
        delta,
        DeltaCounts {
            jobs_created: 1,
            jobs_updated: 0
        }
    );
    let persisted_location: Option<Uuid> = transaction
        .query_one(
            "SELECT location_id FROM public.job_cache WHERE source_id = $1 AND source_job_key = $2",
            &[&source_id, &"runtime-contract-job"],
        )
        .await
        .expect("read resolved job location")
        .get("location_id");
    assert_eq!(persisted_location, Some(location_id));

    let mut invalid_observation = observation.clone();
    invalid_observation.original_url = "https://example.com/outside-job".to_string();
    let mut invalid_report = CrawlReport {
        run_status: RunStatus::Healthy,
        pages_seen: 1,
        jobs_seen: 1,
        jobs_created: 0,
        jobs_updated: 0,
        jobs_missing: 0,
        error_code: None,
        reconciliation_safe: true,
        observations: vec![invalid_observation],
    };
    let invalid_batch = AuthoritativeObservationBatch::from_report(&mut invalid_report)
        .expect("healthy invalid report creates a batch for scope validation");
    assert!(persist_healthy_observations(
        &transaction,
        source_id,
        base_url,
        &scope,
        &invalid_batch,
    )
    .await
    .is_err());

    report.jobs_created = delta.jobs_created;
    report.jobs_updated = delta.jobs_updated;
    assert!(
        finish_crawl_run_if_unfinished(&transaction, run.id, &report)
            .await
            .expect("finish healthy crawl run")
    );
    let status: String = transaction
        .query_one(
            "SELECT run_status FROM public.source_crawl_runs WHERE id = $1",
            &[&run.id],
        )
        .await
        .expect("read finished crawl run")
        .get("run_status");
    assert_eq!(status, "HEALTHY");
    let anomaly_report = CrawlReport {
        run_status: RunStatus::Anomaly,
        pages_seen: 0,
        jobs_seen: 0,
        jobs_created: 0,
        jobs_updated: 0,
        jobs_missing: 0,
        error_code: Some("recovery_probe"),
        reconciliation_safe: false,
        observations: Vec::new(),
    };
    assert!(
        !finish_crawl_run_if_unfinished(&transaction, run.id, &anomaly_report)
            .await
            .expect("compare-and-set finished run")
    );
    let status_after_probe: String = transaction
        .query_one(
            "SELECT run_status FROM public.source_crawl_runs WHERE id = $1",
            &[&run.id],
        )
        .await
        .expect("read run after compare-and-set probe")
        .get("run_status");
    assert_eq!(status_after_probe, "HEALTHY");

    transaction
        .rollback()
        .await
        .expect("rollback contract data");
    let persisted_count: i64 = client
        .query_one(
            "SELECT count(*) FROM public.job_sources WHERE id = $1",
            &[&source_id],
        )
        .await
        .expect("check rollback")
        .get(0);
    assert_eq!(persisted_count, 0);
    let persisted_run_count: i64 = client
        .query_one(
            "SELECT count(*) FROM public.source_crawl_runs WHERE source_id = $1",
            &[&source_id],
        )
        .await
        .expect("check crawl-run rollback")
        .get(0);
    assert_eq!(persisted_run_count, 0);
    let persisted_job_count: i64 = client
        .query_one(
            "SELECT count(*) FROM public.job_cache WHERE source_id = $1",
            &[&source_id],
        )
        .await
        .expect("check job-cache rollback")
        .get(0);
    assert_eq!(persisted_job_count, 0);
}
