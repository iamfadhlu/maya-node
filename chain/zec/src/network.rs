use zcash_protocol::{
    consensus::{BlockHeight, MainNetwork, TestNetwork, NetworkUpgrade, Parameters},
    local_consensus::LocalNetwork,
};
use zcash_primitives::consensus::NetworkType;

// Defines a custom Network enum encompassing Main, Test, and Regtest variants,
// primarily designed to implement the `Parameters` trait needed by some Zcash functions,
// mapping Regtest to a specific set of local consensus rules.

#[derive(Copy, Clone, Debug)]
pub enum Network {
    Main,
    Test,
    Regtest,

}

impl Parameters for Network {
    fn network_type(&self) -> NetworkType {
        match self {
            Network::Main => MainNetwork.network_type(),
            Network::Test => TestNetwork.network_type(),
            Network::Regtest => REGTEST.network_type(),
        }
    }

    fn activation_height(
        &self,
        nu: NetworkUpgrade,
    ) -> Option<zcash_protocol::consensus::BlockHeight> {
        match self {
            Network::Main => MainNetwork.activation_height(nu),
            Network::Test => TestNetwork.activation_height(nu),
            Network::Regtest => REGTEST.activation_height(nu),
        }
    }
}

pub const REGTEST: LocalNetwork = LocalNetwork {
    // network_type_override: Some(NetworkType::Regtest), // Explicitly set the type
    overwinter: Some(BlockHeight::from_u32(1)),
    sapling: Some(BlockHeight::from_u32(1)),
    blossom: Some(BlockHeight::from_u32(1)),
    heartwood: Some(BlockHeight::from_u32(1)),
    canopy: Some(BlockHeight::from_u32(1)),
    nu5: Some(BlockHeight::from_u32(1)),
    nu6: Some(BlockHeight::from_u32(1)),
};
