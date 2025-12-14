use std::path::Path;
use std::fs::{self, OpenOptions};
use anyhow::{Result, Context};
use uuid::Uuid;
use colored::*;

use crate::models::block::{AuditBlock, ActionType};

pub struct LedgerEngine;

impl LedgerEngine {
    /// Путь к файлу лога на флешке
    fn get_log_path(usb_path: &Path) -> std::path::PathBuf {
        usb_path.join(".d-crypt").join("audit").join("chain.json")
    }

    /// Инициализация журнала (Genesis Block)
    pub fn init_ledger(usb_path: &Path) -> Result<()> {
        let audit_dir = usb_path.join(".d-crypt").join("audit");
        fs::create_dir_all(&audit_dir)?;

        let genesis_block = AuditBlock::new(
            0,
            ActionType::Init,
            None,
            "Genesis Block: USB Initialized".to_string(),
            "0".repeat(64), // Genesis prev_hash is zeros
        );

        let chain = vec![genesis_block];
        let file = OpenOptions::new().write(true).create(true).open(Self::get_log_path(usb_path))?;
        serde_json::to_writer_pretty(file, &chain)?;
        
        Ok(())
    }

    /// Добавление события в журнал
    pub fn append_event(
        usb_path: &Path, 
        action: ActionType, 
        project_uuid: Option<Uuid>, 
        message: String
    ) -> Result<()> {
        let log_path = Self::get_log_path(usb_path);
        
        // Читаем текущую цепь
        let mut chain: Vec<AuditBlock> = if log_path.exists() {
            let file = fs::File::open(&log_path)?;
            serde_json::from_reader(file).unwrap_or_else(|_| Vec::new())
        } else {
            Vec::new()
        };

        // Определяем параметры нового блока
        let index = chain.len() as u64;
        let prev_hash = if let Some(last) = chain.last() {
            last.hash.clone()
        } else {
            "0".repeat(64) // Если файл был поврежден/пуст
        };

        let new_block = AuditBlock::new(index, action, project_uuid, message, prev_hash);
        chain.push(new_block);

        // Записываем обратно
        let file = OpenOptions::new().write(true).truncate(true).create(true).open(&log_path)?;
        serde_json::to_writer_pretty(file, &chain)?;

        Ok(())
    }

    /// Чтение и валидация журнала с одной флешки
    pub fn read_and_validate(usb_path: &Path) -> Result<Vec<AuditBlock>> {
        let log_path = Self::get_log_path(usb_path);
        if !log_path.exists() {
            return Ok(Vec::new());
        }

        let file = fs::File::open(&log_path)?;
        let chain: Vec<AuditBlock> = serde_json::from_reader(file)?;

        // Валидация цепочки
        for i in 1..chain.len() {
            let prev = &chain[i-1];
            let curr = &chain[i];

            if curr.prev_hash != prev.hash {
                eprintln!("{} TAMPERING DETECTED on {:?}: Block #{} prev_hash mismatch!", "🚨".red(), usb_path, curr.index);
            }
            
            let recalc_hash = AuditBlock::calculate_hash(
                curr.index, &curr.timestamp, &curr.action, &curr.project_uuid, &curr.message, &curr.prev_hash
            );
            
            if recalc_hash != curr.hash {
                eprintln!("{} CORRUPTION DETECTED on {:?}: Block #{} hash invalid!", "🚨".red(), usb_path, curr.index);
            }
        }

        Ok(chain)
    }
}