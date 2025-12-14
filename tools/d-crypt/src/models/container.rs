use serde::{Serialize, Deserialize};
use uuid::Uuid;

#[derive(Serialize, Deserialize, Debug)]
pub struct EncryptedProject {
    pub version: u8,
    pub project_uuid: Uuid,
    pub nonce: [u8; 12],
    pub data: Vec<u8>,
    pub original_name: String,
    pub threshold: u8, // <--- НОВОЕ ПОЛЕ: сколько ключей нужно для открытия
}

impl EncryptedProject {
    pub fn new(uuid: Uuid, nonce: [u8; 12], data: Vec<u8>, name: String, threshold: u8) -> Self {
        Self {
            version: 1,
            project_uuid: uuid,
            nonce,
            data,
            original_name: name,
            threshold,
        }
    }
}