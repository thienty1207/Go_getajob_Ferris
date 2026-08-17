use anyhow::{anyhow, Context, Result};
use ferris_crawler::config::{
    database_crawl_interval_seconds, load_crawl_interval_seconds, load_database_url,
};
use ferris_crawler::crawl::{crawl_source, JsonLdJobPostingAdapter};
use ferris_crawler::scope::SourceScope;
use ferris_crawler::store::{
	attach_crawl_request_run, begin_crawl_run, claim_next_crawl_request, complete_crawl_request,
	crawl_run_is_finished, fail_crawl_request, finish_crawl_run_if_unfinished, load_active_sources,
	heartbeat_runtime, load_crawler_interval_setting, mark_inactive_crawl_requests,
	mark_runtime_cycle_finished, mark_runtime_cycle_started, mark_runtime_error, mark_runtime_source,
	persist_healthy_observations, reconcile_missing_jobs, ActiveSource, AuthoritativeObservationBatch,
};
use chrono::{Duration as ChronoDuration, Utc};
use std::thread;
use tokio::sync::oneshot;
use tokio::time::{sleep, Duration, Instant};
use tokio_postgres::NoTls;
use uuid::Uuid;

const REQUEST_POLL_INTERVAL_SECONDS: u64 = 5;

#[derive(Debug, PartialEq, Eq)]
enum CommitRecovery {
    AlreadyFinalized,
    Finalized,
}

#[tokio::main]
async fn main() -> Result<()> {
    let database_url = load_database_url()?;
    let fallback_interval_seconds = load_crawl_interval_seconds()?;
    let mut interval_seconds = fallback_interval_seconds;
    let run_once = std::env::args().any(|argument| argument == "--once");
    let mut next_full_cycle = Instant::now();
    let mut completed_full_cycle = false;

    loop {
        if run_once {
            if let Err(error) = drain_pending_requests(&database_url).await {
                eprintln!("crawler: pending request processing failed: {error:#}");
            }
            if let Err(error) = run_scheduled_cycle(&database_url, None).await {
                eprintln!("crawler: cycle failed: {error:#}");
            }
            return Ok(());
        }

        if let Err(error) = process_one_pending_request(&database_url).await {
            eprintln!("crawler: pending request processing failed: {error:#}");
        }

        match load_runtime_interval_seconds(&database_url, fallback_interval_seconds).await {
            Ok(updated_interval) if updated_interval != interval_seconds => {
                println!(
                    "crawler: interval reloaded from database: {} seconds",
                    updated_interval
                );
                interval_seconds = updated_interval;
                if completed_full_cycle {
                    next_full_cycle = Instant::now() + Duration::from_secs(interval_seconds);
                }
            }
            Ok(_) => {}
            Err(error) => eprintln!(
                "crawler: interval reload failed; keeping {} seconds: {error:#}",
                interval_seconds
            ),
        }

        if Instant::now() >= next_full_cycle {
            if let Err(error) = run_scheduled_cycle(&database_url, Some(interval_seconds)).await {
                eprintln!("crawler: cycle failed: {error:#}");
            }
            next_full_cycle = Instant::now() + Duration::from_secs(interval_seconds);
            completed_full_cycle = true;
        }

        tokio::select! {
            _ = tokio::signal::ctrl_c() => {
                println!("crawler: shutdown requested");
                return Ok(());
            }
            _ = sleep(Duration::from_secs(REQUEST_POLL_INTERVAL_SECONDS)) => {}
        }
    }
}

async fn run_scheduled_cycle(database_url: &str, interval_seconds: Option<u64>) -> Result<()> {
	mark_runtime_cycle_started_db(database_url).await?;
	let heartbeat_task = tokio::spawn(runtime_heartbeat_loop(database_url.to_owned()));
	let cycle_result = run_cycle(database_url).await;
	heartbeat_task.abort();
	let _ = heartbeat_task.await;

	let next_cycle_at = interval_seconds.map(|seconds| {
		Utc::now() + ChronoDuration::seconds(i64::try_from(seconds).unwrap_or(i64::MAX))
	});
	let runtime_result = match &cycle_result {
		Ok(()) => mark_runtime_cycle_finished_db(database_url, next_cycle_at).await,
		Err(_) => mark_runtime_error_db(database_url, next_cycle_at, "cycle_failed").await,
	};
	if let Err(error) = runtime_result {
		return Err(error.context("persist crawler runtime result"));
	}
	cycle_result
}

async fn runtime_heartbeat_loop(database_url: String) {
	loop {
		sleep(Duration::from_secs(REQUEST_POLL_INTERVAL_SECONDS)).await;
		if let Err(error) = heartbeat_runtime_db(&database_url).await {
			eprintln!("crawler: runtime heartbeat failed: {error:#}");
		}
	}
}

async fn mark_runtime_cycle_started_db(database_url: &str) -> Result<()> {
	let (client, connection) = tokio_postgres::connect(database_url, NoTls)
		.await
		.context("connect crawler to mark runtime cycle started")?;
	tokio::spawn(async move {
		if let Err(error) = connection.await {
			eprintln!("crawler runtime PostgreSQL connection stopped: {error}");
		}
	});
	mark_runtime_cycle_started(&client)
		.await
		.context("mark crawler runtime cycle started")
}

async fn mark_runtime_cycle_finished_db(
	database_url: &str,
	next_cycle_at: Option<chrono::DateTime<Utc>>,
) -> Result<()> {
	let (client, connection) = tokio_postgres::connect(database_url, NoTls)
		.await
		.context("connect crawler to mark runtime cycle finished")?;
	tokio::spawn(async move {
		if let Err(error) = connection.await {
			eprintln!("crawler runtime PostgreSQL connection stopped: {error}");
		}
	});
	mark_runtime_cycle_finished(&client, next_cycle_at)
		.await
		.context("mark crawler runtime cycle finished")
}

async fn mark_runtime_error_db(
	database_url: &str,
	next_cycle_at: Option<chrono::DateTime<Utc>>,
	error_code: &str,
) -> Result<()> {
	let (client, connection) = tokio_postgres::connect(database_url, NoTls)
		.await
		.context("connect crawler to mark runtime error")?;
	tokio::spawn(async move {
		if let Err(error) = connection.await {
			eprintln!("crawler runtime PostgreSQL connection stopped: {error}");
		}
	});
	mark_runtime_error(&client, next_cycle_at, error_code)
		.await
		.context("mark crawler runtime error")
}

async fn heartbeat_runtime_db(database_url: &str) -> Result<()> {
	let (client, connection) = tokio_postgres::connect(database_url, NoTls)
		.await
		.context("connect crawler to heartbeat runtime")?;
	tokio::spawn(async move {
		if let Err(error) = connection.await {
			eprintln!("crawler runtime PostgreSQL connection stopped: {error}");
		}
	});
	heartbeat_runtime(&client)
		.await
		.context("write crawler runtime heartbeat")
}

async fn load_runtime_interval_seconds(database_url: &str, fallback: u64) -> Result<u64> {
    let (client, connection) = tokio_postgres::connect(database_url, NoTls)
        .await
        .context("connect crawler to load runtime settings")?;
    tokio::spawn(async move {
        if let Err(error) = connection.await {
            eprintln!("crawler settings PostgreSQL connection stopped: {error}");
        }
    });
    match load_crawler_interval_setting(&client)
        .await
        .context("load crawler interval setting")?
    {
        Some(raw) => database_crawl_interval_seconds(&raw),
        None => Ok(fallback),
    }
}

async fn drain_pending_requests(database_url: &str) -> Result<()> {
    loop {
        if !process_one_pending_request(database_url).await? {
            return Ok(());
        }
    }
}

async fn process_one_pending_request(database_url: &str) -> Result<bool> {
    let (mut client, connection) = tokio_postgres::connect(database_url, NoTls)
        .await
        .context("connect crawler to process pending request")?;
    tokio::spawn(async move {
        if let Err(error) = connection.await {
            eprintln!("crawler request PostgreSQL connection stopped: {error}");
        }
    });

    mark_inactive_crawl_requests(&client)
        .await
        .context("mark inactive pending crawl requests")?;
    let Some(request) = claim_next_crawl_request(&client)
        .await
        .context("claim pending crawl request")?
    else {
        return Ok(false);
    };

    let request_id = request.id;
    let source_key = request.source.source_key.clone();
    mark_runtime_cycle_started(&client)
        .await
        .context("mark runtime for manual crawl request")?;
    mark_runtime_source(&client, &source_key)
        .await
        .context("mark manual crawl request source")?;
    let heartbeat_task = tokio::spawn(runtime_heartbeat_loop(database_url.to_owned()));
    let source_result = run_source(&mut client, database_url, request.source, Some(request_id)).await;
    heartbeat_task.abort();
    let _ = heartbeat_task.await;
    match source_result {
        Ok(()) => {
            mark_runtime_cycle_finished(&client, None)
                .await
                .context("mark manual crawl request idle")?;
            Ok(true)
        }
        Err(error) => {
            let failure = fail_crawl_request(&client, request_id, None, "worker_failed").await;
            if let Err(failure_error) = failure {
                return Err(anyhow!(
                    "manual crawl request for {source_key} failed: {error:#}; request recovery failed: {failure_error}"
                ));
            }
            mark_runtime_error(&client, None, "manual_request_failed")
                .await
                .context("mark manual crawl request error")?;
            Err(anyhow!(
                "manual crawl request for {source_key} failed: {error:#}"
            ))
        }
    }
}

async fn run_cycle(database_url: &str) -> Result<()> {
    let (mut client, connection) = tokio_postgres::connect(&database_url, NoTls)
        .await
        .context("connect crawler to PostgreSQL")?;
    tokio::spawn(async move {
        if let Err(error) = connection.await {
            eprintln!("crawler PostgreSQL connection stopped: {error}");
        }
    });

    let sources = load_active_sources(&client)
        .await
        .context("load active Job Links")?;
    if sources.is_empty() {
        println!("crawler: no ACTIVE Job Link sources; nothing to crawl");
        return Ok(());
	}

	for source in sources {
		mark_runtime_source(&client, &source.source_key)
			.await
			.with_context(|| format!("mark crawler source {}", source.source_key))?;
		run_source(&mut client, database_url, source, None).await?;
	}
    Ok(())
}

async fn run_source(
    client: &mut tokio_postgres::Client,
    database_url: &str,
    source: ActiveSource,
    request_id: Option<Uuid>,
) -> Result<()> {
    let run = begin_crawl_run(client, source.id)
        .await
        .with_context(|| format!("start crawl run for {}", source.source_key))?;
    if let Some(request_id) = request_id {
        attach_crawl_request_run(client, request_id, run.id)
            .await
            .with_context(|| format!("attach crawl request to run {}", run.id))?;
    }
    let mut report =
        match crawl_source_isolated(&source.base_url, source.robots_url.as_deref()).await {
            Ok(report) => report,
            Err(error) => {
                eprintln!("crawler: source {} rejected: {error}", source.source_key);
                ferris_crawler::crawl::CrawlReport {
                    run_status: ferris_crawler::reconcile::RunStatus::SourceError,
                    pages_seen: 0,
                    jobs_seen: 0,
                    jobs_created: 0,
                    jobs_updated: 0,
                    jobs_missing: 0,
                    error_code: Some("source_fetch_unavailable"),
                    reconciliation_safe: false,
                    observations: Vec::new(),
                }
            }
        };
    if report.run_status == ferris_crawler::reconcile::RunStatus::Healthy {
        let scope = match SourceScope::new(&source.base_url) {
            Ok(scope) => scope,
            Err(error) => {
                report.run_status = ferris_crawler::reconcile::RunStatus::Anomaly;
                report.error_code = Some("source_scope_invalid");
                report.jobs_created = 0;
                report.jobs_updated = 0;
                report.jobs_missing = 0;
                finalize_run(client, database_url, run.id, &report)
                    .await
                    .with_context(|| {
                        format!("finish invalid source run for {}", source.source_key)
                    })?;
                eprintln!(
                    "crawler: source {} rejected after crawl: {error}",
                    source.source_key
                );
                complete_request(client, request_id, run.id).await?;
                println!(
                    "crawler: source={} status={:?} pages={} jobs={}",
                    source.source_key, report.run_status, report.pages_seen, report.jobs_seen
                );
                return Ok(());
            }
        };

        let batch = match AuthoritativeObservationBatch::from_report(&mut report) {
            Ok(batch) => batch,
            Err(error) => {
                report.run_status = ferris_crawler::reconcile::RunStatus::Anomaly;
                report.error_code = Some("reconciliation_not_authorized");
                report.jobs_created = 0;
                report.jobs_updated = 0;
                report.jobs_missing = 0;
                finalize_run(client, database_url, run.id, &report)
                    .await
                    .with_context(|| {
                        format!("finish unsafe source run for {}", source.source_key)
                    })?;
                eprintln!(
                    "crawler: source {} skipped unsafe reconciliation: {error}",
                    source.source_key
                );
                complete_request(client, request_id, run.id).await?;
                println!(
                    "crawler: source={} status={:?} pages={} jobs={}",
                    source.source_key, report.run_status, report.pages_seen, report.jobs_seen
                );
                return Ok(());
            }
        };

        // Job-cache writes, missing-job reconciliation, and the final run
        // status share one transaction. A failed observation or SQL write
        // therefore cannot leave a half-applied crawl cycle behind.
        let persistence_result = async {
            let transaction = client.transaction().await?;
            let delta = persist_healthy_observations(
                &transaction,
                source.id,
                &source.base_url,
                &scope,
                &batch,
            )
            .await?;
            report.jobs_created = delta.jobs_created;
            report.jobs_updated = delta.jobs_updated;
            report.jobs_missing = reconcile_missing_jobs(
                &transaction,
                source.id,
                run.started_at,
                &source.base_url,
                &batch,
            )
            .await?;
            if !finish_crawl_run_if_unfinished(&transaction, run.id, &report).await? {
                return Err(anyhow!(
                    "crawl run {} was already finalized before transaction commit",
                    run.id
                ));
            }
            transaction.commit().await?;
            Ok::<(), anyhow::Error>(())
        }
        .await;

        if let Err(error) = persistence_result {
            report.run_status = ferris_crawler::reconcile::RunStatus::Anomaly;
            report.error_code = Some("persistence_failed");
            report.jobs_created = 0;
            report.jobs_updated = 0;
            report.jobs_missing = 0;
            match recover_run_finalization(database_url, run.id, &report).await {
                    Ok(CommitRecovery::AlreadyFinalized) => eprintln!(
                        "crawler: source {} persistence outcome already finalized; preserved committed run",
                        source.source_key
                    ),
                    Ok(CommitRecovery::Finalized) => eprintln!(
                        "crawler: source {} persistence failed before commit: {error}",
                        source.source_key
                    ),
                    Err(recovery_error) => {
                        return Err(anyhow!(
                            "crawler: source {} persistence outcome is unresolved: {error}; recovery failed: {recovery_error}",
                            source.source_key
                        ));
                    }
                }
            complete_request(client, request_id, run.id).await?;
            println!(
                "crawler: source={} status={:?} pages={} jobs={}",
                source.source_key, report.run_status, report.pages_seen, report.jobs_seen
            );
            return Ok(());
        }
    } else {
        finalize_run(client, database_url, run.id, &report)
            .await
            .with_context(|| format!("finish crawl run for {}", source.source_key))?;
    }
    complete_request(client, request_id, run.id).await?;
    println!(
        "crawler: source={} status={:?} pages={} jobs={}",
        source.source_key, report.run_status, report.pages_seen, report.jobs_seen
    );
    Ok(())
}

async fn complete_request(
    client: &tokio_postgres::Client,
    request_id: Option<Uuid>,
    run_id: Uuid,
) -> Result<()> {
    if let Some(request_id) = request_id {
        complete_crawl_request(client, request_id, run_id)
            .await
            .with_context(|| format!("complete crawl request {request_id}"))?;
    }
    Ok(())
}

async fn crawl_source_isolated(
    base_url: &str,
    robots_url: Option<&str>,
) -> Result<ferris_crawler::crawl::CrawlReport> {
    // Spider's HTML/link parser can consume substantially more stack on real
    // third-party pages than on unit-test fixtures. Keep that untrusted parse
    // workload on a bounded worker stack instead of the Tokio worker stack.
    const CRAWL_THREAD_STACK_BYTES: usize = 8 * 1024 * 1024;
    let base_url = base_url.to_owned();
    let robots_url = robots_url.map(ToOwned::to_owned);
    let (sender, receiver) = oneshot::channel();
    thread::Builder::new()
        .name("ferris-crawl-source".to_owned())
        .stack_size(CRAWL_THREAD_STACK_BYTES)
        .spawn(move || {
            let result = tokio::runtime::Builder::new_current_thread()
                .enable_all()
                .build()
                .map_err(|error| anyhow!("build isolated crawler runtime: {error}"))
                .and_then(|runtime| {
                    runtime
                        .block_on(crawl_source(
                            &base_url,
                            robots_url.as_deref(),
                            &JsonLdJobPostingAdapter,
                        ))
                        .map_err(anyhow::Error::from)
                });
            let _ = sender.send(result);
        })
        .context("spawn isolated crawler thread")?;
    receiver
        .await
        .context("isolated crawler thread stopped unexpectedly")?
}

async fn finalize_run(
    client: &tokio_postgres::Client,
    database_url: &str,
    run_id: Uuid,
    report: &ferris_crawler::crawl::CrawlReport,
) -> Result<CommitRecovery> {
    match finish_crawl_run_if_unfinished(client, run_id, report).await {
        Ok(true) => Ok(CommitRecovery::Finalized),
        Ok(false) => resolve_run_finalization(client, run_id).await,
        Err(error) => recover_run_finalization(database_url, run_id, report)
            .await
            .with_context(|| format!("resolve ambiguous crawl-run finalization: {error}")),
    }
}

async fn resolve_run_finalization(
    client: &tokio_postgres::Client,
    run_id: Uuid,
) -> Result<CommitRecovery> {
    match crawl_run_is_finished(client, run_id).await? {
        Some(true) => Ok(CommitRecovery::AlreadyFinalized),
        Some(false) => Err(anyhow!(
            "crawler run {run_id} remained unfinished after compare-and-set finalization"
        )),
        None => Err(anyhow!(
            "crawler run {run_id} was not found during finalization"
        )),
    }
}

async fn recover_run_finalization(
    database_url: &str,
    run_id: Uuid,
    anomaly_report: &ferris_crawler::crawl::CrawlReport,
) -> Result<CommitRecovery> {
    let (client, connection) = tokio_postgres::connect(database_url, NoTls)
        .await
        .context("reconnect to resolve crawler commit outcome")?;
    tokio::spawn(async move {
        if let Err(error) = connection.await {
            eprintln!("crawler PostgreSQL recovery connection stopped: {error}");
        }
    });

    if finish_crawl_run_if_unfinished(&client, run_id, anomaly_report).await? {
        return Ok(CommitRecovery::Finalized);
    }
    resolve_run_finalization(&client, run_id).await
}
