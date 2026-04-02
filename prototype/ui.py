from __future__ import annotations

import tkinter as tk
from pathlib import Path
from tkinter import filedialog, messagebox

from .ball import FloatingBall
from .chat_popup import ChatPopup
from .controller import PrototypeController
from .dashboard import BG, ACCENT, DANGER, SUCCESS, WARN, DashboardView


class PrototypeApp:
    def __init__(self) -> None:
        self.project_root = Path(__file__).resolve().parent.parent
        self.root = tk.Tk()
        self.root.title("CialloClaw Prototype")
        self._window_size = (1320, 900)
        self._center_root_window()
        self.root.minsize(1120, 760)
        self.root.configure(bg=BG)
        self.root.option_add("*Font", ("Segoe UI", 10))
        self.root.protocol("WM_DELETE_WINDOW", self.hide_dashboard)

        self.controller = PrototypeController(self.project_root, self.root.after, self.root.after_cancel)
        self.dashboard = DashboardView(self.root, self)
        self.chat = ChatPopup(self)
        self.ball = FloatingBall(self)
        self.controller.bus.on("*", self._on_controller_event)

        self._clipboard_last = ""
        self._toast_windows: list[tk.Toplevel] = []

        self.refresh_all()

    def run(self) -> None:
        self.controller.start()
        self.refresh_all()
        self.root.after(0, self.show_dashboard)
        self.root.after(250, self.show_dashboard)
        self.root.after(450, self._poll_clipboard)
        self.root.after(1800, self._scan_loop)
        self.root.after(1000, self._clock_loop)
        self.root.mainloop()

    def refresh_all(self) -> None:
        self.dashboard.refresh()
        self.chat.refresh()
        self.ball.refresh()

    def _on_controller_event(self, event: str, payload: dict) -> None:
        self.refresh_all()
        if event in {"approval.requested", "video.link.detected"}:
            log = payload.get("log")
            message = getattr(log, "message", None) or "需要审批"
            self.show_toast(message, WARN)
        elif event.endswith(".completed"):
            task = payload.get("task")
            title = getattr(task, "title", None) or "任务"
            self.show_toast(f"已完成：{title}", SUCCESS)
        elif event.endswith(".failed"):
            self.show_toast("任务失败，请查看日志。", DANGER)

    def _center_root_window(self) -> None:
        width, height = self._window_size
        self.root.update_idletasks()
        screen_width = self.root.winfo_screenwidth()
        screen_height = self.root.winfo_screenheight()
        x = max((screen_width - width) // 2, 20)
        y = max((screen_height - height) // 2, 20)
        self.root.geometry(f"{width}x{height}+{x}+{y}")

    def _poll_clipboard(self) -> None:
        try:
            raw = self.root.clipboard_get()
        except tk.TclError:
            raw = ""
        text = raw.strip()
        if text and text != self._clipboard_last:
            self._clipboard_last = text
            analysis = self.controller.handle_clipboard_text(text)
            if analysis.kind == "video":
                self.show_toast("检测到视频链接，等待确认后生成总结。", WARN)
                self.show_chat()
            elif analysis.kind == "error":
                self.show_toast("检测到错误文本，已切换到排查模式。", DANGER)
                self.show_chat()
            elif analysis.kind == "text":
                self.show_toast("检测到可处理文本，快捷动作已更新。", ACCENT)
                self.show_chat()
            self.refresh_all()
        self.root.after(500, self._poll_clipboard)

    def _scan_loop(self) -> None:
        if self.controller.state.auto_scan:
            self.controller.scan_todos()
            self.refresh_all()
        self.root.after(max(1, self.controller.state.scan_interval_seconds) * 1000, self._scan_loop)

    def _clock_loop(self) -> None:
        self.refresh_all()
        self.root.after(1000, self._clock_loop)

    def refresh_tasks_now(self) -> None:
        self.controller.scan_todos()
        self.show_toast("已立即巡检待办。", ACCENT)
        self.refresh_all()

    def open_workspace(self) -> None:
        self.controller.workspace.open_path(self.controller.workspace.workspace_root)

    def open_tasks_folder(self) -> None:
        self.controller.workspace.open_path(self.controller.workspace.tasks_root)

    def choose_workspace(self) -> None:
        selected = filedialog.askdirectory(title="选择工作区")
        if selected:
            self.controller.set_workspace(Path(selected))
            self.refresh_all()

    def add_scan_root(self) -> None:
        selected = filedialog.askdirectory(title="添加巡检目录")
        if selected:
            self.controller.add_scan_root(Path(selected))
            self.refresh_all()

    def remove_selected_scan_root(self) -> None:
        selection = self.dashboard.scan_root_listbox.curselection()
        if not selection:
            return
        value = self.dashboard.scan_root_listbox.get(selection[0])
        self.controller.remove_scan_root(Path(value))
        self.refresh_all()

    def reset_demo_tasks(self) -> None:
        self.controller.workspace.ensure_demo_content(force=True)
        self.controller.scan_todos()
        self.show_toast("样例待办已重建。", ACCENT)
        self.refresh_all()

    def apply_settings(self) -> None:
        self.controller.set_workspace(Path(self.dashboard.workspace_edit_var.get().strip() or self.controller.workspace.workspace_root))
        self.controller.set_scan_interval(int(self.dashboard.scan_interval_var.get()))
        self.controller.set_memory_enabled(bool(self.dashboard.memory_enabled_var.get()))
        self.controller.set_auto_scan(bool(self.dashboard.auto_scan_var.get()))
        self.controller.set_show_floating_ball(bool(self.dashboard.show_ball_var.get()))
        self.controller.set_command_text(self.dashboard.command_var.get().strip() or "dir")
        self.refresh_all()
        self.show_toast("设置已应用。", ACCENT)

    def request_video_summary_from_clipboard(self) -> None:
        state = self.controller.state
        if state.pending_video_approval_id:
            self.controller.respond_to_approval(state.pending_video_approval_id, True)
        elif state.clipboard_text:
            self.controller.handle_clipboard_text(state.clipboard_text)
        else:
            self.show_toast("当前没有可确认的视频链接。", WARN)
        self.refresh_all()

    def request_command_demo(self) -> None:
        approval = self.controller.request_command_demo(self.dashboard.command_var.get().strip())
        if approval:
            self.show_toast("命令演示已提交审批。", WARN)
            self.refresh_all()

    def approve_selected_approval(self) -> None:
        selection = self.dashboard.approval_tree.selection()
        if not selection:
            self.show_toast("请先选择一条审批记录。", WARN)
            return
        self.controller.respond_to_approval(selection[0], True)
        self.refresh_all()

    def reject_selected_approval(self) -> None:
        selection = self.dashboard.approval_tree.selection()
        if not selection:
            self.show_toast("请先选择一条审批记录。", WARN)
            return
        self.controller.respond_to_approval(selection[0], False)
        self.refresh_all()

    def show_dashboard(self) -> None:
        self.root.state("normal")
        self.root.deiconify()
        self._center_root_window()
        self.root.lift()
        self.root.attributes("-topmost", True)
        try:
            self.root.focus_force()
        except tk.TclError:
            pass
        self.root.after(200, lambda: self.root.attributes("-topmost", False))

    def hide_dashboard(self) -> None:
        self.root.withdraw()
        self.show_toast("主面板已隐藏，悬浮球仍在运行。", ACCENT)

    def show_chat(self) -> None:
        self.chat.show()

    def hide_ball(self) -> None:
        self.controller.set_show_floating_ball(False)
        self.ball.hide()
        self.refresh_all()

    def show_ball(self) -> None:
        self.controller.set_show_floating_ball(True)
        self.ball.show()
        self.refresh_all()

    def quit_app(self) -> None:
        if messagebox.askyesno("退出程序", "确认退出 CialloClaw Prototype 吗？"):
            self.root.destroy()

    def show_toast(self, text: str, color: str = ACCENT) -> None:
        toast = tk.Toplevel(self.root)
        toast.overrideredirect(True)
        toast.attributes("-topmost", True)
        toast.configure(bg="#101c2d")
        width, height = 320, 86
        x = toast.winfo_screenwidth() - width - 22
        y = toast.winfo_screenheight() - height - 64 - (len(self._toast_windows) * 12)
        toast.geometry(f"{width}x{height}+{x}+{y}")
        frame = tk.Frame(toast, bg="#101c2d", highlightbackground=color, highlightthickness=1)
        frame.pack(fill="both", expand=True)
        tk.Label(frame, text="CialloClaw", bg="#101c2d", fg=color, font=("Segoe UI Semibold", 10)).pack(anchor="w", padx=12, pady=(10, 0))
        tk.Label(frame, text=text, bg="#101c2d", fg="#e6eef8", wraplength=292, justify="left").pack(anchor="w", padx=12, pady=(2, 10))
        self._toast_windows.append(toast)

        def _close() -> None:
            if toast.winfo_exists():
                toast.destroy()
            if toast in self._toast_windows:
                self._toast_windows.remove(toast)

        toast.after(2600, _close)
