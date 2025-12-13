use serde::{Deserialize, Serialize};
use log::{info, warn};
use std::net::UdpSocket;
use std::sync::{Arc, Mutex};

const BROADCAST_ADDR: &str = "255.255.255.255:9000";
const LISTEN_ADDR: &str = "0.0.0.0:9000";

#[derive(Serialize, Deserialize, Debug)]
struct Alert {
    sender_ip: String,
    threat_type: String, // "RANSOMWARE"
    timestamp: u64,
}

// Отправка сигнала бедствия всем соседям
pub fn broadcast_alert() {
    let socket = UdpSocket::bind("0.0.0.0:0").unwrap();
    socket.set_broadcast(true).unwrap();
    
    let alert = Alert {
        sender_ip: "ME".to_string(), // В реале тут IP
        threat_type: "RANSOMWARE".to_string(),
        timestamp: 0,
    };
    
    let msg = serde_json::to_string(&alert).unwrap();
    socket.send_to(msg.as_bytes(), BROADCAST_ADDR).unwrap();
    info!("📡 DISTRESS SIGNAL BROADCASTED TO THE GRID");
}

// Слушаем сеть на наличие сигналов бедствия
pub fn listen_for_alerts(trigger_lockdown: impl Fn() + Send + 'static) {
    std::thread::spawn(move || {
        let socket = UdpSocket::bind(LISTEN_ADDR).unwrap();
        let mut buf = [0u8; 1024];
        
        loop {
            if let Ok((amt, _src)) = socket.recv_from(&mut buf) {
                let msg = &buf[..amt];
                if let Ok(alert) = serde_json::from_slice::<Alert>(msg) {
                    warn!("📡 RECEIVED ALERT from neighbor! Threat: {}", alert.threat_type);
                    // Сосед атакован! Немедленно включаем защиту у себя.
                    trigger_lockdown();
                }
            }
        }
    });
}