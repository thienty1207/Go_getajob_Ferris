use sha2::{Digest, Sha256};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RunStatus {
    Healthy,
    SourceError,
    ParserError,
    Anomaly,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum JobStatus {
    Active,
    Verifying,
    Closed,
    Expired,
    Disabled,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct JobState {
    pub status: JobStatus,
    pub missing_healthy_cycles: u8,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ContentHashDelta {
    Unchanged,
    Changed,
}

impl ContentHashDelta {
    pub fn from_hashes(existing: &str, observed: &str) -> Self {
        if existing == observed {
            Self::Unchanged
        } else {
            Self::Changed
        }
    }
}

pub fn content_hash(bytes: &[u8]) -> String {
    let digest = Sha256::digest(bytes);
    digest.iter().map(|byte| format!("{byte:02x}")).collect()
}

pub fn reconcile_job(
    state: JobState,
    run_status: RunStatus,
    observed: bool,
    source_declared_closed: bool,
) -> JobState {
    if source_declared_closed {
        return JobState {
            status: JobStatus::Closed,
            missing_healthy_cycles: 2,
        };
    }
    if run_status != RunStatus::Healthy {
        return state;
    }
    if observed {
        return JobState {
            status: JobStatus::Active,
            missing_healthy_cycles: 0,
        };
    }
    match state.status {
        JobStatus::Active | JobStatus::Verifying => {
            let cycles = state.missing_healthy_cycles.saturating_add(1).min(2);
            JobState {
                status: if cycles >= 2 {
                    JobStatus::Closed
                } else {
                    JobStatus::Verifying
                },
                missing_healthy_cycles: cycles,
            }
        }
        JobStatus::Closed | JobStatus::Expired | JobStatus::Disabled => state,
    }
}

#[cfg(test)]
mod tests {
    use super::{content_hash, ContentHashDelta};

    #[test]
    fn content_hash_is_sha256_hex() {
        let hash = content_hash(b"ferris");
        assert_eq!(hash.len(), 64);
        assert_eq!(
            ContentHashDelta::from_hashes(&hash, &hash),
            ContentHashDelta::Unchanged
        );
        assert_eq!(
            ContentHashDelta::from_hashes(&hash, "different"),
            ContentHashDelta::Changed
        );
    }
}
