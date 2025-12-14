use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::net::{IpAddr, Ipv4Addr};
use anyhow::{Result, anyhow};
use serde::{Serialize, Deserialize};
use sysinfo::{Pid, Process, System, Networks}; 

use netlink_sys::{Socket, protocols::NETLINK_SOCK_DIAG};
use netlink_packet_core::{
    NetlinkMessage, NetlinkHeader, NetlinkPayload,
    NLM_F_REQUEST, NLM_F_DUMP
};
use netlink_packet_utils::traits::Emitable; 
use netlink_packet_sock_diag::{
    SockDiagMessage, 
    inet::{InetRequest, SocketId, StateFlags, ExtensionFlags},
    constants::{IPPROTO_TCP, IPPROTO_UDP, AF_INET},
};

// DTO для сохранения
#[derive(Serialize, Deserialize)]
pub struct SocketMap {
    pub map: HashMap<(u8, u16), ProcessInfo>,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ProcessInfo {
    pub pid: u32,
    pub name: String,
    pub user: String,
    pub cpu_usage: f32,       
    pub mem_usage: u64,       
    pub disk_read: u64,       
    pub disk_written: u64,    
    pub status: String,       
    pub cmd: String,          
    pub socket_count: usize,  
}

pub type SharedSocketMap = Arc<RwLock<ProcessMonitor>>;

pub struct ProcessMonitor {
    pub net_map: HashMap<(u8, u16), ProcessInfo>,
    pub all_processes: HashMap<u32, ProcessInfo>,
    sys: System,
}

impl ProcessMonitor {
    pub fn new() -> Self {
        ProcessMonitor { 
            net_map: HashMap::new(),
            all_processes: HashMap::new(),
            sys: System::new_all(),
        }
    }

    // === НОВЫЙ МЕТОД: Загрузка из дампа ===
    pub fn from_dump(dump: SocketMap) -> Self {
        let mut pm = ProcessMonitor::new();
        pm.net_map = dump.map;
        
        // Восстанавливаем список процессов для вкладки UI
        // Так как в net_map ключи - порты, один процесс может встречаться много раз.
        // Нам нужно собрать уникальные.
        for info in pm.net_map.values() {
            pm.all_processes.insert(info.pid, info.clone());
        }
        pm
    }

    pub fn get_process_by_port(&self, proto: u8, port: u16) -> Option<ProcessInfo> {
        self.net_map.get(&(proto, port)).cloned()
    }

    pub fn update(&mut self) {
        self.sys.refresh_all(); 

        let mut inode_to_port_tcp = HashMap::new();
        let mut inode_to_port_udp = HashMap::new();

        if let Ok(sockets) = fetch_sockets(IPPROTO_TCP) {
            for s in sockets { inode_to_port_tcp.insert(s.inode, s.port); }
        }
        if let Ok(sockets) = fetch_sockets(IPPROTO_UDP) {
            for s in sockets { inode_to_port_udp.insert(s.inode, s.port); }
        }

        let mut new_net_map = HashMap::new();
        let mut new_all_processes = HashMap::new();

        for (pid, proc) in self.sys.processes() {
            let pid_u32 = pid.as_u32();
            let (open_sockets, ports) = self.scan_process_sockets(pid_u32, &inode_to_port_tcp, &inode_to_port_udp);

            let user = if let Some(uid) = proc.user_id() {
                uid.to_string() 
            } else {
                "root".to_string() 
            };
            
            let status_str = format!("{:?}", proc.status());

            let info = ProcessInfo {
                pid: pid_u32,
                name: proc.name().to_string(), 
                user,
                cpu_usage: proc.cpu_usage(),
                mem_usage: proc.memory(),
                disk_read: proc.disk_usage().read_bytes,
                disk_written: proc.disk_usage().written_bytes,
                status: status_str,
                cmd: format!("{:?}", proc.cmd()),
                socket_count: open_sockets,
            };

            new_all_processes.insert(pid_u32, info.clone());

            for (proto, port) in ports {
                new_net_map.insert((proto, port), info.clone());
            }
        }

        self.all_processes = new_all_processes;
        self.net_map = new_net_map;
    }

    fn scan_process_sockets(
        &self, 
        pid: u32, 
        tcp_inodes: &HashMap<u32, u16>, 
        udp_inodes: &HashMap<u32, u16>
    ) -> (usize, Vec<(u8, u16)>) {
        let fd_path = format!("/proc/{}/fd", pid);
        let mut socket_count = 0;
        let mut ports = Vec::new();

        if let Ok(entries) = std::fs::read_dir(fd_path) {
            for fd_entry in entries.flatten() {
                if let Ok(target) = std::fs::read_link(fd_entry.path()) {
                    let target_str = target.to_string_lossy();
                    if target_str.starts_with("socket:[") {
                        let inode_str = target_str
                            .trim_start_matches("socket:[")
                            .trim_end_matches(']');
                        
                        if let Ok(inode) = inode_str.parse::<u32>() {
                            socket_count += 1;
                            if let Some(port) = tcp_inodes.get(&inode) {
                                ports.push((6, *port));
                            } else if let Some(port) = udp_inodes.get(&inode) {
                                ports.push((17, *port));
                            }
                        }
                    }
                }
            }
        }
        (socket_count, ports)
    }

    pub fn kill_process(&self, pid: u32) -> bool {
        if let Some(proc) = self.sys.process(Pid::from_u32(pid)) {
            return proc.kill();
        }
        false
    }
    
    pub fn suspend_process(&self, pid: u32) -> bool {
        if let Some(proc) = self.sys.process(Pid::from_u32(pid)) {
            return proc.kill_with(sysinfo::Signal::Stop).unwrap_or(false);
        }
        false
    }

    pub fn resume_process(&self, pid: u32) -> bool {
         if let Some(proc) = self.sys.process(Pid::from_u32(pid)) {
            return proc.kill_with(sysinfo::Signal::Continue).unwrap_or(false);
        }
        false
    }
}

struct RawSocketInfo {
    inode: u32,
    port: u16,
}

fn fetch_sockets(proto: u8) -> Result<Vec<RawSocketInfo>> {
    let mut socket = Socket::new(NETLINK_SOCK_DIAG)?; 
    let mut header = NetlinkHeader::default();
    header.flags = NLM_F_REQUEST | NLM_F_DUMP;

    let payload = SockDiagMessage::InetRequest(InetRequest {
        family: AF_INET,
        protocol: proto,
        extensions: ExtensionFlags::empty(),
        states: StateFlags::all(),
        socket_id: SocketId {
            source_port: 0, destination_port: 0,
            source_address: IpAddr::V4(Ipv4Addr::new(0, 0, 0, 0)),
            destination_address: IpAddr::V4(Ipv4Addr::new(0, 0, 0, 0)),
            interface_id: 0, cookie: [0; 8],
        },
    });

    let mut packet = NetlinkMessage::new(header, NetlinkPayload::InnerMessage(payload));
    packet.finalize(); 
    let mut buf = vec![0u8; packet.buffer_len()];
    packet.emit(&mut buf); 
    socket.send(&buf, 0)?;

    let mut results = Vec::new();
    let mut receive_buffer = vec![0u8; 8192];

    loop {
        let size = socket.recv(&mut receive_buffer, 0)?;
        let mut offset = 0;
        while offset < size {
            let bytes = &receive_buffer[offset..];
            let rx_packet = <NetlinkMessage<SockDiagMessage>>::deserialize(bytes)
                .map_err(|e| anyhow!("Parse error: {}", e))?;

            match rx_packet.payload {
                NetlinkPayload::InnerMessage(SockDiagMessage::InetResponse(response)) => {
                    results.push(RawSocketInfo {
                        inode: response.header.inode,
                        port: response.header.socket_id.source_port,
                    });
                }
                NetlinkPayload::Done(_) => return Ok(results),
                _ => {},
            }
            offset += rx_packet.header.length as usize;
            if offset >= size || rx_packet.header.length == 0 { break; }
        }
    }
}