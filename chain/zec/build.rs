fn main() {
    // Only generate UniFFI scaffolding when the uniffi feature is enabled
    #[cfg(feature = "uniffi")]
    uniffi::generate_scaffolding("src/interface.udl").unwrap();
}