use napi_derive::napi;

#[napi]
pub fn test_addon() -> String {
    "Hello from NAPI!".to_string()
}