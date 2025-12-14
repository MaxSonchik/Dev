use crate::capture::model::{PacketSummary};
use crate::analysis::process::SharedSocketMap;
use std::sync::{Arc, Mutex};
use std::thread;
use tokio::sync::mpsc::Sender;
use pcap::{Capture, Device};
use etherparse::{SlicedPacket, TransportSlice, InternetSlice, LinkSlice};
use tempfile::NamedTempFile;
use std::path::PathBuf;
use std::io::Write; 

#[derive(Clone)]
pub enum CaptureMode {
    Live(String),         
    File(PathBuf),          
}

pub struct CaptureEngine {
    mode: CaptureMode,
    stop_signal: Arc<Mutex<bool>>,
    pub socket_map: SharedSocketMap,
    pub temp_pcap_path: Arc<Mutex<Option<PathBuf>>>,
}

impl CaptureEngine {
    pub fn new(mode: CaptureMode, map: SharedSocketMap) -> Self {
        Self {
            mode,
            stop_signal: Arc::new(Mutex::new(false)),
            socket_map: map,
            temp_pcap_path: Arc::new(Mutex::new(None)),
        }
    }

    pub fn start(&self, tx: Sender<PacketSummary>) {
        let mode = self.mode.clone();
        let stop = self.stop_signal.clone();
        let sock_map = self.socket_map.clone();
        let temp_path_store = self.temp_pcap_path.clone();

        thread::spawn(move || {
            let temp_file = NamedTempFile::new().expect("Failed to create temp pcap");
            let temp_path = temp_file.path().to_path_buf();
            *temp_path_store.lock().unwrap() = Some(temp_path.clone());

            match mode {
                CaptureMode::Live(ref device_name) => {
                    let device = Device::list().unwrap().into_iter()
                        .find(|d| d.name == *device_name)
                        .unwrap_or_else(|| Device::lookup().unwrap().unwrap());
                    
                    let mut cap = Capture::from_device(device).unwrap()
                        .promisc(true)
                        .snaplen(65535)
                        .timeout(100)
                        .open().unwrap();

                    let mut savefile = cap.savefile(&temp_path).expect("Failed to open savefile");
                    let mut id_counter = 0;

                    loop {
                        if *stop.lock().unwrap() { break; }
                        match cap.next_packet() {
                            Ok(packet) => {
                                id_counter += 1;
                                savefile.write(&packet);
                                process_packet_logic(id_counter, &packet, &sock_map, &tx);
                            },
                            Err(pcap::Error::TimeoutExpired) => continue,
                            Err(pcap::Error::NoMorePackets) => break,
                            Err(_) => continue, 
                        }
                    }
                },
                CaptureMode::File(ref path) => {
                    let mut cap = Capture::from_file(path).unwrap();
                    let mut savefile = cap.savefile(&temp_path).expect("Failed to open savefile");
                    let mut id_counter = 0;

                    loop {
                        if *stop.lock().unwrap() { break; }
                        match cap.next_packet() {
                            Ok(packet) => {
                                id_counter += 1;
                                savefile.write(&packet);
                                process_packet_logic(id_counter, &packet, &sock_map, &tx);
                            },
                            Err(pcap::Error::NoMorePackets) => break,
                            Err(_) => continue,
                        }
                    }
                }
            };
        });
    }

    pub fn stop(&self) {
        *self.stop_signal.lock().unwrap() = true;
    }
}

fn process_packet_logic(
    id: usize, 
    packet: &pcap::Packet, 
    map: &SharedSocketMap, 
    tx: &Sender<PacketSummary>
) {
    if let Some(summary) = parse_packet_fast(id, packet, map) {
        let _ = tx.blocking_send(summary);
    }
}

fn parse_packet_fast(id: usize, packet: &pcap::Packet, map: &SharedSocketMap) -> Option<PacketSummary> {
    match SlicedPacket::from_ethernet(&packet.data) {
        Ok(value) => {
            let mut src = String::from("?");
            let mut dst = String::from("?");
            let mut proto_name = "RAW".to_string();
            let mut info_extra = String::new();
            let mut src_port = 0;
            let mut dst_port = 0;
            let mut proto_num = 0;

            if let Some(link) = value.link {
                match link {
                    LinkSlice::Ethernet2(eth) => {
                        if eth.ether_type() == 0x0806 {
                             proto_name = "ARP".into();
                             src = format!("{:02x?}", eth.source());
                             dst = format!("{:02x?}", eth.destination());
                             info_extra = "ARP Request/Reply".into();
                        }
                    }
                    _ => {}
                }
            }

            if let Some(ip) = value.ip {
                match ip {
                    InternetSlice::Ipv4(h, _) => {
                        src = h.source_addr().to_string();
                        dst = h.destination_addr().to_string();
                        proto_name = "IPv4".into();
                    },
                    InternetSlice::Ipv6(h, _) => {
                        src = h.source_addr().to_string();
                        dst = h.destination_addr().to_string();
                        proto_name = "IPv6".into();
                    }
                }
            }

            if let Some(transport) = value.transport {
                match transport {
                    TransportSlice::Tcp(h) => {
                        proto_name = "TCP".into();
                        proto_num = 6;
                        src_port = h.source_port();
                        dst_port = h.destination_port();
                    },
                    TransportSlice::Udp(h) => {
                        proto_name = "UDP".into();
                        proto_num = 17;
                        src_port = h.source_port();
                        dst_port = h.destination_port();
                    },
                    TransportSlice::Icmpv4(h) => {
                        proto_name = "ICMP".into();
                        info_extra = format!("Type: {} Code: {}", h.type_u8(), h.code_u8());
                    },
                    TransportSlice::Icmpv6(h) => {
                        proto_name = "ICMPv6".into();
                        info_extra = format!("Type: {} Code: {}", h.type_u8(), h.code_u8());
                    },
                    TransportSlice::Unknown(_) => {} 
                }
            }

            let info = if src_port != 0 {
                format!("{} -> {}", src_port, dst_port)
            } else {
                info_extra
            };

            let mut proc_name = None;
            let mut pid = None;
            let mut user = None;

            if proto_num != 0 {
                let map_read = map.read().unwrap();
                // Обновленный вызов метода
                if let Some(info) = map_read.get_process_by_port(proto_num, src_port) {
                    proc_name = Some(info.name);
                    pid = Some(info.pid);
                    user = Some(info.user);
                }
                else if let Some(info) = map_read.get_process_by_port(proto_num, dst_port) {
                    proc_name = Some(info.name);
                    pid = Some(info.pid);
                    user = Some(info.user);
                }
            }

            Some(PacketSummary {
                id,
                timestamp: chrono::Local::now().format("%H:%M:%S%.3f").to_string(),
                src,
                dst,
                protocol: proto_name,
                length: packet.header.len as usize,
                info,
                src_port,
                dst_port,
                process_name: proc_name,
                pid,
                user,
            })
        },
        Err(_) => None
    }
}