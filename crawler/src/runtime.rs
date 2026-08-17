#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RuntimeStatus {
    Offline,
    Idle,
    Running,
    Error,
}

impl RuntimeStatus {
    pub fn database_value(self) -> &'static str {
        match self {
            Self::Offline => "OFFLINE",
            Self::Idle => "IDLE",
            Self::Running => "RUNNING",
            Self::Error => "ERROR",
        }
    }
}

pub fn cycle_status(succeeded: bool) -> RuntimeStatus {
	if succeeded {
		RuntimeStatus::Idle
	} else {
		RuntimeStatus::Error
	}
}

#[cfg(test)]
mod tests {
	use super::{cycle_status, RuntimeStatus};

	#[test]
	fn cycle_completion_maps_to_truthful_runtime_state() {
		assert_eq!(cycle_status(true), RuntimeStatus::Idle);
		assert_eq!(cycle_status(false), RuntimeStatus::Error);
		assert_eq!(RuntimeStatus::Running.database_value(), "RUNNING");
	}
}
