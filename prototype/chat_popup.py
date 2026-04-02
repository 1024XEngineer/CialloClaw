from __future__ import annotations

import tkinter as tk
from tkinter.scrolledtext import ScrolledText

from .core import is_video_url

BG = "#08111a"
SURFACE = "#101c2d"
CARD = "#16263c"
CARD_ALT = "#132235"
TEXT = "#e6eef8"
MUTED = "#8da2bd"
ACCENT = "#4dd8c8"
ACCENT_2 = "#6ea8ff"
WARN = "#f0b84b"
DANGER = "#ff6b6b"


class ChatPopup:
    def __init__(self, app) -> None:
        self.app = app
        self.window = tk.Toplevel(app.root)
        self.window.title("聊天弹层")
        self.window.geometry("520x640+860+140")
        self.window.configure(bg=SURFACE)
        self.window.protocol("WM_DELETE_WINDOW", self.hide)
        self.window.withdraw()
        self.window.attributes("-topmost", True)

        header = tk.Frame(self.window, bg=CARD)
        header.pack(fill="x")
        left = tk.Frame(header, bg=CARD)
        left.pack(side="left", fill="x", expand=True, padx=12, pady=10)
        tk.Label(left, text="聊天弹层", bg=CARD, fg=TEXT, font=("Segoe UI Semibold", 13)).pack(anchor="w")
        self.status_var = tk.StringVar(value="等待剪贴板输入")
        tk.Label(left, textvariable=self.status_var, bg=CARD, fg=MUTED).pack(anchor="w")
        actions = tk.Frame(header, bg=CARD)
        actions.pack(side="right", padx=10, pady=10)
        self._button(actions, "主面板", self.app.show_dashboard, width=9).pack(side="left", padx=4)
        self._button(actions, "最小化", self.hide, kind="muted", width=9).pack(side="left", padx=4)

        self.banner = tk.Frame(self.window, bg="#2a220c")
        self.banner.pack(fill="x", padx=12, pady=(12, 6))
        self.banner_var = tk.StringVar()
        tk.Label(self.banner, textvariable=self.banner_var, bg="#2a220c", fg="#f5d68a", wraplength=480, justify="left").pack(side="left", padx=10, pady=8)
        self.banner_buttons = tk.Frame(self.banner, bg="#2a220c")
        self.banner_buttons.pack(side="right", padx=10, pady=8)
        self.banner.pack_forget()

        self.history = ScrolledText(self.window, bg="#0b1422", fg=TEXT, insertbackground=TEXT, relief="flat", wrap="word")
        self.history.pack(fill="both", expand=True, padx=12, pady=(6, 8))
        self.history.configure(state="disabled")

        self.context_var = tk.StringVar()
        tk.Label(self.window, textvariable=self.context_var, bg=SURFACE, fg=MUTED, anchor="w").pack(fill="x", padx=12, pady=(0, 6))

        self.actions_frame = tk.Frame(self.window, bg=SURFACE)
        self.actions_frame.pack(fill="x", padx=12)

        input_row = tk.Frame(self.window, bg=SURFACE)
        input_row.pack(fill="x", padx=12, pady=(8, 12))
        self.input_var = tk.StringVar()
        self.input = tk.Entry(input_row, textvariable=self.input_var, bg="#0b1422", fg=TEXT, insertbackground=TEXT, relief="flat", bd=0, highlightthickness=1, highlightbackground="#26364f", highlightcolor=ACCENT_2)
        self.input.pack(side="left", fill="x", expand=True, padx=(0, 8))
        self.input.bind("<Return>", lambda _evt: self.submit_message())
        self._button(input_row, "发送", self.submit_message, width=10).pack(side="left")

    def _button(self, parent: tk.Widget, text: str, command, kind: str = "accent", width: int = 14) -> tk.Button:
        if kind == "warn":
            bg = WARN
            fg = "#1a1300"
        elif kind == "danger":
            bg = DANGER
            fg = "#240000"
        elif kind == "muted":
            bg = CARD_ALT
            fg = TEXT
        else:
            bg = ACCENT
            fg = "#001a18"
        return tk.Button(parent, text=text, command=command, bg=bg, fg=fg, activebackground=bg, activeforeground=fg, relief="flat", bd=0, padx=10, pady=6, width=width)

    def show(self) -> None:
        self.window.deiconify()
        self.window.lift()
        self.window.attributes("-topmost", True)
        self.window.after(200, lambda: self.window.attributes("-topmost", True))
        self.input.focus_set()

    def hide(self) -> None:
        self.window.withdraw()

    def append_message(self, role: str, text: str) -> None:
        self.history.configure(state="normal")
        tag = "user" if role == "user" else "assistant" if role == "assistant" else "system"
        prefix = {"user": "你", "assistant": "原型", "system": "系统"}.get(role, role)
        self.history.insert("end", f"{prefix}：\n", tag)
        self.history.insert("end", f"{text.strip()}\n\n", tag)
        self.history.tag_configure("user", foreground=ACCENT_2)
        self.history.tag_configure("assistant", foreground=TEXT)
        self.history.tag_configure("system", foreground=WARN)
        self.history.configure(state="disabled")
        self.history.see("end")

    def submit_message(self) -> None:
        text = self.input_var.get().strip()
        if not text:
            return
        self.input_var.set("")
        self.append_message("user", text)
        if text.startswith("/summary"):
            self.app.controller.start_clipboard_action("summary")
            reply = self.app.controller.generate_chat_reply(self.app.controller.state.clipboard_text or text)
        elif text.startswith("/translate"):
            self.app.controller.start_clipboard_action("translate")
            reply = self.app.controller.generate_chat_reply(self.app.controller.state.clipboard_text or text)
        elif text.startswith("/explain"):
            self.app.controller.start_clipboard_action("explain")
            reply = self.app.controller.generate_chat_reply(self.app.controller.state.clipboard_text or text)
        elif text.startswith("/next"):
            self.app.controller.start_clipboard_action("next")
            reply = self.app.controller.generate_chat_reply(self.app.controller.state.clipboard_text or text)
        elif text.startswith("/video"):
            parts = text.split(maxsplit=1)
            if len(parts) > 1:
                self.app.controller.handle_clipboard_text(parts[1])
                reply = "已识别到视频链接，等待确认。"
            else:
                reply = "请在 /video 后面附上链接。"
        else:
            reply = self.app.controller.generate_chat_reply(text)
        self.append_message("assistant", reply)
        self.app.refresh_all()
        self.show()

    def refresh(self) -> None:
        state = self.app.controller.state
        self.status_var.set(f"{state.clipboard_kind or 'idle'} · {state.active_task.title if state.active_task else '空闲'}")
        self.context_var.set(f"上下文：{state.clipboard_text[:90] if state.clipboard_text else '无'}")
        if state.pending_video_approval_id and state.pending_video_url:
            self.banner_var.set(f"检测到视频链接，是否现在生成视频总结？\n{state.pending_video_url}")
            self.banner.pack(fill="x", padx=12, pady=(12, 6))
            for child in self.banner_buttons.winfo_children():
                child.destroy()
            self._button(self.banner_buttons, "同意", lambda pid=state.pending_video_approval_id: self.app.controller.respond_to_approval(pid, True), width=8).pack(side="left", padx=4)
            self._button(self.banner_buttons, "拒绝", lambda pid=state.pending_video_approval_id: self.app.controller.respond_to_approval(pid, False), kind="danger", width=8).pack(side="left", padx=4)
        else:
            self.banner.pack_forget()

        for child in self.actions_frame.winfo_children():
            child.destroy()
        actions = state.clipboard_actions or ["总结", "翻译", "解释", "下一步"]
        mapping = {"总结": "summary", "翻译": "translate", "解释": "explain", "下一步": "next", "排查问题": "troubleshoot"}
        for idx, label in enumerate(actions):
            if label == "开始视频总结" and state.pending_video_approval_id:
                cmd = lambda pid=state.pending_video_approval_id: self.app.controller.respond_to_approval(pid, True)
            elif label == "忽略" and state.pending_video_approval_id:
                cmd = lambda pid=state.pending_video_approval_id: self.app.controller.respond_to_approval(pid, False)
            else:
                cmd = lambda k=mapping.get(label, label): self.app.controller.start_clipboard_action(k)
            self._button(self.actions_frame, label, cmd, kind="muted", width=12).grid(row=idx // 3, column=idx % 3, padx=4, pady=4, sticky="ew")

