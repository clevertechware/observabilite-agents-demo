use std::str::FromStr;
use std::sync::{Arc, Mutex, PoisonError};

use crate::error::ApiError;

#[derive(Debug, thiserror::Error)]
pub enum FaultsError {
    #[error("{name}={value:?} is not a valid value")]
    InvalidValue { name: &'static str, value: String },
    #[error("FAULT_ERROR_RATE must be between 0 and 1, got {0}")]
    ErrorRateOutOfRange(f64),
}

#[derive(Clone, Default)]
pub struct Faults {
    error_rate: f64,
    leak_bytes_per_request: usize,
    leaked: Arc<Mutex<Vec<Vec<u8>>>>,
}

impl Faults {
    pub fn new(error_rate: f64, leak_bytes_per_request: usize) -> Result<Self, FaultsError> {
        if !(0.0..=1.0).contains(&error_rate) {
            return Err(FaultsError::ErrorRateOutOfRange(error_rate));
        }
        Ok(Self {
            error_rate,
            leak_bytes_per_request,
            leaked: Arc::default(),
        })
    }

    pub fn from_env() -> Result<Self, FaultsError> {
        Self::new(
            env_or("FAULT_ERROR_RATE", 0.0)?,
            env_or("FAULT_LEAK_BYTES_PER_REQUEST", 0)?,
        )
    }

    pub fn apply(&self) -> Result<(), ApiError> {
        if self.leak_bytes_per_request > 0 {
            self.retained().push(vec![1u8; self.leak_bytes_per_request]);
        }
        if rand::random::<f64>() < self.error_rate {
            tracing::error!("injected failure");
            return Err(ApiError::InjectedFailure);
        }
        Ok(())
    }

    pub fn leaked_bytes(&self) -> usize {
        self.retained().iter().map(Vec::len).sum()
    }

    fn retained(&self) -> std::sync::MutexGuard<'_, Vec<Vec<u8>>> {
        self.leaked.lock().unwrap_or_else(PoisonError::into_inner)
    }
}

fn env_or<T: FromStr>(name: &'static str, default: T) -> Result<T, FaultsError> {
    match std::env::var(name) {
        Err(_) => Ok(default),
        Ok(value) => value
            .parse()
            .map_err(|_| FaultsError::InvalidValue { name, value }),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn error_rate_above_one_is_rejected() {
        assert!(matches!(
            Faults::new(1.5, 0),
            Err(FaultsError::ErrorRateOutOfRange(_))
        ));
    }

    #[test]
    fn negative_error_rate_is_rejected() {
        assert!(Faults::new(-0.1, 0).is_err());
    }

    #[test]
    fn error_rate_of_one_always_fails() {
        let faults = Faults::new(1.0, 0).unwrap();
        assert!(matches!(faults.apply(), Err(ApiError::InjectedFailure)));
    }

    #[test]
    fn error_rate_of_zero_never_fails() {
        let faults = Faults::new(0.0, 0).unwrap();
        assert!((0..100).all(|_| faults.apply().is_ok()));
    }

    #[test]
    fn each_request_retains_the_configured_number_of_bytes() {
        let faults = Faults::new(0.0, 1024).unwrap();
        faults.apply().unwrap();
        faults.apply().unwrap();
        assert_eq!(faults.leaked_bytes(), 2048);
    }

    #[test]
    fn nothing_is_retained_by_default() {
        let faults = Faults::default();
        faults.apply().unwrap();
        assert_eq!(faults.leaked_bytes(), 0);
    }
}
