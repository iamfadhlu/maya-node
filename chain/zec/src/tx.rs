use crate::config::{get_config, Config};
use crate::error::ZecError;
use crate::network::Network;

use std::{
    cmp::{max, min},
    str::FromStr as _,
};

use anyhow::anyhow;
use blake2b_simd::Params;

// Remove all sapling and orchard imports
use rand_chacha::ChaCha20Rng;
use rand_core::{OsRng, SeedableRng};
use secp256k1::{ecdsa::Signature, All, PublicKey, Secp256k1, SecretKey};
use serde::{Deserialize, Serialize};
use zcash_keys::{
    address::{Address, Receiver},
    encoding::AddressCodec,
};
use zcash_primitives::transaction::{
    fees::zip317,
    sighash::{signature_hash, SignableInput},
    txid::TxIdDigester,
    Authorized, // Needed for bundle types after proving
    TransactionData,
    TxVersion,
    Unauthorized, // Top-level marker
};
use zcash_transparent::{
    // Replaces zcash_primitives::legacy::Script
    address::Script,
    // Replaces zcash_primitives::legacy::TransparentAddress
    address::TransparentAddress,
    // Contains the new TransparentBuilder
    builder::TransparentBuilder,
    // Replaces zcash_primitives::transaction::components::transparent::Bundle
    bundle::Bundle as TransparentBundle,
    // Replaces zcash_primitives::transaction::components::OutPoint and TxOut
    bundle::{OutPoint, TxOut},
    sighash::SighashType,
    // May need the inner SignableInput later
    sighash::SignableInput as TransparentSignableInputData,
    // Replaces zcash_primitives::transaction::sighash::SIGHASH_ALL
    sighash::SIGHASH_ALL,
};

// Removed Sapling Local Prover import
use zcash_protocol::{
    consensus::{BlockHeight, BranchId},
    memo::{Memo, MemoBytes},
    value::{ZatBalance as Amount, Zatoshis}, // Use Amount alias for ZatBalance
};

use hex;
use tracing::{debug, info, warn};

pub struct TxBytes {
    pub txid: String,
    pub data: Vec<u8>,
}

pub type Sighash = Vec<u8>;
pub type Sighashes = Vec<Sighash>;

#[derive(Serialize, Debug)]
pub struct PartialTx {
    pub height: u32,
    pub txid: Vec<u8>,
    pub inputs: Vec<UTXO>,
    pub outputs: Vec<Output>,
    pub fee: u64,
    #[serde(skip)]
    pub sighashes: Sighashes,
    pub expiry_height: u32, // 0 means never expires
    pub version: u32, // Transaction version calculated from consensus rules
}

#[derive(Clone, Serialize, Deserialize, Debug)]
pub struct UTXO {
    pub txid: String,
    pub height: u32,
    #[serde(rename = "outputIndex")]
    pub vout: u32,
    pub script: String,
    #[serde(rename = "satoshis")]
    pub value: u64,
}

#[derive(Clone, Serialize, Default, Debug)]
pub struct Output {
    pub address: String,
    pub amount: u64,
    pub memo: String,
}

pub fn build_ptx(
    vault: Vec<u8>,
    mut ptx: PartialTx,
    network: Network,
) -> Result<PartialTx, ZecError> {
    let unauthed_tx = build_unauthorized_tx(vault, &ptx, network)?;

    // Calculate and set the TXID
    let txid_parts = unauthed_tx.digest(TxIdDigester);
    let txid = signature_hash(&unauthed_tx, &SignableInput::Shielded, &txid_parts)
        .as_ref()
        .to_vec();
    tracing::info!("calculated txid (build_ptx) {}", hex::encode(&txid));
    ptx.txid = txid;

    // The build_unauthorized_tx already determined the version and included it in unauthed_tx
    let version = unauthed_tx.version();
    ptx.version = version.header();
    tracing::info!("Transaction version extracted: {:#x}", ptx.version);

    // Calculate sighashes
    let mut sighashes = vec![];
    for (index, inp) in ptx.inputs.iter().enumerate() {
        let value = Zatoshis::from_u64(inp.value)
            .map_err(|e| ZecError::InvalidAmount(inp.value, e.into()))?;

        let script = Script(hex::decode(&inp.script).map_err(|e| {
            ZecError::GenericError(format!("Invalid script hex for input {}: {}", index, e))
        })?);

        // Log the script details for debugging
        tracing::info!(
            "Input {} - value: {}, script_hex: {}",
            index,
            inp.value,
            inp.script
        );

        // For P2PKH, the script_code is the script without the leading byte (length) and 
        // the two trailing bytes (CHECKSIG). This might vary depending on script type.
        // For now, use the full script for both.
        let transparent_input_data = TransparentSignableInputData::from_parts(
            SighashType::ALL, // Use SIGHASH_ALL constant to sign all inputs and outputs
            index,
            &script, // script_code (this is the script being executed)
            &script, // script_pubkey (for validation)
            value,
        );

        // Get the sighash - this needs to match what the signer expects
        let sighash = signature_hash(
            &unauthed_tx,
            &SignableInput::Transparent(transparent_input_data),
            &txid_parts,
        )
        .as_ref()
        .to_vec();
        
        tracing::info!("Generated sighash for input {}: {}", index, hex::encode(&sighash));
        sighashes.push(sighash);
    }
    ptx.sighashes = sighashes;

    tracing::debug!(
        "returning txid in ptx (build_ptx) {}",
        hex::encode(&ptx.txid)
    );
    Ok(ptx)
}

pub fn apply_signatures(
    vault: Vec<u8>,
    ptx: PartialTx,
    signatures_der: Vec<Vec<u8>>,
    network: Network,
) -> Result<Vec<u8>, ZecError> {
    // Build the initial unauthorized transaction
    let unauthed_tx = build_unauthorized_tx(vault, &ptx, network)?;

    // Prepare secp256k1 signatures from DER format
    tracing::info!("Processing {} DER signature(s)", signatures_der.len());
    for (i, sig_der) in signatures_der.iter().enumerate() {
        tracing::info!("DER Signature {}: length={}, hex={}", i, sig_der.len(), hex::encode(sig_der));
    }
    
    // Prepare secp256k1 signatures from DER format
    let secp_signatures = signatures_der
        .iter()
        .map(|s| Signature::from_der(&s))
        .collect::<Result<Vec<_>, _>>()
        .map_err(|e| ZecError::GenericError(format!("invalid DER signature(s): {}", e)))?;

    // Apply the signatures to the transaction - transparent only, using new flow
    let signed_transparent_bundle_opt: Option<zcash_transparent::bundle::Bundle<zcash_transparent::bundle::Authorized>>;

    if let Some(unauth_transparent_bundle) = unauthed_tx.transparent_bundle().cloned() {
        // Define the sighash calculation logic as a closure.
        let txid_parts = unauthed_tx.digest(TxIdDigester);
        let calculate_sighash = |signable_input: TransparentSignableInputData| -> [u8; 32] {
            let hash = signature_hash(
                &unauthed_tx,
                &SignableInput::Transparent(signable_input),
                &txid_parts,
            );
            let hash_slice: &[u8] = hash.as_ref();
            hash_slice
                .try_into()
                .expect("signature_hash must be 32 bytes")
        };

        // Prepare the signing context by passing the closure.
        let secp = Secp256k1::verification_only();
        let mut context = unauth_transparent_bundle
            .prepare_transparent_signatures(calculate_sighash, &secp)
            .map_err(|e: zcash_transparent::builder::Error| ZecError::GenericError(format!("Failed to prepare transparent signatures: {}", e)))?;

        // Append external signatures
        context = context
            .append_external_signatures(&secp_signatures)
            .map_err(|e: zcash_transparent::builder::Error| ZecError::GenericError(format!("Failed to append external transparent signatures: {}", e)))?;
        tracing::info!(
            "Appended {} secp256k1 signatures to transparent context.",
            secp_signatures.len()
        );
        
        // Finalize the signed transparent bundle
        let signed_bundle = context.finalize_signatures()
            .map_err(|e: zcash_transparent::builder::Error| {
                ZecError::GenericError(format!("Failed to finalize transparent signatures: {}", e))
            })?;
        tracing::info!("Transparent signatures finalized.");
        signed_transparent_bundle_opt = Some(signed_bundle);
    } else {
        // No transparent bundle at all
        tracing::info!("No transparent bundle in the transaction.");
        if !secp_signatures.is_empty() {
            return Err(ZecError::GenericError("Signatures provided but no transparent bundle in transaction".into()))
        }
        signed_transparent_bundle_opt = None;
    }
    
    // Reconstruct the transaction with the signed transparent bundle
    // and other bundles from the original unauthed_tx
    let tx_data = TransactionData::from_parts(
        unauthed_tx.version(),
        unauthed_tx.consensus_branch_id(),
        unauthed_tx.lock_time(),
        unauthed_tx.expiry_height(),
        signed_transparent_bundle_opt,
        None, // unauthed_tx.sprout_bundle().cloned(),
        None, // unauthed_tx.sapling_bundle().cloned(),
        None, // unauthed_tx.orchard_bundle().cloned(),
    );

    // Serialize the signed transaction
    let tx = tx_data
        .freeze()
        .map_err(|e| ZecError::GenericError(format!("fail to freeze tx_data: {}", e)))?;
    let mut buffer = vec![];
    tx.write(&mut buffer)
        .map_err(|e| ZecError::GenericError(format!("fail to write tx_data to buffer: {}", e)))?;

    Ok(buffer)
}

fn decode_hexstring(s: &str) -> Result<Vec<u8>, ZecError> {
    hex::decode(s).map_err(|_| ZecError::GenericError("Invalid Hex string".into()))
}

fn to_ba<const N: usize>(v: &[u8]) -> Result<[u8; N], ZecError> {
    let v: Result<[u8; N], _> = v.try_into();
    v.map_err(|e| {
        ZecError::GenericError(format!(
            "fail to convert slice to array of size {}, err: {}",
            N, e
        ))
    })
}

fn to_hash(s: &str) -> Result<[u8; 32], ZecError> {
    let mut v = decode_hexstring(s)?;
    v.reverse();
    to_ba(&v)
}

pub fn get_ovk(pubkey: Vec<u8>) -> Result<Vec<u8>, ZecError> {
    let hash = blake2b_simd::Params::new()
        .hash_length(32)
        .personal(b"Zcash_Maya_OVK__")
        .hash(&pubkey);
    let ovk = hash.as_bytes().to_vec();
    Ok(ovk)
}

fn build_unauthorized_tx(
    vault: Vec<u8>,
    ptx: &PartialTx,
    network: Network,
) -> Result<TransactionData<zcash_primitives::transaction::Unauthorized>, ZecError> {
    // let _config: &Config = get_config()?;
    let data = bincode::serialize(ptx)
        .map_err(|e| ZecError::GenericError(format!("fail to serialize ptx: {}", e)))?;
    let tx_seed = Params::new().hash_length(32).hash(&data);
    // No need for RNG since we're not using Sapling/Orchard
    let _tx_rng = ChaCha20Rng::from_seed(
        to_ba(tx_seed.as_bytes())
            .map_err(|_| ZecError::GenericError("invalid RNG seed".to_string()))?,
    );

    let pk = PublicKey::from_slice(&vault).map_err(|e| ZecError::InvalidVaultPubkey(e.into()))?;
    let ovk = get_ovk(vault)?;

    let mut tbuilder = TransparentBuilder::empty();
    for i in ptx.inputs.iter() {
        let UTXO {
            txid,
            vout,
            script,
            value,
            ..
        } = i;
        let op = OutPoint::new(to_hash(&txid)?, *vout);
        
        // Decode the script for logging
        let script_bytes = hex::decode(script)
            .map_err(|e| ZecError::GenericError(format!("Invalid script hex: {}", e)))?;
        
        let coin = TxOut {
            value: Zatoshis::from_u64(*value)
                .map_err(|e| ZecError::InvalidAmount(*value, e.into()))?,
            script_pubkey: Script(script_bytes.clone()),
        };
        
        let op_clone = op.clone();
        let coin_clone = coin.clone();
        
        // Log more details about the input we're adding
        tracing::info!(
            "Adding input - txid: {}, vout: {}, script_hex: {}, value: {}",
            txid, 
            vout, 
            script, 
            value
        );
        tracing::debug!(
            "tbuilder.add_input(without_sk) - pk: {:?}, op: {:?}, coin: {:?} ",
            pk,
            op,
            coin
        );
        tbuilder
            .add_input(pk, op.clone(), coin.clone())
            .map_err(|e| ZecError::GenericError(format!("Failed to add transparent input, err: {}, inputs: pk: {:?}, op: {:?}, coin: {:?}[{:?}/{:?}]", e, pk, op_clone, coin_clone, coin_clone.value, coin_clone.script_pubkey)))?;
    }

    // Commented out Sapling and Orchard builders as we're not using them at the moment
    /*
    let mut sbuilder = sapling_crypto::builder::Builder::new(
        Zip212Enforcement::On,
        sapling_crypto::builder::BundleType::Transactional {
            bundle_required: false,
        },
        Anchor::empty_tree(), // not required when there are no shielded input
    );
    let mut obuilder = orchard::builder::Builder::new(
        BundleType::Transactional {
            flags: Flags::ENABLED,
            bundle_required: false,
        },
        orchard::Anchor::empty_tree(),
    );
    */
    // Create mutable dummy values that will satisfy the type checker
    let mut sbuilder = ();
    let mut obuilder = ();

    for o in ptx.outputs.iter() {
        let Output {
            address,
            amount,
            memo,
        } = o;
        let recipient =
            Address::decode(&network, &address).ok_or(ZecError::InvalidAddress(address.clone()))?;

        let mut hr = |receiver: Receiver| {
            handle_receiver(
                receiver,
                *amount,
                &memo,
                &ovk,
                &mut tbuilder,
                &mut sbuilder,
                &mut obuilder,
            )
        };

        match recipient {
            Address::Tex(_) => Err(ZecError::InvalidAddress(address.clone())),
            Address::Transparent(transparent_address) => {
                hr(Receiver::Transparent(transparent_address))
            }
            Address::Sapling(_) => Err(ZecError::GenericError(
                "Sapling addresses are not supported".into(),
            )),
            Address::Unified(unified_address) => {
                // Only check for transparent component since we've removed Sapling and Orchard support
                if let Some(&receiver) = unified_address.transparent() {
                    hr(Receiver::Transparent(receiver))
                } else {
                    Err(ZecError::GenericError(
                        "only transparent address components are supported".into(),
                    ))
                }
            }
        }?;
    }

    let tbundle = tbuilder.build();

    // Commented out Sapling and Orchard bundle building as we're not using them at the moment
    /*
    let sbundle = sbuilder
        .build::<LocalTxProver, LocalTxProver, _, Amount>(&[], &mut tx_rng) // Pass &[]
        .map_err(|e| ZecError::GenericError(format!("fail to build sapling bundle: {}", e)))?
        .map(|(bundle, _)| {
            let prover: &LocalTxProver = &config.sapling_prover;
            bundle.create_proofs(prover, prover, &mut tx_rng, ())
        });
    let obundle = obuilder
        .build::<Amount>(&mut tx_rng)
        .map_err(|e| ZecError::GenericError(format!("fail to build orchard bundle: {}", e)))?
        .map(|v| v.0);
    */

    // Use None for Sapling and Orchard bundles
    let sbundle = None;
    let obundle = None;

    let height = BlockHeight::from_u32(ptx.height);
    let consensus_branch_id = BranchId::for_height(&network, height);
    
    // Use the explicitly suggested version for the current network height
    let version = TxVersion::suggested_for_branch(consensus_branch_id);
    
    // Use expiry_height if provided (0 means never expires), otherwise default to height + some blocks
    let expiry_height = if ptx.expiry_height == 0 {
        BlockHeight::from_u32(0) // Never expires
    } else {
        BlockHeight::from_u32(ptx.expiry_height)
    };
    
    tracing::info!(
        "Building transaction with version: {:?}, height: {}, expiry_height: {}",
        version,
        height,
        expiry_height
    );
    
    let unauthed_tx: TransactionData<zcash_primitives::transaction::Unauthorized> =
        TransactionData::from_parts(
            version,
            consensus_branch_id,
            0, // lock time
            expiry_height, // expiry height (0 = never expires)
            tbundle,
            None, // no sprout bundle
            sbundle, // no sapling bundle (None)
            obundle, // no orchard bundle (None)
        );

    Ok(unauthed_tx)
}

fn handle_receiver(
    receiver: Receiver,
    amount: u64,
    memo: &str,
    _ovk: &[u8], // Underscore to indicate it's unused
    tbuilder: &mut TransparentBuilder,
    _sbuilder: &(), // Changed to () since we're not using Sapling
    _obuilder: &(), // Changed to () since we're not using Orchard
) -> Result<(), ZecError> {
    // We keep memo parsing for transparent addresses with OP_RETURN
    // No need for memo_array since we're not using Sapling/Orchard

    match receiver {
        Receiver::Transparent(transparent_address) => {
            let amount = Zatoshis::from_u64(amount)
                .map_err(|e| ZecError::InvalidAmount(amount, e.into()))?;

            // Log output details
            tracing::info!(
                "Adding output to address: {:?} with amount: {:?}",
                transparent_address,
                amount
            );
                
            // Add the regular output
            tbuilder
                .add_output(&transparent_address, amount)
                .map_err(|e| {
                    ZecError::GenericError(format!("fail to add transparent output: {}", e))
                })?;
                
            // Handle memo as OP_RETURN
            if !memo.is_empty() {
                tracing::info!("Adding OP_RETURN memo: {}", memo);
                tbuilder
                    .add_null_data_output(memo.as_bytes())
                    .map_err(|e| {
                        ZecError::GenericError(format!("fail to add transparent memo: {}", e))
                    })?;
            }
        }
        // For Sapling addresses (and any other variants that might be added)
        _ => {
            // Return error for now since we're only supporting transparent
            return Err(ZecError::GenericError(
                "Only transparent addresses are currently supported".into(),
            ));
        }
    }

    Ok::<_, ZecError>(())
}

pub fn compute_txid(vault: Vec<u8>, ptx: PartialTx, network: Network) -> Result<String, ZecError> {
    // build the initial unauthorized transaction
    let unauthed_tx = build_unauthorized_tx(vault, &ptx, network)?;
    // calculate the tx hash
    let txid_parts = unauthed_tx.digest(TxIdDigester);
    let txid = signature_hash(&unauthed_tx, &SignableInput::Shielded, &txid_parts)
        .as_ref()
        .clone();
    let txid_hex = hex::encode(&txid);
    tracing::info!("calculated txid (compute_txid) {}", txid_hex);
    Ok(txid_hex)
}

