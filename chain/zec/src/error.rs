use thiserror::Error;
use anyhow;

// Define an error enum that UniFFI can understand
#[derive(Debug, Error)]
pub enum ZecError {
    #[error("something went wrong: {0}")]
    GenericError(String),
    #[error("invalid Vault public key {0}")]
    InvalidVaultPubkey(anyhow::Error),
    #[error("invalid address: {0}")]
    InvalidAddress(String),
    #[error("initialization Failed: {0}")]
    InitError(String),
    #[error("invalid memo: {0}, error: {1}")]
    InvalidMemo(String, anyhow::Error),
    #[error("invalid amount: {0}, error: {1}")]
    InvalidAmount(u64, anyhow::Error),
}
