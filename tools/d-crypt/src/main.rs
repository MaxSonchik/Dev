use clap::{Parser, Subcommand};
use std::path::PathBuf;
use std::fs; 
use uuid::Uuid;
use colored::*;
use anyhow::{Result, anyhow, Context};
use sysinfo::{DiskExt, System, SystemExt};
use inquire::{MultiSelect, Confirm};

mod core;
mod archive; 
mod models;

use core::crypto::CryptoEngine;
use core::shamir::ShamirEngine;
use core::ledger::LedgerEngine; // Новый модуль
use archive::archiver::Archiver;
use models::container::EncryptedProject;
use models::block::{ActionType, AuditBlock}; // Типы для логов

#[derive(Parser)]
#[command(name = "d-crypt")]
#[command(about = "Physical Multi-Sig Project Encryption", long_about = None)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    Init, 
    Encrypt {
        #[arg(short, long)]
        input: PathBuf,
        #[arg(short, long)]
        output: Option<PathBuf>,
        #[arg(short = 't', long)]
        threshold: Option<u8>,
        #[arg(short = 'n', long)]
        total: Option<u8>,
        #[arg(long, num_args = 1.., value_delimiter = ' ')]
        keys: Option<Vec<PathBuf>>,
    },
    Decrypt {
        #[arg(short, long)]
        input: PathBuf,
        #[arg(short, long)]
        output: Option<PathBuf>,
        #[arg(long, num_args = 1.., value_delimiter = ' ')]
        keys: Option<Vec<PathBuf>>,
    },
}

// ... (Функция select_usb_drives осталась без изменений, скопируй из прошлого ответа) ...
fn select_usb_drives(count_needed: Option<u8>) -> Result<Vec<PathBuf>> {
    let mut sys = System::new_all();
    sys.refresh_disks();
    let mut choices = Vec::new();
    let mut disks_map = Vec::new();
    for disk in sys.disks() {
        let mount = disk.mount_point();
        let mount_str = mount.to_string_lossy();
        if mount_str == "/" || mount_str.starts_with("/boot") || mount_str.starts_with("/home") || mount_str.starts_with("/var") || mount_str.starts_with("/usr") || mount_str.starts_with("/etc") || mount_str.starts_with("/snap") { continue; }
        let label = format!("{} ({:?}) - {} GB", disk.name().to_string_lossy(), mount, disk.total_space() / 1024 / 1024 / 1024);
        choices.push(label);
        disks_map.push(mount.to_path_buf());
    }
    if choices.is_empty() { return Err(anyhow!("No mounted drives found!")); }
    let msg = if let Some(n) = count_needed { format!("Select {} USB drives:", n) } else { "Select USB drive(s):".to_string() };
    let selection = MultiSelect::new(&msg, choices).prompt()?;
    if let Some(n) = count_needed { if selection.len() != n as usize { return Err(anyhow!("Incorrect selection count")); } }
    let mut selected_paths = Vec::new();
    for item in selection {
        let idx = disks_map.iter().enumerate().find(|(_, path)| {
             let d = sys.disks().iter().find(|d| d.mount_point() == *path).unwrap();
             let l = format!("{} ({:?}) - {} GB", d.name().to_string_lossy(), d.mount_point(), d.total_space() / 1024 / 1024 / 1024);
            l == item
        }).map(|(i, _)| i).unwrap();
        selected_paths.push(disks_map[idx].clone());
    }
    Ok(selected_paths)
}

// === ВСПОМОГАТЕЛЬНАЯ ФУНКЦИЯ ДЛЯ ВЫВОДА ЛОГОВ ===
fn print_audit_report(logs: Vec<(PathBuf, AuditBlock)>) {
    println!("\n{}", "📜 AUDIT LEDGER REPORT (Combined History)".bold().underline());
    
    // Сортируем по времени (новые внизу)
    let mut sorted_logs = logs;
    sorted_logs.sort_by_key(|(_, block)| block.timestamp);

    for (source, block) in sorted_logs {
        let date_str = block.timestamp.format("%Y-%m-%d %H:%M:%S").to_string();
        let icon = match block.action {
            ActionType::Init => "🔌".blue(),
            ActionType::Encrypt => "🔒".green(),
            ActionType::DecryptSuccess => "🔓".green(),
            ActionType::DecryptAttemptFail => "⚠️".red(),
        };
        
        let usb_name = source.file_name().unwrap_or_default().to_string_lossy();
        
        println!("{} [{}] [{}] {:<20} | {}", 
            icon, 
            date_str.dimmed(), 
            usb_name.cyan(), 
            format!("{:?}", block.action), 
            block.message
        );
    }
    println!();
}

fn main() -> Result<()> {
    let cli = Cli::parse();

    match &cli.command {
        Commands::Init => {
            println!("{} USB Initialization Mode", "🔌".blue());
            let usb_paths = select_usb_drives(None)?;
            
            for path in usb_paths {
                let keys_dir = path.join(".d-crypt").join("keys");
                fs::create_dir_all(&keys_dir).context("Failed to create d-crypt structure")?;
                
                // ИНИЦИАЛИЗАЦИЯ ЛЕДЖЕРА
                LedgerEngine::init_ledger(&path)?;
                
                println!("{} Initialized USB & Ledger at {:?}", "✔".green(), path);
            }
            Ok(())
        }
        
        Commands::Encrypt { input, output, threshold, total, keys } => {
            if !input.exists() { return Err(anyhow!("Input not found")); }

            let n = total.unwrap_or(2);
            let m = threshold.unwrap_or(2);

            let selected_keys = if let Some(k) = keys { k.clone() } else { select_usb_drives(Some(n))? };

            println!("   Archiving & Encrypting...");
            let compressed_data = Archiver::compress_directory(&input)?;
            let project_key = CryptoEngine::generate_key();
            let project_uuid = Uuid::new_v4();
            let (encrypted_data, nonce) = CryptoEngine::encrypt(&compressed_data, &project_key)?;

            let project_name = input.file_name().unwrap_or_default().to_string_lossy().to_string();
            let container = EncryptedProject::new(project_uuid, nonce, encrypted_data, project_name.clone(), m);
            
            let output_path = output.clone().unwrap_or_else(|| {
                let mut p = input.clone();
                p.set_extension("dcr");
                p
            });
            
            let encoded_container = bincode::serialize(&container)?;
            fs::write(&output_path, encoded_container)?;
            
            let shards = ShamirEngine::split_secret(&project_key, m, n)?;

            for (i, usb_path) in selected_keys.iter().enumerate() {
                let key_dir = usb_path.join(".d-crypt").join("keys");
                if !key_dir.exists() { let _ = fs::create_dir_all(&key_dir); }
                
                let shard_path = key_dir.join(format!("{}.shard", project_uuid));
                fs::write(&shard_path, &shards[i])?;
                
                // ЛОГИРОВАНИЕ
                LedgerEngine::append_event(
                    usb_path, 
                    ActionType::Encrypt, 
                    Some(project_uuid), 
                    format!("Encrypted project: {}", project_name)
                ).ok(); // Игнорируем ошибки логгирования, чтобы не прерывать процесс
                
                println!("   {} Key part #{} written", "💾".green(), i + 1);
            }

            println!("\n{} Encrypted to {:?} (UUID: {})", "✔".green(), output_path, project_uuid);
            Ok(())
        }

        Commands::Decrypt { input, output, keys } => {
            println!("{} Reading container {:?}", "🔓".blue(), input);
            let file_data = fs::read(input)?;
            let container: EncryptedProject = bincode::deserialize(&file_data)?;

            println!("   Target: {} (Need {} keys)", container.original_name, container.threshold);

            let mut found_shards = Vec::new();
            let mut found_paths = Vec::new();

            // 1. Поиск ключей (код поиска опущен для краткости, он такой же)
            // Для примера - берем логику автопоиска из прошлого шага
            let mut sys = System::new_all();
            sys.refresh_disks();
            if let Some(manual) = keys {
                for p in manual { if p.exists() { found_paths.push(p.clone()); } }
            } else {
                for disk in sys.disks() {
                    let mp = disk.mount_point();
                    let sp = mp.join(".d-crypt").join("keys").join(format!("{}.shard", container.project_uuid));
                    if sp.exists() { found_paths.push(mp.to_path_buf()); }
                }
            }

            // 2. Сбор аудита со всех найденных флешек
            let mut aggregated_logs = Vec::new();
            for path in &found_paths {
                if let Ok(logs) = LedgerEngine::read_and_validate(path) {
                    for block in logs {
                        aggregated_logs.push((path.clone(), block));
                    }
                }
            }
            // Показываем отчет ПЕРЕД расшифровкой
            if !aggregated_logs.is_empty() {
                print_audit_report(aggregated_logs);
            }

            // 3. Проверка количества
            if found_paths.len() < container.threshold as usize {
                // ЛОГИРУЕМ НЕУДАЧУ на те флешки, что нашли
                for path in &found_paths {
                    LedgerEngine::append_event(
                        path, 
                        ActionType::DecryptAttemptFail, 
                        Some(container.project_uuid), 
                        format!("Not enough keys. Found {}, Need {}", found_paths.len(), container.threshold)
                    ).ok();
                }
                return Err(anyhow!("Not enough keys found!"));
            }

            // 4. Подтверждение
            if !Confirm::new("Proceed?").with_default(true).prompt()? { return Ok(()); }

            // 5. Чтение шардов
            for path in &found_paths {
                let sp = path.join(".d-crypt").join("keys").join(format!("{}.shard", container.project_uuid));
                found_shards.push(fs::read(sp)?);
            }

            // 6. Восстановление
            let shards_to_use = &found_shards[0..container.threshold as usize];
            let master_key_vec = ShamirEngine::recover_secret(shards_to_use, container.threshold)?;
            let mut master_key = [0u8; 32];
            master_key.copy_from_slice(&master_key_vec);

            let decrypted = CryptoEngine::decrypt(&container.data, &master_key, &container.nonce)?;
            let out_dir = output.clone().unwrap_or_else(|| PathBuf::from(&container.original_name));
            Archiver::decompress_to(&decrypted, &out_dir)?;

            // ЛОГИРУЕМ УСПЕХ НА ВСЕ ФЛЕШКИ
            for path in &found_paths {
                LedgerEngine::append_event(
                    path, 
                    ActionType::DecryptSuccess, 
                    Some(container.project_uuid), 
                    format!("Decrypted successfully to {:?}", out_dir)
                ).ok();
            }

            println!("\n{} Restored!", "✔".green());
            Ok(())
        }
    }
}