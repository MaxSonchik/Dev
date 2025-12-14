use aes_gcm::{
    aead::{Aead, KeyInit}, // Убрали Payload
    Aes256Gcm, Nonce
};
use rand::RngCore; // Убрали Rng
use anyhow::{Result, anyhow};

pub struct CryptoEngine;

impl CryptoEngine {
    pub fn generate_key() -> [u8; 32] {
        let mut key = [0u8; 32];
        rand::thread_rng().fill_bytes(&mut key);
        key
    }

    pub fn encrypt(data: &[u8], key: &[u8; 32]) -> Result<(Vec<u8>, [u8; 12])> {
        let cipher = Aes256Gcm::new(key.into());
        
        let mut nonce_bytes = [0u8; 12];
        rand::thread_rng().fill_bytes(&mut nonce_bytes);
        let nonce = Nonce::from_slice(&nonce_bytes);

        let ciphertext = cipher.encrypt(nonce, data)
            .map_err(|e| anyhow!("Encryption failed: {}", e))?;

        Ok((ciphertext, nonce_bytes))
    }

    pub fn decrypt(encrypted_data: &[u8], key: &[u8; 32], nonce: &[u8; 12]) -> Result<Vec<u8>> {
        let cipher = Aes256Gcm::new(key.into());
        let nonce = Nonce::from_slice(nonce);

        let plaintext = cipher.decrypt(nonce, encrypted_data)
            .map_err(|e| anyhow!("Decryption failed (wrong key or corrupted data): {}", e))?;

        Ok(plaintext)
    }
}