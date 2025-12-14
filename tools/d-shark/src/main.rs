use clap::Parser;
use crossterm::{
    event::{self, DisableMouseCapture, EnableMouseCapture, Event, KeyCode, KeyModifiers, MouseEventKind},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};
use ratatui::{
    backend::{CrosstermBackend},
    layout::{Constraint, Direction, Layout, Rect},
    style::{Color, Modifier, Style},
    text::{Span},
    widgets::{Block, Borders, List, ListItem, Paragraph, Wrap, Tabs, Table, Row, Cell},
    Terminal, Frame,
};
use std::{io, time::Duration, thread, sync::{Arc, RwLock}};
use std::path::PathBuf;
use tokio::sync::mpsc;
use bytesize::ByteSize; 

mod capture;
mod ui;
mod firewall;
mod analysis;
mod storage;

use capture::engine::{CaptureEngine, CaptureMode};
use analysis::process::ProcessMonitor;
use capture::model::PacketSummary;
use ui::app::{App, ActiveTab};

#[derive(Parser)]
struct Cli {
    #[arg(short, long, default_value = "eth0")]
    interface: String,
    
    // Флаг для открытия файла (pcap или dshark)
    #[arg(short, long)]
    file: Option<String>,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let cli = Cli::parse();
    
    let process_monitor;
    let mode;
    
    // Переменная для хранения временного файла (чтобы он не удалился раньше времени)
    let mut _temp_file_guard: Option<tempfile::NamedTempFile> = None;

    if let Some(filename) = cli.file {
        if filename.ends_with(".dshark") {
            // Режим D-SHARK (Archive)
            let (temp_file, loaded_map) = storage::load_dshark_archive(&filename)?;
            // Инициализируем монитор загруженными данными
            process_monitor = Arc::new(RwLock::new(ProcessMonitor::from_dump(loaded_map)));
            
            // Используем путь к распакованному временному pcap
            mode = CaptureMode::File(temp_file.path().to_path_buf());
            _temp_file_guard = Some(temp_file);
            
            // Фоновый поток НЕ запускаем, так как это снимок
        } else {
            // Режим PCAP (Standard)
            process_monitor = Arc::new(RwLock::new(ProcessMonitor::new()));
            mode = CaptureMode::File(PathBuf::from(filename));
            // Фоновый поток НЕ запускаем (нет смысла, процессы не те)
        }
    } else {
        // Режим LIVE
        process_monitor = Arc::new(RwLock::new(ProcessMonitor::new()));
        let monitor_clone = process_monitor.clone();
        
        // Запускаем мониторинг процессов только в LIVE режиме
        thread::spawn(move || {
            loop {
                let mut mon = monitor_clone.write().unwrap();
                mon.update();
                drop(mon);
                thread::sleep(Duration::from_millis(1000));
            }
        });
        
        mode = CaptureMode::Live(cli.interface.clone());
    }

    let engine = CaptureEngine::new(mode, process_monitor.clone());
    let (tx, mut rx) = mpsc::channel::<PacketSummary>(1000);
    engine.start(tx);

    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen, EnableMouseCapture)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    let mut app = App::new(engine.temp_pcap_path.clone(), process_monitor.clone());
    
    // Если открыли файл, сразу обновляем список процессов в UI, чтобы он не был пустым
    if _temp_file_guard.is_some() {
        app.active_tab = ActiveTab::Processes; // Хак чтобы сработал апдейт
        app.update_process_list();
        app.active_tab = ActiveTab::Packets;
    }

    let res = run_app(&mut terminal, &mut app, &mut rx).await;

    engine.stop();
    disable_raw_mode()?;
    execute!(terminal.backend_mut(), LeaveAlternateScreen, DisableMouseCapture)?;
    terminal.show_cursor()?;

    if let Err(err) = res { println!("{:?}", err) }
    Ok(())
}

async fn run_app(
    terminal: &mut Terminal<CrosstermBackend<io::Stdout>>,
    app: &mut App,
    rx: &mut mpsc::Receiver<PacketSummary>,
) -> io::Result<()> {
    loop {
        // В режиме Live процессы обновляются фоновым потоком.
        // В режиме File они статичны, но update_process_list нужен для сортировки/фильтрации UI.
        if app.active_tab == ActiveTab::Processes {
            app.update_process_list();
        }

        terminal.draw(|f| ui_draw(f, app))?;

        for _ in 0..100 {
            if let Ok(p) = rx.try_recv() { app.on_packet(p); } else { break; }
        }

        if event::poll(Duration::from_millis(16))? {
            match event::read()? {
                Event::Key(key) => {
                    if app.active_tab == ActiveTab::Processes && app.show_process_search {
                        app.handle_process_search_input(key.code);
                    } else { 
                        match key.code {
                            KeyCode::Char('q') => return Ok(()),
                            KeyCode::Tab => app.switch_tab(),
                            
                            KeyCode::Char('s') => app.save_capture(),
                            
                            KeyCode::Char('k') => { 
                                if app.active_tab == ActiveTab::Processes {
                                    app.kill_selected_process();
                                }
                            },
                            KeyCode::Char('p') => { 
                                if app.active_tab == ActiveTab::Processes {
                                    app.suspend_selected_process();
                                }
                            },
                            KeyCode::Char('r') => { 
                                if app.active_tab == ActiveTab::Processes {
                                    app.resume_selected_process();
                                }
                            },
                            KeyCode::Char('o') => { 
                                if app.active_tab == ActiveTab::Processes {
                                    app.cycle_sort();
                                }
                            },
                            KeyCode::Char('/') => { 
                                if app.active_tab == ActiveTab::Processes {
                                    app.toggle_process_search();
                                }
                            },

                            KeyCode::Char('b') => app.block_ip(),
                            KeyCode::Char('d') => app.toggle_details(),
                            KeyCode::Enter => {
                                if app.active_tab == ActiveTab::Packets && app.show_details_pane {
                                    app.toggle_tree_item();
                                }
                            },
                            
                            KeyCode::Down => {
                                if key.modifiers.contains(KeyModifiers::SHIFT) {
                                    app.next_tree_item();
                                } else {
                                    app.next();
                                }
                            },
                            KeyCode::Up => {
                                if key.modifiers.contains(KeyModifiers::SHIFT) {
                                    app.prev_tree_item();
                                } else {
                                    app.previous();
                                }
                            },
                            _ => {}
                        }
                    }
                },
                Event::Mouse(mouse) => {
                    match mouse.kind {
                        MouseEventKind::ScrollDown => app.next(),
                        MouseEventKind::ScrollUp => app.previous(),
                        _ => {}
                    }
                }
                _ => {}
            }
        }
    }
}

fn ui_draw(f: &mut Frame, app: &mut App) {
    let main_chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([Constraint::Length(3), Constraint::Min(0), Constraint::Length(1)].as_ref())
        .split(f.size());

    let tab_titles = vec![" Packets (Wireshark) ", " Processes (Stratoshark) "];
    let tabs = Tabs::new(tab_titles)
        .block(Block::default().borders(Borders::ALL).title(" d-shark v0.4 "))
        .highlight_style(Style::default().fg(Color::Yellow))
        .select(app.active_tab as usize);
    f.render_widget(tabs, main_chunks[0]);

    match app.active_tab {
        ActiveTab::Packets => draw_packets_tab(f, app, main_chunks[1]),
        ActiveTab::Processes => draw_processes_tab(f, app, main_chunks[1]),
    }

    let help_msg = match app.active_tab {
        ActiveTab::Packets => "KEYS: [Tab] Switch | [Arrows] List | [Shift+Arrows] Tree | [Enter] Expand | [s] Save | [b] Block IP",
        ActiveTab::Processes => {
            if app.show_process_search {
                "Type to search (ESC to exit search)"
            } else {
                "KEYS: [Tab] Switch | [o] Sort (Cpu/Mem/Pid) | [k] Kill | [p] Pause | [r] Resume | [/] Search"
            }
        },
    };

    let status = Paragraph::new(format!("{} | Status: {}", help_msg, app.status_msg))
        .style(Style::default().bg(Color::Blue).fg(Color::White));
    f.render_widget(status, main_chunks[2]);
}

fn draw_packets_tab(f: &mut Frame, app: &mut App, area: Rect) {
    let constraints = if app.show_details_pane {
        vec![Constraint::Percentage(50), Constraint::Percentage(50)]
    } else {
        vec![Constraint::Percentage(100)]
    };

    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints(constraints)
        .split(area);

    let items: Vec<ListItem> = app.packets.iter().map(|p| {
        let color = match p.protocol.as_str() {
            "TCP" => Color::LightGreen,
            "UDP" => Color::LightBlue,
            "ARP" => Color::Yellow,
            "ICMP" | "ICMPv6" => Color::LightMagenta,
            _ => Color::White,
        };
        
        let proc_info = if let Some(name) = &p.process_name {
            format!("[{}]", name)
        } else { "".to_string() };

        let content = format!(
            "{:<10} {:<16} -> {:<16} | {:<5} | {:<25} {:<10}", 
            p.timestamp, p.src, p.dst, p.protocol, p.info, proc_info
        );
        ListItem::new(Span::styled(content, Style::default().fg(color)))
    }).collect();

    let list = List::new(items)
        .block(Block::default().borders(Borders::ALL).title(" Live Traffic "))
        .highlight_style(Style::default().bg(Color::DarkGray));
    f.render_stateful_widget(list, chunks[0], &mut app.table_state);

    if app.show_details_pane && chunks.len() > 1 {
        let bottom_chunks = Layout::default()
            .direction(Direction::Horizontal)
            .constraints([Constraint::Percentage(60), Constraint::Percentage(40)])
            .split(chunks[1]);

        let tree_items: Vec<ListItem> = app.flattened_tree.iter().map(|(depth, text, is_header)| {
            let indent = "  ".repeat(*depth);
            let content = format!("{}{}", indent, text);
            let style = if *is_header {
                Style::default().fg(Color::Cyan).add_modifier(Modifier::BOLD)
            } else {
                Style::default().fg(Color::Gray)
            };
            ListItem::new(Span::styled(content, style))
        }).collect();
        
        let tree_list = List::new(tree_items)
            .block(Block::default().borders(Borders::ALL).title(" OSI Layers (Shift+Arr, Enter) "))
            .highlight_style(Style::default().bg(Color::Blue));
            
        f.render_stateful_widget(tree_list, bottom_chunks[0], &mut app.tree_list_state);

        let hex_text = if let Some(d) = &app.selected_details {
            String::from_utf8_lossy(&d.raw_data).to_string()
        } else { "".to_string() };

        let hex = Paragraph::new(hex_text)
            .block(Block::default().borders(Borders::ALL).title(" Hex Dump "))
            .wrap(Wrap { trim: true });
        f.render_widget(hex, bottom_chunks[1]);
    }
}

fn draw_processes_tab(f: &mut Frame, app: &mut App, area: Rect) {
    let (input_area, table_area) = if app.show_process_search {
        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([Constraint::Length(3), Constraint::Min(0)])
            .split(area);
        (Some(chunks[0]), chunks[1])
    } else {
        (None, area)
    };

    if let Some(input_area) = input_area {
        let input = Paragraph::new(app.process_search_query.as_str())
            .style(Style::default().fg(Color::Yellow))
            .block(Block::default().borders(Borders::ALL).title(" Search Process (ESC to cancel) "));
        f.render_widget(input, input_area);
    }

    let header = Row::new(vec![
        "PID", "Name", "User", "CPU %", "MEM", "State", "Sockets", "Cmd"
    ])
    .style(Style::default().fg(Color::Yellow).add_modifier(Modifier::BOLD))
    .bottom_margin(1);

    let rows: Vec<Row> = app.filtered_process_list.iter().map(|p| {
        let style = if p.cpu_usage > 50.0 || p.mem_usage > 1024 * 1024 * 500 {
            Style::default().fg(Color::Red)
        } else if p.cpu_usage > 10.0 {
            Style::default().fg(Color::LightRed)
        } else {
            Style::default().fg(Color::White)
        };

        Row::new(vec![
            Cell::from(p.pid.to_string()),
            Cell::from(p.name.clone()),
            Cell::from(p.user.clone()),
            Cell::from(format!("{:.1}%", p.cpu_usage)),
            Cell::from(ByteSize(p.mem_usage).to_string()),
            Cell::from(p.status.clone()),
            Cell::from(p.socket_count.to_string()),
            Cell::from(p.cmd.clone()),
        ]).style(style)
    }).collect();

    let table = Table::new(rows, [
        Constraint::Length(6),  
        Constraint::Length(15), 
        Constraint::Length(10), 
        Constraint::Length(8),  
        Constraint::Length(10), 
        Constraint::Length(8),  
        Constraint::Min(20),    
    ])
    .header(header)
    .block(Block::default().borders(Borders::ALL).title(" System Processes "))
    .highlight_style(Style::default().bg(Color::DarkGray));

    f.render_stateful_widget(table, table_area, &mut app.process_table_state);
}