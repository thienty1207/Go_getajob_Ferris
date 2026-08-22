//! Regression contract for location ownership on job_cache.
//!
//! Admin-assigned canonical locations (job_cache.location_id plus
//! location_assignment_source = 'ADMIN') must survive subsequent crawler
//! upserts, including unresolved (NULL) results and a re-crawl of the same
//! source_job_key. AUTO rows, in turn, must accept fresh resolution but never
//! be NULL-ed out by an unresolved result.
//!
//! Runs against a migrated PostgreSQL (DATABASE_URL). It is #[ignore]d and is
//! executed explicitly as a smoke gate, mirroring `postgres_contract.rs`.

use chrono::Utc;
use ferris_crawler::crawl::{CrawlReport, StructuredJobObservation};
use ferris_crawler::reconcile::RunStatus;
use ferris_crawler::scope::SourceScope;
use ferris_crawler::store::{persist_healthy_observations, AuthoritativeObservationBatch};
use tokio_postgres::NoTls;
use uuid::Uuid;

fn observation(key: &str, location_text: &str) -> StructuredJobObservation {
    // content_hash must be exactly 64 lowercase hex characters; derive a
    // stable hash so repeated observations have a distinct, valid value.
    use sha2::{Digest, Sha256};
    let content_hash = {
        let mut hasher = Sha256::new();
        hasher.update(key.as_bytes());
        hasher.update(location_text.as_bytes());
        format!("{:x}", hasher.finalize())
    };
    StructuredJobObservation {
        source_job_key: key.to_string(),
        content_hash,
        title: "Location ownership contract job".to_string(),
        company: "Ownership contract".to_string(),
        location_text: location_text.to_string(),
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
        original_url: format!("https://example.com/careers/{key}"),
        source_declared_closed: false,
    }
}

async fn persist(
    transaction: &tokio_postgres::Transaction<'_>,
    source_id: Uuid,
    base_url: &str,
    observation: StructuredJobObservation,
) {
    let mut report = CrawlReport {
        run_status: RunStatus::Healthy,
        pages_seen: 1,
        jobs_seen: 1,
        jobs_created: 0,
        jobs_updated: 0,
        jobs_missing: 0,
        error_code: None,
        reconciliation_safe: true,
        observations: vec![observation],
    };
    let batch = AuthoritativeObservationBatch::from_report(&mut report)
        .expect("healthy report creates an authoritative observation batch");
    let scope = SourceScope::new(base_url).expect("test source scope");
    let _ = persist_healthy_observations(transaction, source_id, base_url, &scope, &batch)
        .await
        .expect("persist observation");
}

#[tokio::test]
#[ignore = "requires a migrated PostgreSQL database; run explicitly as a location-ownership smoke gate"]
async fn admin_assigned_location_survives_recrawls_and_unresolved_null() {
    let database_url = std::env::var("DATABASE_URL")
        .expect("DATABASE_URL must be set for the location ownership contract test");
    let (mut client, connection) = tokio_postgres::connect(&database_url, NoTls)
        .await
        .expect("connect to PostgreSQL");
    tokio::spawn(async move {
        connection
            .await
            .expect("PostgreSQL connection remains healthy");
    });

    // Use a committed transaction so row identity persists across upserts; the
    // whole test wraps in a transaction we roll back at the end.
    let transaction = client.transaction().await.expect("begin test transaction");

    let source_id = Uuid::new_v4();
    let base_url = "https://example.com/careers/";
    transaction
        .execute(
            "INSERT INTO public.job_sources (id, source_key, display_name, base_url, source_type, approval_status, approved_at, approved_by) VALUES ($1, $2, $3, $4, 'EXPLICIT_PERMISSION', 'ACTIVE', $5, $6)",
            &[
                &source_id,
                &format!("location-own-{source_id}"),
                &"Location ownership contract",
                &base_url,
                &Utc::now(),
                &"location-ownership-contract",
            ],
        )
        .await
        .expect("insert temporary active source");

    let hcm_id = Uuid::new_v4();
    let hanoi_id = Uuid::new_v4();
    // Use a unique suffix so this test never collides with the project's real
    // canonical locations even when run against a populated database. The test
    // transaction is rolled back, so nothing leaks.
    let suffix = Uuid::new_v4().simple().to_string();
    let hcm_city = format!("LocOwn HCM {suffix}");
    let hanoi_city = format!("LocOwn Hanoi {suffix}");
    let hcm_key = format!("locown-hcm-{suffix}");
    let hanoi_key = format!("locown-hanoi-{suffix}");
    // Resolved alias text must equal the normalized form of the observation
    // location_text; keep it as pre-normalized lower-case alphanumeric+spaces.
    let hcm_resolved = format!("locown hcm {suffix} city");
    let hanoi_resolved = format!("locown hanoi {suffix} city");
    for (id, display, province, key, resolved) in [
        (hcm_id, &hcm_city, &hcm_city, &hcm_key, &hcm_resolved),
        (
            hanoi_id,
            &hanoi_city,
            &hanoi_city,
            &hanoi_key,
            &hanoi_resolved,
        ),
    ] {
        transaction
            .execute(
                "INSERT INTO public.locations (id, display_name, province, country, canonical_key) VALUES ($1, $2, $3, $4, $5)",
                &[&id, &display, &province, &"Vietnam", &key],
            )
            .await
            .expect("insert temporary canonical location");
        transaction
            .execute(
                "INSERT INTO public.location_aliases (location_id, normalized_text, alias_text) VALUES ($1, $2, $3)",
                &[&id, &resolved, &format!("{} City", display)],
            )
            .await
            .expect("insert temporary location alias");
    }

    // Seed the job. AUTO resolution resolves "ho chi minh city" -> HCM.
    persist(
        &transaction,
        source_id,
        base_url,
        observation("owned-job", hcm_resolved.as_str()),
    )
    .await;

    let seeded_job_row = transaction
        .query_one(
            "SELECT id, location_id, location_assignment_source FROM public.job_cache WHERE source_id = $1 AND source_job_key = $2",
            &[&source_id, &"owned-job"],
        )
        .await
        .expect("read seeded job");
    let row_id: Uuid = seeded_job_row.get("id");
    let initial_location: Option<Uuid> = seeded_job_row.get("location_id");
    let initial_source: String = seeded_job_row.get("location_assignment_source");
    let seeded = (
        row_id,
        initial_location.expect("AUTO job resolves to HCM"),
        initial_source,
    );
    assert_eq!(seeded.2, "AUTO", "crawler-created row must start as AUTO");
    assert_eq!(seeded.1, hcm_id, "AUTO resolution must select HCM");

    // Admin assigns location to HCM (idempotent with the existing resolution)
    // and the API marks ownership. Run the same SQL the backend uses.
    transaction
        .execute(
            "UPDATE public.job_cache SET location_id = $2, location_assignment_source = CASE WHEN $2::uuid IS NULL THEN 'AUTO' ELSE 'ADMIN' END, updated_at = now() WHERE id = $1",
            &[&seeded.0, &hcm_id],
        )
        .await
        .expect("admin assigns HCM");
    let source_after_assign: String = transaction
        .query_one(
            "SELECT location_assignment_source FROM public.job_cache WHERE id = $1",
            &[&seeded.0],
        )
        .await
        .expect("read assignment source after admin assign")
        .get("location_assignment_source");
    assert_eq!(
        source_after_assign, "ADMIN",
        "admin assignment must be ADMIN"
    );

    // Phase 4 case 1: crawler re-result resolves NULL (unknown location text).
    persist(
        &transaction,
        source_id,
        base_url,
        observation("owned-job", "unknown industrial zone xyz"),
    )
    .await;
    let row1 = transaction
        .query_one(
            "SELECT id, location_id, location_assignment_source FROM public.job_cache WHERE source_id = $1 AND source_job_key = $2",
            &[&source_id, &"owned-job"],
        )
        .await
        .expect("read after NULL resolve");
    let id1: Uuid = row1.get("id");
    let loc1: Option<Uuid> = row1.get("location_id");
    let src1: String = row1.get("location_assignment_source");
    assert_eq!(id1, seeded.0, "row id must not change across re-crawls");
    assert_eq!(
        loc1,
        Some(hcm_id),
        "admin location must survive unresolved NULL"
    );
    assert_eq!(
        src1, "ADMIN",
        "ADMIN ownership must survive unresolved NULL"
    );

    // Phase 4 case 2: crawler resolves a different location (Hanoi).
    persist(
        &transaction,
        source_id,
        base_url,
        observation("owned-job", hanoi_resolved.as_str()),
    )
    .await;
    let row2 = transaction
        .query_one(
            "SELECT id, location_id, location_assignment_source FROM public.job_cache WHERE source_id = $1 AND source_job_key = $2",
            &[&source_id, &"owned-job"],
        )
        .await
        .expect("read after different-location resolve");
    let id2: Uuid = row2.get("id");
    let loc2: Option<Uuid> = row2.get("location_id");
    let src2: String = row2.get("location_assignment_source");
    assert_eq!(id2, seeded.0, "row id must not change across re-crawls");
    assert_eq!(
        loc2,
        Some(hcm_id),
        "admin location must survive crawler resolving elsewhere"
    );
    assert_eq!(
        src2, "ADMIN",
        "ADMIN ownership must survive crawler resolving elsewhere"
    );

    // Phase 4 case 4/5: an AUTO-only job accepts fresh resolution.
    persist(
        &transaction,
        source_id,
        base_url,
        observation("auto-job", hanoi_resolved.as_str()),
    )
    .await;
    let auto_job_row = transaction
        .query_one(
            "SELECT location_id, location_assignment_source FROM public.job_cache WHERE source_id = $1 AND source_job_key = $2",
            &[&source_id, &"auto-job"],
        )
        .await
        .expect("read AUTO job");
    let auto_location: Option<Uuid> = auto_job_row.get("location_id");
    let auto_source: String = auto_job_row.get("location_assignment_source");
    assert_eq!(auto_location, Some(hanoi_id), "AUTO job resolves to Hanoi");
    assert_eq!(auto_source, "AUTO", "crawler-created row stays AUTO");

    // Phase 4 case 6: an unresolved AUTO result must not NULL out the resolved value.
    persist(
        &transaction,
        source_id,
        base_url,
        observation("auto-job", "unknown town v"),
    )
    .await;
    let auto_after_null: Option<Uuid> = transaction
        .query_one(
            "SELECT location_id FROM public.job_cache WHERE source_id = $1 AND source_job_key = $2",
            &[&source_id, &"auto-job"],
        )
        .await
        .expect("read AUTO job after unresolved")
        .get("location_id");
    assert_eq!(
        auto_after_null,
        Some(hanoi_id),
        "AUTO job must keep last resolved location on unresolved"
    );

    // Roll the whole test transaction back so nothing leaks into the database.
    transaction
        .rollback()
        .await
        .expect("rollback ownership test data");
}
