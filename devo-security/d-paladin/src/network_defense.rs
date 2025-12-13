use log::{info, error};
use std::process::Command;

pub fn isolate_host() -> anyhow::Result<()> {
    info!("🛡️  NETWORK DEFENSE: INITIATING HOST ISOLATION PROTOCOL");
    
    // 1. Блокируем исходящий трафик к приватным сетям (Lateral Movement Prevention)
    // 172.16.0.0/12 - стандартный диапазон Docker сетей
    let status = Command::new("iptables")
        .args(&["-I", "OUTPUT", "-d", "172.16.0.0/12", "-j", "DROP"])
        .status();

    match status {
        Ok(s) if s.success() => {
            info!("✅ FIREWALL: Blocked outgoing traffic to local subnets.");
            info!("✅ Host is now QUARANTINED.");
        },
        _ => error!("❌ FIREWALL: Failed to apply iptables rules. Run as root!"),
    }
    
    Ok(())
}