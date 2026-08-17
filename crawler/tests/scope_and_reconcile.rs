use ferris_crawler::reconcile::{reconcile_job, ContentHashDelta, JobState, JobStatus, RunStatus};
use ferris_crawler::scope::SourceScope;

#[test]
fn scope_allows_only_same_origin_and_registered_path() {
    let scope = SourceScope::new("https://jobs.example.com/careers").expect("valid scope");

    assert!(scope.allows("https://jobs.example.com/careers/engineering/123"));
    assert!(scope.allows("https://JOBS.EXAMPLE.COM/careers#fragment"));
    assert!(!scope.allows("https://jobs.example.com/other/123"));
    assert!(!scope.allows("https://other.example.com/careers/123"));
    assert!(!scope.allows("http://jobs.example.com/careers/123"));
    assert!(!scope.allows("https://jobs.example.com.evil.test/careers/123"));
}

#[test]
fn scope_rejects_private_and_credential_bearing_sources() {
    assert!(SourceScope::new("http://127.0.0.1/jobs").is_err());
    assert!(SourceScope::new("https://user:password@jobs.example.com/jobs").is_err());
}

#[test]
fn healthy_missing_cycles_close_only_after_two_consecutive_cycles() {
    let active = JobState {
        status: JobStatus::Active,
        missing_healthy_cycles: 0,
    };
    let verifying = reconcile_job(active, RunStatus::Healthy, false, false);
    assert_eq!(verifying.status, JobStatus::Verifying);
    assert_eq!(verifying.missing_healthy_cycles, 1);

    let closed = reconcile_job(verifying, RunStatus::Healthy, false, false);
    assert_eq!(closed.status, JobStatus::Closed);
    assert_eq!(closed.missing_healthy_cycles, 2);
}

#[test]
fn unhealthy_runs_never_mark_jobs_missing() {
    let state = JobState {
        status: JobStatus::Active,
        missing_healthy_cycles: 0,
    };

    for status in [
        RunStatus::SourceError,
        RunStatus::ParserError,
        RunStatus::Anomaly,
    ] {
        assert_eq!(reconcile_job(state, status, false, false), state);
    }
}

#[test]
fn source_declared_closed_wins_immediately_and_hash_delta_is_deterministic() {
    let state = JobState {
        status: JobStatus::Active,
        missing_healthy_cycles: 0,
    };
    let closed = reconcile_job(state, RunStatus::Healthy, true, true);
    assert_eq!(closed.status, JobStatus::Closed);
    assert_eq!(closed.missing_healthy_cycles, 2);

    assert_eq!(
        ContentHashDelta::from_hashes("abc", "abc"),
        ContentHashDelta::Unchanged
    );
    assert_eq!(
        ContentHashDelta::from_hashes("abc", "def"),
        ContentHashDelta::Changed
    );
}
