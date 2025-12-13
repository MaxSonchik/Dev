use actix_web::{get, web, App, HttpResponse, HttpServer, Responder};
use std::process::Command;
use std::collections::HashMap;

#[get("/ping")]
async fn ping(info: web::Query<HashMap<String, String>>) -> impl Responder {
    // ИСПРАВЛЕНИЕ: Клонируем строку, чтобы владеть ею
    let ip = info.get("ip").cloned().unwrap_or_else(|| "127.0.0.1".to_string());
    
    // ОПАСНО: RCE
    let output = Command::new("sh")
        .arg("-c")
        .arg(format!("ping -c 1 {}", ip)) 
        .output();

    match output {
        Ok(o) => HttpResponse::Ok().body(format!("Result: {}", String::from_utf8_lossy(&o.stdout))),
        Err(_) => HttpResponse::InternalServerError().body("Ping failed"),
    }
}

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    println!("💀 VULNERABLE SERVICE STARTED ON PORT 8080");
    HttpServer::new(|| {
        App::new().service(ping)
    })
    .bind(("0.0.0.0", 8080))?
    .run()
    .await
}