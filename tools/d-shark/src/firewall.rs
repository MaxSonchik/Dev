use std::process::Command;
use anyhow::{Result, anyhow};
use log::info;

pub struct Firewall;

impl Firewall {
    /// Блокирует IP адрес через nftables
    pub fn block_ip(ip: &str) -> Result<()> {
        // 1. Проверяем, существует ли таблица (для простоты пытаемся создать)
        // В реальном Arch Linux nftables уже настроен, мы добавляем в цепочку input
        // Команда: nft add rule inet filter input ip saddr <IP> drop
        
        info!("🔥 FIREWALL: Blocking IP {}", ip);

        // Мы используем 'inet filter' как стандартную таблицу. 
        // Если её нет, команда упадет, но для MVP считаем, что окружение подготовлено.
        let status = Command::new("nft")
            .args(&["add", "rule", "inet", "filter", "input", "ip", "saddr", ip, "drop"])
            .status();

        match status {
            Ok(s) if s.success() => Ok(()),
            Ok(_) => Err(anyhow!("Failed to add nftables rule (check permissions/tables)")),
            Err(e) => Err(anyhow!("Failed to execute nft: {}", e)),
        }
    }

    /// Блокирует порт (TCP)
    pub fn block_port(port: &str) -> Result<()> {
        info!("🔥 FIREWALL: Blocking Port {}", port);
        
        let status = Command::new("nft")
            .args(&["add", "rule", "inet", "filter", "input", "tcp", "dport", port, "drop"])
            .status();

        match status {
            Ok(s) if s.success() => Ok(()),
            _ => Err(anyhow!("Failed to block port")),
        }
    }
}