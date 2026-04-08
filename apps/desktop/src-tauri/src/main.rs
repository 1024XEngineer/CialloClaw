// 该入口负责启动桌面端 Tauri 宿主。
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

// main 处理当前模块的相关逻辑。
fn main() {
    tauri::Builder::default()
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
