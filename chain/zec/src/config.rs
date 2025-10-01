// No provers needed for transparent-only implementation
use tracing::{debug, info, warn}; // Import tracing macros
use tracing_subscriber::{fmt, EnvFilter, layer::SubscriberExt, util::SubscriberInitExt, registry}; 

use crate::network::Network;
use crate::error::ZecError;
use crate::CONFIG;

pub struct Config {
    // Sapling and Orchard provers commented out as we're not using these features at the moment
    // pub sapling_prover: LocalTxProver,
    // pub orchard_prover: ProvingKey,
}

pub fn init_config() -> Result<Config, ZecError> {
    info!("Rust ZEC Initializing configuration");
    // Commented out prover initialization as we're not using Sapling or Orchard
    // let (sapling_prover, orchard_prover) = build_provers()?;
    let config = Config {
        // sapling_prover: sapling_prover,
        // orchard_prover: orchard_prover,
    };
    info!("Rust ZEC Configuration initialized successfully.");
    Ok(config)
}

pub fn get_config() -> Result<&'static Config, ZecError> {
    CONFIG.get().ok_or_else(|| ZecError::InitError("Config not initialized.".to_string()))
}

pub fn init_logger(){
    // Reads filter configuration from RUST_LOG environment variable
    // Example RUST_LOG values:
    //   RUST_LOG=info                (Show info, warn, error)
    //   RUST_LOG=debug               (Show debug, info, warn, error)
    //   RUST_LOG=trace               (Show all levels)
    //   RUST_LOG=warn,my_crate=debug (Show warn+ globally, but debug+ for your crate)
    //   RUST_LOG=error               (Show only errors)
    let filter = EnvFilter::try_from_default_env()
        .unwrap_or_else(|_| EnvFilter::new("info")); // Default to "info" if RUST_LOG not set

    // Customize the fmt::Layer
    let format_layer = fmt::layer()
    .with_level(true)     // Show log level
    .with_target(true)    // Show module path
    .with_thread_ids(true) // Show thread ID
    .with_timer(fmt::time::UtcTime::rfc_3339());
    // .with_file(true)
    // .with_line_number(true)

    match tracing_subscriber::registry()
        .with(format_layer)
        .with(filter)
        .try_init() // <-- USE THIS METHOD
    {
        Ok(()) => info!("Logger initialized successfully."),
        Err(e) => {
            // Log a warning if it's already initialized, or potentially error on other issues.
            // Using debug! or warn! is usually sufficient for the "already initialized" case.
            warn!("Logger initialization failed or logger already initialized: {}", e);
        }
    }
}

// Commented out build_provers as we're not using Sapling or Orchard
/*
fn build_provers() -> Result<(LocalTxProver, ProvingKey), ZecError> {
    info!("Initializing Zcash provers (will download/verify parameters if needed)...");

    // --- Sapling ---
    let sapling_paths = download_sapling_parameters(None).map_err(|e| ZecError::InitError(format!("fail to download/verify Zcash Sapling parameters: {}", e)))?;
    info!("Sapling parameters verified/downloaded to: spend={:?}, output={:?}", sapling_paths.spend, sapling_paths.output);

    info!("Loading Sapling prover from verified paths...");
    let sapling_prover = LocalTxProver::new(&sapling_paths.spend, &sapling_paths.output);
    info!("Sapling prover loaded.");

    // --- Orchard ---
    info!("Building Orchard proving key...");
     let orchard_prover = ProvingKey::build();
    info!("Orchard proving key built.");

    info!("Provers initialized successfully.");
    Ok((sapling_prover, orchard_prover))
}
*/