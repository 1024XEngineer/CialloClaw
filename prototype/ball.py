from __future__ import annotations

import tkinter as tk

BG = "#08111a"
ACCENT = "#4dd8c8"
ACCENT_2 = "#6ea8ff"
WARN = "#f0b84b"
DANGER = "#ff6b6b"
SUCCESS = "#46d39a"


class FloatingBall:
    def __init__(self, app) -> None:
        self.app = app
        self.drag = {"x": 0, "y": 0}
        self.window = tk.Toplevel(app.root)
        self.window.overrideredirect(True)
        self.window.attributes("-topmost", True)
        self.window.geometry("88x88+40+40")
        self.window.configure(bg=BG)
        self.window.protocol("WM_DELETE_WINDOW", self.window.withdraw)

        self.canvas = tk.Canvas(self.window, width=88, height=88, bg=BG, highlightthickness=0)
        self.canvas.pack(fill="both", expand=True)
        self.circle = self.canvas.create_oval(10, 10, 78, 78, fill=ACCENT_2, outline="#d8e8ff", width=2)
        self.text = self.canvas.create_text(44, 42, text="C", fill="#001520", font=("Segoe UI Semibold", 21))
        self.badge = self.canvas.create_oval(62, 60, 76, 74, fill=SUCCESS, outline="")
        self.menu = tk.Menu(self.window, tearoff=0, bg="#101c2d", fg="#e6eef8", activebackground="#16263c", activeforeground="#e6eef8")
        self.menu.add_command(label="打开主面板", command=self.app.show_dashboard)
        self.menu.add_command(label="打开聊天", command=self.app.show_chat)
        self.menu.add_command(label="暂停任务", command=self.app.controller.pause_active_task)
        self.menu.add_command(label="恢复任务", command=self.app.controller.resume_active_task)
        self.menu.add_command(label="终止任务", command=self.app.controller.terminate_active_task)
        self.menu.add_separator()
        self.menu.add_command(label="隐藏悬浮球", command=self.hide)
        self.menu.add_command(label="显示悬浮球", command=self.show)
        self.menu.add_separator()
        self.menu.add_command(label="退出", command=self.app.quit_app)

        self.canvas.bind("<Button-1>", lambda _evt: self.app.show_chat())
        self.canvas.bind("<Double-Button-1>", lambda _evt: self.app.show_dashboard())
        self.canvas.bind("<Button-3>", self._show_menu)
        self.canvas.bind("<ButtonPress-1>", self._start_drag)
        self.canvas.bind("<B1-Motion>", self._drag_move)
        self.canvas.bind("<Enter>", lambda _evt: self.canvas.itemconfig(self.circle, width=3))
        self.canvas.bind("<Leave>", lambda _evt: self.canvas.itemconfig(self.circle, width=2))

    def _show_menu(self, event: tk.Event) -> None:
        try:
            self.menu.tk_popup(event.x_root, event.y_root)
        finally:
            self.menu.grab_release()

    def _start_drag(self, event: tk.Event) -> None:
        self.drag["x"] = event.x
        self.drag["y"] = event.y

    def _drag_move(self, event: tk.Event) -> None:
        x = self.window.winfo_x() + event.x - self.drag["x"]
        y = self.window.winfo_y() + event.y - self.drag["y"]
        self.window.geometry(f"+{x}+{y}")

    def show(self) -> None:
        self.window.deiconify()
        self.window.lift()
        self.window.attributes("-topmost", True)

    def hide(self) -> None:
        self.window.withdraw()

    def refresh(self) -> None:
        state = self.app.controller.state
        if not state.show_floating_ball:
            self.hide()
            return
        if not self.window.winfo_viewable():
            self.show()
        color = ACCENT_2
        badge = SUCCESS
        if state.active_task:
            if state.active_task.status == "running":
                color = ACCENT
                badge = SUCCESS
            elif state.active_task.status == "paused":
                color = WARN
                badge = WARN
            elif state.active_task.status in {"terminated", "failed"}:
                color = DANGER
                badge = DANGER
            elif state.active_task.status == "completed":
                color = SUCCESS
                badge = SUCCESS
        elif state.clipboard_kind == "video":
            color = WARN
            badge = WARN
        elif state.clipboard_kind == "error":
            color = DANGER
            badge = DANGER
        self.canvas.itemconfig(self.circle, fill=color)
        self.canvas.itemconfig(self.badge, fill=badge)
        self.canvas.itemconfig(self.text, fill="#001520" if color != DANGER else "#220000")

