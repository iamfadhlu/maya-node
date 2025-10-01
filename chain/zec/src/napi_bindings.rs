use napi::bindgen_prelude::*;
use napi_derive::napi;

use crate::error::ZecError;
use crate::network::Network;
use crate::tx::{UTXO, Output, PartialTx, Sighashes};
use crate::{init_zec, build_ptx, apply_signatures, compute_txid, get_ovk, validate_address};

// Convert internal types to NAPI-compatible types

#[napi(object)]
pub struct NapiUTXO {
    pub txid: String,
    pub height: u32,
    pub vout: u32,
    pub script: String,
    pub value: f64, // Using f64 instead of u64 for JavaScript compatibility
}

#[napi(object)]
pub struct NapiOutput {
    pub address: String,
    pub amount: f64, // Using f64 instead of u64 for JavaScript compatibility
    pub memo: String,
}

#[napi(object)]
pub struct NapiPartialTx {
    pub height: u32,
    pub txid: Buffer,
    pub inputs: Vec<NapiUTXO>,
    pub outputs: Vec<NapiOutput>,
    pub fee: f64, // Using f64 instead of u64 for JavaScript compatibility
    pub sighashes: Vec<Buffer>,
    pub expiry_height: u32, // 0 means never expires
    pub version: u32, // Transaction version calculated by Rust
}

#[napi(string_enum)]
pub enum NapiNetwork {
    Main,
    Regtest,
    Test,
}

// Helper functions to convert between internal and NAPI types
impl From<UTXO> for NapiUTXO {
    fn from(utxo: UTXO) -> Self {
        NapiUTXO {
            txid: utxo.txid,
            height: utxo.height,
            vout: utxo.vout,
            script: utxo.script,
            value: utxo.value as f64, // Convert u64 to f64
        }
    }
}

impl From<NapiUTXO> for UTXO {
    fn from(napi_utxo: NapiUTXO) -> Self {
        UTXO {
            txid: napi_utxo.txid,
            height: napi_utxo.height,
            vout: napi_utxo.vout,
            script: napi_utxo.script,
            value: napi_utxo.value as u64, // Convert f64 to u64
        }
    }
}

impl From<Output> for NapiOutput {
    fn from(output: Output) -> Self {
        NapiOutput {
            address: output.address,
            amount: output.amount as f64, // Convert u64 to f64
            memo: output.memo,
        }
    }
}

impl From<NapiOutput> for Output {
    fn from(napi_output: NapiOutput) -> Self {
        Output {
            address: napi_output.address,
            amount: napi_output.amount as u64, // Convert f64 to u64
            memo: napi_output.memo,
        }
    }
}

impl From<PartialTx> for NapiPartialTx {
    fn from(ptx: PartialTx) -> Self {
        NapiPartialTx {
            height: ptx.height,
            txid: Buffer::from(ptx.txid),
            inputs: ptx.inputs.into_iter().map(|u| u.into()).collect(),
            outputs: ptx.outputs.into_iter().map(|o| o.into()).collect(),
            fee: ptx.fee as f64, // Convert u64 to f64
            sighashes: ptx.sighashes.into_iter().map(Buffer::from).collect(),
            expiry_height: ptx.expiry_height,
            version: ptx.version,
        }
    }
}

impl From<NapiPartialTx> for PartialTx {
    fn from(napi_ptx: NapiPartialTx) -> Self {
        PartialTx {
            height: napi_ptx.height,
            txid: napi_ptx.txid.to_vec(),
            inputs: napi_ptx.inputs.into_iter().map(|u| u.into()).collect(),
            outputs: napi_ptx.outputs.into_iter().map(|o| o.into()).collect(),
            fee: napi_ptx.fee as u64, // Convert f64 to u64
            sighashes: napi_ptx.sighashes.into_iter().map(|b| b.to_vec()).collect(),
            expiry_height: napi_ptx.expiry_height,
            version: napi_ptx.version,
        }
    }
}

impl From<Network> for NapiNetwork {
    fn from(network: Network) -> Self {
        match network {
            Network::Main => NapiNetwork::Main,
            Network::Regtest => NapiNetwork::Regtest,
            Network::Test => NapiNetwork::Test,
        }
    }
}

impl From<NapiNetwork> for Network {
    fn from(napi_network: NapiNetwork) -> Self {
        match napi_network {
            NapiNetwork::Main => Network::Main,
            NapiNetwork::Regtest => Network::Regtest,
            NapiNetwork::Test => Network::Test,
        }
    }
}

// Convert ZecError to NAPI Error
fn zec_error_to_napi(err: ZecError) -> napi::Error {
    napi::Error::new(napi::Status::GenericFailure, format!("{:?}", err))
}

// NAPI exported functions

#[napi]
pub fn init_zec_lib() -> napi::Result<()> {
    init_zec().map_err(zec_error_to_napi)
}

#[napi]
pub fn build_partial_tx(
    vault: Buffer,
    ptx: NapiPartialTx,
    network: NapiNetwork,
) -> napi::Result<NapiPartialTx> {
    let internal_ptx: PartialTx = ptx.into();
    let internal_network: Network = network.into();
    
    let result = build_ptx(vault.to_vec(), internal_ptx, internal_network)
        .map_err(zec_error_to_napi)?;
    
    Ok(result.into())
}

#[napi]
pub fn apply_tx_signatures(
    vault: Buffer,
    ptx: NapiPartialTx,
    signatures: Vec<Buffer>,
    network: NapiNetwork,
) -> napi::Result<Buffer> {
    let internal_ptx: PartialTx = ptx.into();
    let internal_network: Network = network.into();
    let sig_bytes: Vec<Vec<u8>> = signatures.into_iter().map(|b| b.to_vec()).collect();
    
    let result = apply_signatures(vault.to_vec(), internal_ptx, sig_bytes, internal_network)
        .map_err(zec_error_to_napi)?;
    
    Ok(Buffer::from(result))
}

#[napi]
pub fn get_output_viewing_key(vault: Buffer) -> napi::Result<Buffer> {
    let result = get_ovk(vault.to_vec())
        .map_err(zec_error_to_napi)?;
    
    Ok(Buffer::from(result))
}

#[napi]
pub fn compute_transaction_id(
    vault: Buffer,
    ptx: NapiPartialTx,
    network: NapiNetwork,
) -> napi::Result<String> {
    let internal_ptx: PartialTx = ptx.into();
    let internal_network: Network = network.into();
    
    let result = compute_txid(vault.to_vec(), internal_ptx, internal_network)
        .map_err(zec_error_to_napi)?;
    
    Ok(result)
}

#[napi]
pub fn validate_zec_address(address: String, network: NapiNetwork) -> napi::Result<()> {
    let internal_network: Network = network.into();
    
    validate_address(address, internal_network)
        .map_err(zec_error_to_napi)
}