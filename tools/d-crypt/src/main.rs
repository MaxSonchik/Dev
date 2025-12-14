use clap::{Parser, Subcommand};
use std::path::PathBuf;
use std::fs; 
use uuid::Uuid;
use colored::*;
use anyhow::{Result, anyhow, Context};
use sysinfo::{DiskExt, System, SystemExt};
use inquire::{MultiSelect, Confirm}; // Убрали Text

mod core;
mod archive; 
mod models;

use core::crypto::CryptoEngine;
use core::shamir::ShamirEngine;
use archive::archiver::Archiver;
use models::container::EncryptedProject;

#[derive(Parser)]
#[command(name = "d-crypt")]
#[command(about = "Physical Multi-Sig Project Encryption", long_about = None)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Инициализация флешки
    Init, 
    
    /// Шифрование проекта
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
    
    /// Расшифровка
    Decrypt {
        #[arg(short, long)]
        input: PathBuf,
        
        #[arg(short, long)]
        output: Option<PathBuf>,
        
        #[arg(long, num_args = 1.., value_delimiter = ' ')]
        keys: Option<Vec<PathBuf>>,
    },
}

// --- Helper: Интерактивный выбор дисков ---
fn select_usb_drives(count_needed: Option<u8>) -> Result<Vec<PathBuf>> {
    let mut sys = System::new_all();
    sys.refresh_disks();

    let mut choices = Vec::new();
    let mut disks_map = Vec::new();

    for disk in sys.disks() {
        let mount = disk.mount_point();
        let mount_str = mount.to_string_lossy();

        // === ФИЛЬТРАЦИЯ СИСТЕМНЫХ ДИСКОВ ===
        // Игнорируем корень, boot, home и системные папки
        if mount_str == "/" 
            || mount_str.starts_with("/boot") 
            || mount_str.starts_with("/home")
            || mount_str.starts_with("/var")
            || mount_str.starts_with("/usr")
            || mount_str.starts_with("/etc")
            || mount_str.starts_with("/snap") { // Для Ubuntu snap пакетов
            continue;
        }

        // Опционально: можно вообще показывать ТОЛЬКО /run/media, /media и /mnt
        // Раскомментируй строки ниже для строгой фильтрации:
        /*
        if !mount_str.starts_with("/run/media") 
            && !mount_str.starts_with("/media") 
            && !mount_str.starts_with("/mnt") {
            continue;
        }
        */

        let label = format!("{} ({:?}) - {} GB", 
            disk.name().to_string_lossy(), 
            mount, 
            disk.total_space() / 1024 / 1024 / 1024
        );
        
        choices.push(label);
        disks_map.push(mount.to_path_buf());
    }

    if choices.is_empty() {
        // Подсказка пользователю
        return Err(anyhow!("No removable drives found!\n1. Ensure USB is plugged in.\n2. Ensure it is MOUNTED (open it in file manager)."));
    }

    let msg = if let Some(n) = count_needed {
        format!("Select {} USB drives to store keys:", n)
    } else {
        "Select USB drive(s):".to_string()
    };

    let selection = MultiSelect::new(&msg, choices)
        .with_help_message("Space to select, Enter to confirm")
        .prompt()?;

    if let Some(n) = count_needed {
        if selection.len() != n as usize {
            return Err(anyhow!("You selected {} drives, but required {}.", selection.len(), n));
        }
    }

    let mut selected_paths = Vec::new();
    for item in selection {
        let idx = disks_map.iter().enumerate().find(|(_, path)| {
             // Ищем диск, у которого метка совпадает с выбранной
             // Регенерируем метку для сравнения (немного неоптимально, но надежно)
             let d = sys.disks().iter().find(|d| d.mount_point() == *path).unwrap();
             let l = format!("{} ({:?}) - {} GB", 
                d.name().to_string_lossy(), 
                d.mount_point(), 
                d.total_space() / 1024 / 1024 / 1024
            );
            l == item
        }).map(|(i, _)| i).unwrap();
        
        selected_paths.push(disks_map[idx].clone());
    }

    Ok(selected_paths)
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
                println!("{} Initialized USB at {:?}", "✔".green(), path);
            }
            Ok(())
        }
        
        Commands::Encrypt { input, output, threshold, total, keys } => {
            println!("{} Processing project: {:?}", "🔒".blue(), input);
            if !input.exists() {
            return Err(anyhow!("Input directory {:?} does not exist! Please create it or check the path.", input));
            }
            if !input.is_dir() {
            return Err(anyhow!("Input {:?} is not a directory! d-crypt works with folders.", input));
            }
            let n = total.unwrap_or(2);
            let m = threshold.unwrap_or(2);

            if m > n {
                return Err(anyhow!("Threshold ({}) cannot be larger than Total keys ({})", m, n));
            }

            // Выбор флешек
            let selected_keys = if let Some(k) = keys {
                k.clone()
            } else {
                println!("{} Scheme: Need {} keys to decrypt, Creating {} keys total.", "🔑".yellow(), m, n);
                // Если флешки не переданы аргументами, запускаем интерактивный выбор
                select_usb_drives(Some(n))?
            };

            println!("   Archiving files...");
            let compressed_data = Archiver::compress_directory(&input)?;
            
            println!("   Generating cryptographic keys (AES-256)...");
            let project_key = CryptoEngine::generate_key();
            let project_uuid = Uuid::new_v4();
            
            let (encrypted_data, nonce) = CryptoEngine::encrypt(&compressed_data, &project_key)?;

            let project_name = input.file_name().unwrap_or_default().to_string_lossy().to_string();
            let container = EncryptedProject::new(project_uuid, nonce, encrypted_data, project_name, m);
            
            let output_path = output.clone().unwrap_or_else(|| {
                let mut p = input.clone();
                p.set_extension("dcr");
                p
            });
            
            let encoded_container = bincode::serialize(&container)?;
            fs::write(&output_path, encoded_container)?;
            
            println!("   Splitting key into {} parts...", n);
            let shards = ShamirEngine::split_secret(&project_key, m, n)?;

            for (i, usb_path) in selected_keys.iter().enumerate() {
                let key_dir = usb_path.join(".d-crypt").join("keys");
                if !key_dir.exists() {
                    let _ = fs::create_dir_all(&key_dir);
                }
                
                let shard_path = key_dir.join(format!("{}.shard", project_uuid));
                fs::write(&shard_path, &shards[i])?;
                println!("   {} Key part #{} written to {:?}", "💾".green(), i + 1, shard_path);
            }

            let hex_key = hex::encode(project_key);
            println!("\n{} Encrypted successfully to {:?}", "✔".green(), output_path);
            println!("{} Project UUID: {}", "🆔".yellow(), project_uuid);
            println!("{} MASTER RECOVERY CODE: {}", "🚨".red(), hex_key);
            
            Ok(())
        }

        Commands::Decrypt { input, output, keys } => {
            println!("{} Reading container {:?}", "🔓".blue(), input);

            let file_data = fs::read(input).context("Failed to read .dcr file")?;
            let container: EncryptedProject = bincode::deserialize(&file_data)
                .context("Invalid d-crypt container format")?;

            println!("   Target Project: {}", container.original_name);
            println!("   Keys required: {}", container.threshold);

            let mut found_shards = Vec::new();
            let mut found_paths = Vec::new();

            if let Some(manual_keys) = keys {
                for usb_path in manual_keys {
                    let shard_path = usb_path.join(".d-crypt").join("keys").join(format!("{}.shard", container.project_uuid));
                    if shard_path.exists() {
                        found_paths.push(usb_path.clone());
                        found_shards.push(fs::read(shard_path)?);
                    }
                }
            } else {
                println!("{} Scanning system for keys...", "🔍".cyan());
                let mut sys = System::new_all();
                sys.refresh_disks();

                for disk in sys.disks() {
                    let mount_point = disk.mount_point();
                    let shard_path = mount_point
                        .join(".d-crypt")
                        .join("keys")
                        .join(format!("{}.shard", container.project_uuid));

                    if shard_path.exists() {
                        found_paths.push(mount_point.to_path_buf());
                        found_shards.push(fs::read(shard_path)?);
                    }
                }
            }

            if found_shards.is_empty() {
                return Err(anyhow!("No keys found! Please insert USB drives."));
            }

            println!("\nFound keys on:");
            for p in &found_paths {
                println!(" - {:?}", p);
            }
            
            if found_shards.len() < container.threshold as usize {
                println!("{}", format!("WARNING: Found {} keys, but need {}.", found_shards.len(), container.threshold).red());
                return Err(anyhow!("Not enough keys."));
            }

            let confirm = Confirm::new("Proceed with decryption?")
                .with_default(true)
                .prompt()?;

            if !confirm {
                println!("Operation cancelled.");
                return Ok(());
            }

            println!("   Recovering Master Key...");
            
            // Берем ровно столько шардов, сколько нужно (threshold)
            let shards_to_use = &found_shards[0..container.threshold as usize];

            let master_key_vec = ShamirEngine::recover_secret(shards_to_use, container.threshold)
                .context("Failed to recover key. Ensure drives are correct.")?;
            
            let mut master_key = [0u8; 32];
            if master_key_vec.len() != 32 { return Err(anyhow!("Invalid key length")); }
            master_key.copy_from_slice(&master_key_vec);

            println!("   Decrypting & Unpacking...");
            let decrypted_compressed = CryptoEngine::decrypt(&container.data, &master_key, &container.nonce)?;

            let out_dir = output.clone().unwrap_or_else(|| PathBuf::from(&container.original_name));
            if out_dir.exists() {
                println!("   {} Directory exists, overwriting contents...", "⚠️".yellow());
            }
            
            Archiver::decompress_to(&decrypted_compressed, &out_dir)?;

            println!("\n{} Project restored to: {:?}", "✔".green(), out_dir);
            Ok(())
        }
    }
}