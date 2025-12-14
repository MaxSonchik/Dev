use serde::{Deserialize, Serialize};
use serde_json::Value;

// Облегченная структура для списка (хранится в памяти UI)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PacketSummary {
    pub id: usize,            // Порядковый номер (1-based)
    pub timestamp: String,
    pub src: String,
    pub dst: String,
    pub protocol: String,
    pub length: usize,
    pub info: String,         // Краткая сводка (порты, флаги)
    
    // Данные для маппинга и фильтрации
    pub src_port: u16,
    pub dst_port: u16,
    
    // Stratoshark Data
    pub process_name: Option<String>,
    pub pid: Option<u32>,
    pub user: Option<String>,
}

// Полная структура (подгружается по запросу)
#[derive(Debug, Clone)]
pub struct PacketDetails {
    pub summary: PacketSummary,
    pub raw_data: Vec<u8>,    // Hex dump source
    pub layers: Value,        // Wireshark JSON tree
}