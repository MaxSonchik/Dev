use anyhow::Result;
use log::{debug, error, info, warn};
use notify::{Config, EventKind, RecommendedWatcher, RecursiveMode, Watcher};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

mod grid;
mod network_defense;

const ENTROPY_THRESHOLD: f32 = 7.0;
const ATTACK_THRESHOLD: u32 = 2;

// Список ловушек (Honeypots). Имена должны быть привлекательными для вируса (по алфавиту)
const HONEYPOTS: &[&str] = &[
    "00_ADMIN_PASSWORD.txt", 
    "AA_CONFIDENTIAL.doc",
    "ZZ_BACKUP.db"
];

struct SecurityState {
    suspicious_events: u32,
    last_snapshot: String,
    protected_path: String,
    triggered: bool,
}

#[tokio::main]
async fn main() -> Result<()> {
    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or("info")).init();

    // Защищаем не только /tmp, но и важные места (в рамках контейнера)
    // Для демо возьмем /tmp/devos-lab, но Honeypots раскидаем
    let protected_path = "/tmp/devos-lab";

    info!("🛡️  D-PALADIN v3.0 (HONEYPOT DEFENSE)");
    
    // 1. Расставляем ловушки
    deploy_honeypots(protected_path)?;

    // 2. Снапшот
    let initial_snap = create_snapshot(protected_path, "base_safe_state")?;

    let state = Arc::new(Mutex::new(SecurityState {
        suspicious_events: 0,
        last_snapshot: initial_snap,
        protected_path: protected_path.to_string(),
        triggered: false,
    }));

    // --- GRID ---
    let grid_state = state.clone();
    grid::listen_for_alerts(move || {
        let mut s = grid_state.lock().unwrap();
        if s.triggered { return; }
        warn!("📡 GRID ALERT! INITIATING LOCKDOWN.");
        s.triggered = true;
        let _ = network_defense::isolate_host();
    });

    // --- FILE WATCHER ---
    let (tx, mut rx) = tokio::sync::mpsc::channel(2000); // Большой буфер для массовой атаки
    
    let mut watcher = RecommendedWatcher::new(move |res| {
        if let Ok(event) = res {
            let _ = tx.blocking_send(event);
        }
    }, Config::default())?;

    if Path::new(protected_path).exists() {
        watcher.watch(Path::new(protected_path), RecursiveMode::Recursive)?;
        info!("👁️  Watching file system and traps...");
    }

    while let Some(event) = rx.recv().await {
        // Мгновенная реакция на модификацию
        match event.kind {
            EventKind::Modify(_) | EventKind::Remove(_) | EventKind::Create(_) => {
                for path in event.paths {
                    // 1. ПРОВЕРКА ЛОВУШКИ (Молниеносно)
                    if is_honeypot(&path) {
                        error!("🚨 HONEYPOT TRIGGERED: {:?}", path);
                        // Не ждем энтропии, не ждем счетчика. KILL ON SIGHT.
                        let mut s = state.lock().unwrap();
                        if !s.triggered {
                            s.triggered = true;
                            let snap = s.last_snapshot.clone();
                            let p_path = s.protected_path.clone();
                            drop(s);
                            
                            // Сначала убиваем, потом анализируем
                            terminate_threat(); 
                            grid::broadcast_alert();
                            trigger_defense(&p_path, &snap)?;
                        }
                    } else if path.is_file() {
                        // 2. Обычная проверка энтропии
                        check_file_entropy(&path, state.clone())?;
                    }
                }
            }
            _ => {}
        }
    }
    Ok(())
}

fn deploy_honeypots(base_dir: &str) -> Result<()> {
    if !Path::new(base_dir).exists() { fs::create_dir_all(base_dir)?; }
    
    for name in HONEYPOTS {
        let path = Path::new(base_dir).join(name);
        fs::write(&path, "HONEYPOT DATA DO NOT TOUCH")?;
        info!("🪤 Trap set: {:?}", path);
    }
    Ok(())
}

fn is_honeypot(path: &Path) -> bool {
    if let Some(name) = path.file_name().and_then(|n| n.to_str()) {
        for hp in HONEYPOTS {
            if name.contains(hp) { return true; }
        }
    }
    return false;
}

fn terminate_threat() {
    // SIGKILL (-9) немедленно
    let _ = Command::new("pkill").arg("-9").arg("-f").arg("d-ransom").output();
}

// ... check_file_entropy ... (тот же код, но можно убрать задержку sleep, мы полагаемся на ловушки)
fn check_file_entropy(path: &Path, state: Arc<Mutex<SecurityState>>) -> Result<()> {
    if path.to_string_lossy().contains(".snapshots") { return Ok(()); }
    // Проверяем только файлы, которые существуют (не удаленные)
    if !path.exists() { return Ok(()); }

    // Читаем без задержки
    let mut buffer = [0u8; 4096];
    if let Ok(mut file) = std::fs::File::open(path) {
        use std::io::Read;
        let n = file.read(&mut buffer)?;
        if n == 0 { return Ok(()); }
        
        let entropy = calculate_entropy(&buffer[0..n]);
        if entropy > ENTROPY_THRESHOLD {
             let mut s = state.lock().unwrap();
             if s.triggered { return Ok(()); }
             s.suspicious_events += 1;
             
             // Если массовая атака (многопоточная), реагируем на ПЕРВЫЙ файл
             if s.suspicious_events >= 1 { 
                 error!("🚨 HIGH ENTROPY DETECTED ({:.2}). IMMEDIATE ACTION.", entropy);
                 s.triggered = true;
                 let snap = s.last_snapshot.clone();
                 let p = s.protected_path.clone();
                 drop(s);
                 
                 terminate_threat();
                 grid::broadcast_alert();
                 trigger_defense(&p, &snap)?;
             }
        }
    }
    Ok(())
}

// ... calculate_entropy, trigger_defense, create_snapshot (как было) ...
fn calculate_entropy(data: &[u8]) -> f32 {
    let mut counts = [0usize; 256];
    for &b in data { counts[b as usize] += 1; }
    let len = data.len() as f32;
    let mut entropy = 0.0;
    for &count in &counts {
        if count == 0 { continue; }
        let p = count as f32 / len;
        entropy -= p * p.log2();
    }
    entropy
}

fn trigger_defense(mount_point: &str, snapshot: &str) -> Result<()> {
    let start = Instant::now();
    // Изоляция сети
    let _ = network_defense::isolate_host();
    
    // Откат
    info!("⏳ RESTORING DATA...");
    for entry in fs::read_dir(mount_point)? {
        let entry = entry?;
        let path = entry.path();
        if path.is_file() { let _ = fs::remove_file(path); }
    }
    let snap_path = format!("{}/.snapshots/{}", mount_point, snapshot);
    Command::new("cp").arg("-a").arg(format!("{}/.", snap_path)).arg(mount_point).output()?;
    
    // Восстанавливаем ловушки, если они были удалены
    deploy_honeypots(mount_point)?;
    
    info!("✅ RECOVERY COMPLETE in {:.2?}", start.elapsed());
    Ok(())
}

fn create_snapshot(mount_point: &str, name: &str) -> Result<String> {
    let snap_dir = format!("{}/.snapshots/{}", mount_point, name);
    if !Path::new(&snap_dir).exists() {
        fs::create_dir_all(&snap_dir)?;
        Command::new("rsync").arg("-a").arg("--exclude=.snapshots").arg(format!("{}/", mount_point)).arg(&snap_dir).output()?;
    }
    Ok(name.to_string())
}