use crate::capture::model::{PacketSummary, PacketDetails};
use crate::firewall::Firewall;
use crate::storage::save_dshark_archive;
use crate::analysis::process::{SharedSocketMap, ProcessInfo};
// ИСПРАВЛЕНО: Убраны виджеты отображения, оставлены только States
use ratatui::widgets::{ListState, ScrollbarState, TableState}; 
use std::process::{Command};
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use serde_json::Value;
use std::collections::{HashSet, HashMap};
use crossterm::event::{KeyCode};

#[derive(PartialEq, Clone, Copy)]
pub enum ActiveTab {
    Packets = 0,
    Processes = 1,
}

#[derive(Clone, Debug)]
pub struct ProtocolNode {
    pub label: String,
    pub children: Vec<ProtocolNode>,
    pub layer_type: String,
}

#[derive(PartialEq, Clone, Copy, Debug)]
pub enum ProcessSort {
    Pid,
    Cpu,
    Mem,
}

pub struct App {
    pub packets: Vec<PacketSummary>,
    pub selected_details: Option<PacketDetails>,
    pub socket_map: SharedSocketMap,
    pub process_list_cache: Vec<ProcessInfo>, 
    
    pub active_tab: ActiveTab,
    pub table_state: ListState,
    pub process_table_state: TableState, 
    pub scroll_state: ScrollbarState,
    pub show_details_pane: bool,
    pub process_sort: ProcessSort, 
    
    pub tree_root: Vec<ProtocolNode>,
    pub expanded_layers: HashSet<String>,
    pub tree_list_state: ListState,
    pub flattened_tree: Vec<(usize, String, bool)>, 
    
    pub auto_scroll: bool,
    pub temp_pcap_path: Arc<Mutex<Option<PathBuf>>>,
    pub status_msg: String,

    pub process_search_query: String,
    pub show_process_search: bool,
    pub filtered_process_list: Vec<ProcessInfo>, 
}

impl App {
    pub fn new(temp_path: Arc<Mutex<Option<PathBuf>>>, map: SharedSocketMap) -> App {
        let mut expanded = HashSet::new();
        expanded.insert("frame".into());
        expanded.insert("eth".into());
        expanded.insert("ip".into());
        expanded.insert("tcp".into());
        expanded.insert("udp".into());

        App {
            packets: Vec::new(),
            selected_details: None,
            socket_map: map,
            process_list_cache: Vec::new(),
            active_tab: ActiveTab::Packets,
            table_state: ListState::default(),
            process_table_state: TableState::default(),
            scroll_state: ScrollbarState::default(),
            show_details_pane: true,
            auto_scroll: true,
            temp_pcap_path: temp_path,
            status_msg: "Ready.".to_string(),
            process_sort: ProcessSort::Cpu,
            
            tree_root: Vec::new(),
            expanded_layers: expanded,
            tree_list_state: ListState::default(),
            flattened_tree: Vec::new(),

            process_search_query: String::new(),
            show_process_search: false,
            filtered_process_list: Vec::new(),
        }
    }

    pub fn on_packet(&mut self, p: PacketSummary) {
        self.packets.push(p);
        self.scroll_state = self.scroll_state.content_length(self.packets.len());
        if self.auto_scroll {
            self.table_state.select(Some(self.packets.len() - 1));
        }
    }

    pub fn update_process_list(&mut self) {
        if self.active_tab != ActiveTab::Processes { return; }
        
        let selected_pid = if let Some(idx) = self.process_table_state.selected() {
            self.filtered_process_list.get(idx).map(|p| p.pid)
        } else {
            None
        };

        let map = self.socket_map.read().unwrap();
        let mut list: Vec<ProcessInfo> = map.all_processes.values().cloned().collect();
        
        match self.process_sort {
            ProcessSort::Pid => list.sort_by_key(|p| p.pid),
            ProcessSort::Cpu => list.sort_by(|a, b| b.cpu_usage.partial_cmp(&a.cpu_usage).unwrap()),
            ProcessSort::Mem => list.sort_by_key(|p| std::cmp::Reverse(p.mem_usage)),
        }
        self.process_list_cache = list; 

        let query_lower = self.process_search_query.to_lowercase();
        self.filtered_process_list = self.process_list_cache.iter()
            .filter(|p| p.name.to_lowercase().contains(&query_lower) || p.cmd.to_lowercase().contains(&query_lower))
            .cloned()
            .collect();

        if let Some(pid) = selected_pid {
            if let Some(new_idx) = self.filtered_process_list.iter().position(|p| p.pid == pid) {
                self.process_table_state.select(Some(new_idx));
            } else {
                if self.filtered_process_list.is_empty() {
                    self.process_table_state.select(None);
                } else {
                    self.process_table_state.select(Some(0));
                }
            }
        } else if !self.filtered_process_list.is_empty() {
             // Если ничего не было выбрано, выбираем первый, если список не пуст
             if self.process_table_state.selected().is_none() {
                 self.process_table_state.select(Some(0));
             }
        } else {
            self.process_table_state.select(None);
        }
    }

    pub fn next(&mut self) {
        match self.active_tab {
            ActiveTab::Packets => {
                let i = match self.table_state.selected() {
                    Some(i) => if i >= self.packets.len().saturating_sub(1) { i } else { i + 1 },
                    None => 0,
                };
                self.table_state.select(Some(i));
                self.auto_scroll = false;
                self.load_details(i);
            }
            ActiveTab::Processes => {
                 let count = self.filtered_process_list.len(); 
                 let i = match self.process_table_state.selected() {
                    Some(i) => if i >= count.saturating_sub(1) { i } else { i + 1 },
                    None => 0,
                 };
                 self.process_table_state.select(Some(i));
            }
        }
    }

    pub fn previous(&mut self) {
        match self.active_tab {
            ActiveTab::Packets => {
                let i = match self.table_state.selected() {
                    Some(i) => if i == 0 { 0 } else { i - 1 },
                    None => 0,
                };
                self.table_state.select(Some(i));
                self.auto_scroll = false;
                self.load_details(i);
            }
             ActiveTab::Processes => {
                 let i = match self.process_table_state.selected() {
                    Some(i) => if i == 0 { 0 } else { i - 1 },
                    None => 0,
                 };
                 self.process_table_state.select(Some(i));
            }
        }
    }

    pub fn cycle_sort(&mut self) {
        self.process_sort = match self.process_sort {
            ProcessSort::Cpu => ProcessSort::Mem,
            ProcessSort::Mem => ProcessSort::Pid,
            ProcessSort::Pid => ProcessSort::Cpu,
        };
        self.status_msg = format!("Sorted by {:?}", self.process_sort);
        self.update_process_list(); 
    }

    pub fn kill_selected_process(&mut self) {
        if let Some(idx) = self.process_table_state.selected() {
            if let Some(proc) = self.filtered_process_list.get(idx) { 
                let pid = proc.pid;
                let map = self.socket_map.read().unwrap();
                if map.kill_process(pid) {
                    self.status_msg = format!("Killed PID {}", pid);
                } else {
                    self.status_msg = format!("Failed to kill PID {}", pid);
                }
            }
        }
    }

    pub fn suspend_selected_process(&mut self) {
        if let Some(idx) = self.process_table_state.selected() {
            if let Some(proc) = self.filtered_process_list.get(idx) { 
                let pid = proc.pid;
                let map = self.socket_map.read().unwrap();
                if map.suspend_process(pid) {
                    self.status_msg = format!("Suspended PID {}", pid);
                } else {
                    self.status_msg = format!("Failed to suspend PID {}", pid);
                }
            }
        }
    }

    pub fn resume_selected_process(&mut self) {
        if let Some(idx) = self.process_table_state.selected() {
            if let Some(proc) = self.filtered_process_list.get(idx) { 
                let pid = proc.pid;
                let map = self.socket_map.read().unwrap();
                if map.resume_process(pid) {
                    self.status_msg = format!("Resumed PID {}", pid);
                } else {
                    self.status_msg = format!("Failed to resume PID {}", pid);
                }
            }
        }
    }

    pub fn toggle_process_search(&mut self) {
        self.show_process_search = !self.show_process_search;
        if !self.show_process_search {
            self.process_search_query.clear(); 
            self.update_process_list(); 
        } else {
            self.status_msg = "Type to search processes (ESC to exit search)".to_string();
        }
    }

    pub fn handle_process_search_input(&mut self, key_code: KeyCode) {
        match key_code {
            KeyCode::Char(c) => {
                self.process_search_query.push(c);
                self.update_process_list(); 
            },
            KeyCode::Backspace => {
                self.process_search_query.pop();
                self.update_process_list();
            },
            KeyCode::Esc => {
                self.toggle_process_search();
            },
            _ => {}
        }
    }

    pub fn next_tree_item(&mut self) {
        if self.flattened_tree.is_empty() { return; }
        let i = match self.tree_list_state.selected() {
            Some(i) => if i >= self.flattened_tree.len().saturating_sub(1) { i } else { i + 1 },
            None => 0
        };
        self.tree_list_state.select(Some(i));
    }

    pub fn prev_tree_item(&mut self) {
        if self.flattened_tree.is_empty() { return; }
        let i = match self.tree_list_state.selected() {
            Some(i) => if i == 0 { 0 } else { i - 1 },
            None => 0
        };
        self.tree_list_state.select(Some(i));
    }

    pub fn toggle_tree_item(&mut self) {
        if let Some(idx) = self.tree_list_state.selected() {
            if let Some((_, text, is_header)) = self.flattened_tree.get(idx) {
                if *is_header {
                    let target_label = text.replace("[+]", "").replace("[-]", "").trim().to_string();
                    let layer = self.find_layer_type_by_label(&target_label, &self.tree_root);
                    
                    if let Some(l) = layer {
                        if self.expanded_layers.contains(&l) {
                            self.expanded_layers.remove(&l);
                        } else {
                            self.expanded_layers.insert(l);
                        }
                        self.rebuild_flattened_tree();
                    }
                }
            }
        }
    }

    fn find_layer_type_by_label(&self, label: &str, nodes: &[ProtocolNode]) -> Option<String> {
        for node in nodes {
            if node.label.contains(label) || label.contains(&node.label) {
                return Some(node.layer_type.clone());
            }
            if let Some(res) = self.find_layer_type_by_label(label, &node.children) {
                return Some(res);
            }
        }
        None
    }

    fn load_details(&mut self, index: usize) {
        if index >= self.packets.len() { return; }
        if !self.show_details_pane { return; }
        
        let summary = self.packets[index].clone();
        
        let pcap_path = {
            let guard = self.temp_pcap_path.lock().unwrap();
            guard.clone()
        };
        
        if let Some(path) = pcap_path {
            let filter = format!("frame.number == {}", summary.id);
            let output = Command::new("tshark")
                .args(&["-r", path.to_str().unwrap(), "-Y", &filter, "-T", "ek", "-n"])
                .output();

            if let Ok(out) = output {
                let json_str = String::from_utf8_lossy(&out.stdout);
                for line in json_str.lines() {
                    if let Ok(val) = serde_json::from_str::<Value>(line) {
                        if let Some(layers) = val.get("layers") {
                             let hex_out = Command::new("tshark")
                                .args(&["-r", path.to_str().unwrap(), "-Y", &filter, "-x"])
                                .output();
                             let raw = if let Ok(h) = hex_out { h.stdout } else { vec![] };

                             self.selected_details = Some(PacketDetails {
                                 summary,
                                 raw_data: raw,
                                 layers: val.clone(),
                             });
                             
                             self.tree_root = parse_osi_layers(layers);
                             self.rebuild_flattened_tree();
                             
                             return;
                        }
                    }
                }
            }
        }
    }

    fn rebuild_flattened_tree(&mut self) {
        self.flattened_tree.clear();
        build_flat_list(&self.tree_root, 0, &self.expanded_layers, &mut self.flattened_tree);
    }
    
    pub fn switch_tab(&mut self) {
        self.active_tab = match self.active_tab {
            ActiveTab::Packets => ActiveTab::Processes,
            ActiveTab::Processes => ActiveTab::Packets,
        };
        if self.active_tab == ActiveTab::Processes {
            self.update_process_list();
        }
    }
    
    pub fn toggle_details(&mut self) {
        self.show_details_pane = !self.show_details_pane;
        if self.show_details_pane {
             if let Some(idx) = self.table_state.selected() {
                 self.load_details(idx);
             }
        }
    }

    pub fn block_ip(&mut self) {
        if self.active_tab == ActiveTab::Packets {
            if let Some(d) = &self.selected_details {
                if let Err(e) = Firewall::block_ip(&d.summary.src) {
                    self.status_msg = format!("Firewall Error: {}", e);
                } else {
                    self.status_msg = format!("Blocked IP: {}", d.summary.src);
                }
            }
        }
    }
    
    pub fn save_capture(&mut self) {
        let path_guard = self.temp_pcap_path.lock().unwrap();
        if let Some(path) = path_guard.as_ref() {
             let timestamp = chrono::Local::now().format("%H-%M-%S");
             let filename = format!("capture_{}.dshark", timestamp);
             let map = self.socket_map.read().unwrap();
             
             let mut temp_map = crate::analysis::process::SocketMap { map: HashMap::new() };
             for (k, v) in &map.net_map {
                 temp_map.map.insert(*k, v.clone());
             }

             match save_dshark_archive(&filename, path, &temp_map) {
                 Ok(_) => self.status_msg = format!("Saved to {}", filename),
                 Err(e) => self.status_msg = format!("Save Failed: {}", e),
             }
        }
    }
}

// === Parsing Logic (нужно оставить в app.rs) ===

fn parse_osi_layers(layers: &Value) -> Vec<ProtocolNode> {
    let mut nodes = Vec::new();
    if let Some(obj) = layers.as_object() {
        let order = vec!["frame", "eth", "ip", "ipv6", "arp", "tcp", "udp", "icmp", "http", "tls", "dns"];
        for key in &order {
            if let Some(val) = obj.get(*key) {
                nodes.push(parse_single_layer(key, val));
            }
        }
        for (key, val) in obj {
            if !order.contains(&key.as_str()) && !key.ends_with("_raw") && key != "data" {
                nodes.push(parse_single_layer(key, val));
            }
        }
    }
    nodes
}

fn parse_single_layer(key: &str, val: &Value) -> ProtocolNode {
    let label = match key {
        "frame" => "Frame (Physical Layer)".to_string(),
        "eth" => "Ethernet II (Data Link)".to_string(),
        "ip" => "Internet Protocol v4 (Network)".to_string(),
        "ipv6" => "Internet Protocol v6 (Network)".to_string(),
        "arp" => "Address Resolution Protocol".to_string(),
        "tcp" => "Transmission Control Protocol".to_string(),
        "udp" => "User Datagram Protocol".to_string(),
        _ => key.to_uppercase(),
    };

    let children = json_to_nodes(val);
    
    ProtocolNode {
        label,
        children,
        layer_type: key.to_string(),
    }
}

fn json_to_nodes(val: &Value) -> Vec<ProtocolNode> {
    let mut children = Vec::new();
    match val {
        Value::Object(map) => {
            for (k, v) in map {
                if k.ends_with("_raw") { continue; }
                let clean_k = k.split('.').last().unwrap_or(k);
                match v {
                    Value::Object(_) => {
                         children.push(ProtocolNode {
                             label: clean_k.to_string(),
                             children: json_to_nodes(v),
                             layer_type: "".into(),
                         });
                    },
                    Value::String(s) => children.push(simple_node(format!("{}: {}", clean_k, s))),
                    Value::Number(n) => children.push(simple_node(format!("{}: {}", clean_k, n))),
                    Value::Bool(b) => children.push(simple_node(format!("{}: {}", clean_k, b))),
                    _ => {}
                }
            }
        },
        _ => {}
    }
    children
}

fn simple_node(text: String) -> ProtocolNode {
    ProtocolNode { label: text, children: vec![], layer_type: "".into() }
}

fn build_flat_list(
    nodes: &[ProtocolNode], 
    depth: usize, 
    expanded: &HashSet<String>, 
    out: &mut Vec<(usize, String, bool)>
) {
    for node in nodes {
        let is_header = !node.layer_type.is_empty();
        let is_expanded = expanded.contains(&node.layer_type);
        
        let prefix = if is_header {
            if is_expanded { "[-] " } else { "[+] " }
        } else {
            "" 
        };
        
        out.push((depth, format!("{}{}", prefix, node.label), is_header));

        if is_expanded || !is_header {
            build_flat_list(&node.children, depth + 1, expanded, out);
        }
    }
}