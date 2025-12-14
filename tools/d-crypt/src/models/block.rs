use serde::{Serialize, Deserialize};
use sha2::{Sha256, Digest};
use chrono::{DateTime, Utc};
use uuid::Uuid;

#[derive(Serialize, Deserialize, Debug, Clone)]
pub enum ActionType {
    Init,
    Encrypt,
    DecryptSuccess,
    DecryptAttemptFail, // Если флешку вставили, но других ключей не хватило
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct AuditBlock {
    pub index: u64,
    pub timestamp: DateTime<Utc>,
    pub action: ActionType,
    pub project_uuid: Option<Uuid>, // Может быть None для Init
    pub message: String,
    pub prev_hash: String,
    pub hash: String,
}

impl AuditBlock {
    pub fn new(
        index: u64, 
        action: ActionType, 
        project_uuid: Option<Uuid>, 
        message: String, 
        prev_hash: String
    ) -> Self {
        let timestamp = Utc::now();
        let hash = Self::calculate_hash(index, &timestamp, &action, &project_uuid, &message, &prev_hash);

        Self {
            index,
            timestamp,
            action,
            project_uuid,
            message,
            prev_hash,
            hash,
        }
    }

    pub fn calculate_hash(
        index: u64, 
        timestamp: &DateTime<Utc>, 
        action: &ActionType, 
        uuid: &Option<Uuid>, 
        msg: &str, 
        prev: &str
    ) -> String {
        let mut hasher = Sha256::new();
        hasher.update(index.to_le_bytes());
        hasher.update(timestamp.to_rfc3339().as_bytes());
        hasher.update(format!("{:?}", action).as_bytes());
        if let Some(u) = uuid {
            hasher.update(u.as_bytes());
        }
        hasher.update(msg.as_bytes());
        hasher.update(prev.as_bytes());
        
        hex::encode(hasher.finalize())
    }
}