use std::fs::File;
use std::io::{Read, Write};
use std::path::Path;
use anyhow::{Result};
use crate::analysis::process::SocketMap;
use crate::analysis::process::ProcessInfo;
use std::collections::HashMap;
use tempfile::NamedTempFile; // Импорт

#[derive(serde::Serialize, serde::Deserialize)]
struct ProcessMapDump {
    map: HashMap<String, ProcessInfo>,
}

pub fn save_dshark_archive(
    out_path: &str, 
    pcap_path: &Path, 
    socket_map: &SocketMap
) -> Result<()> {
    let file = File::create(out_path)?;
    let mut zip = zip::ZipWriter::new(file);
    let options = zip::write::FileOptions::default()
        .compression_method(zip::CompressionMethod::Stored);

    zip.start_file("dump.pcap", options)?;
    let mut pcap_file = File::open(pcap_path)?;
    let mut buffer = Vec::new();
    pcap_file.read_to_end(&mut buffer)?;
    zip.write_all(&buffer)?;

    zip.start_file("process_map.json", options)?;
    
    let mut dump = ProcessMapDump { map: HashMap::new() };
    for ((proto, port), info) in &socket_map.map {
        let key = format!("{}:{}", proto, port);
        dump.map.insert(key, info.clone());
    }
    
    let json_data = serde_json::to_string_pretty(&dump)?;
    zip.write_all(json_data.as_bytes())?;

    zip.finish()?;
    Ok(())
}

// === НОВАЯ ФУНКЦИЯ: Загрузка архива ===
pub fn load_dshark_archive(path: &str) -> Result<(NamedTempFile, SocketMap)> {
    let file = File::open(path)?;
    let mut archive = zip::ZipArchive::new(file)?;

    // 1. Читаем JSON
    let mut json_file = archive.by_name("process_map.json")?;
    let dump: ProcessMapDump = serde_json::from_reader(&mut json_file)?;
    drop(json_file); // Освобождаем архив для чтения следующего файла

    // Преобразуем строковые ключи обратно в (u8, u16)
    let mut map = SocketMap { map: HashMap::new() };
    for (k, v) in dump.map {
        let parts: Vec<&str> = k.split(':').collect();
        if parts.len() == 2 {
            if let (Ok(proto), Ok(port)) = (parts[0].parse::<u8>(), parts[1].parse::<u16>()) {
                map.map.insert((proto, port), v);
            }
        }
    }

    // 2. Распаковываем PCAP во временный файл
    let mut pcap_in_zip = archive.by_name("dump.pcap")?;
    let mut temp_pcap = NamedTempFile::new()?;
    std::io::copy(&mut pcap_in_zip, &mut temp_pcap)?;

    Ok((temp_pcap, map))
}