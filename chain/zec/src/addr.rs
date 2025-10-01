use crate::config::get_config;
use crate::error::ZecError;
use tracing::{debug, info, warn}; // Import tracing macros
#[cfg(feature = "uniffi")]
use uniffi::{export, Enum};
use zcash_keys::address::Address;
use zcash_primitives::consensus::Parameters;
use crate::network::Network;

pub fn validate_address(address: String, network: Network) -> Result<(), ZecError> {
    let r: Option<Address> = Address::decode(&network, &address);
    // Match on the result of decode
    match r {
        Some(Address::Tex(_)) => {
            warn!(addr = %address, "Decoded as TEX address (considered invalid)");
            // treat TEX address as invalid
            Err(ZecError::GenericError(format!("Invalid address type (TEX): {}", address)))
        }
        Some(Address::Transparent(_transparent_addr)) => {
            // transparent address types
            debug!(addr = %address, kind = "Transparent", "Decoded as a valid transparent address");
            Ok(())
        }
        Some(_) => {
            debug!(addr = %address, "Decoded as valid but unsupported address");
            Err(ZecError::GenericError(format!("Unsupported address type: {}", address)))
        }
        None => {
            // decoding failed entirely (bad format, checksum, etc.)
            info!(addr = %address, "Failed to decode address (invalid format/checksum)");
            Err(ZecError::GenericError(format!("Invalid address format or checksum: {}", address)))
        }
    }
}
