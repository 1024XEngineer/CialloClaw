from __future__ import annotations

import time
import tkinter as tk
from datetime import datetime
from pathlib import Path
from tkinter import filedialog, ttk
from tkinter.scrolledtext import ScrolledText

from .core import short_text

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
SUCCESS = "#46d39a"


def fmt_duration(seconds: float) -> str:
    seconds = max(0, int(seconds))
    hours, rem = divmod(seconds, 3600)
    minutes, secs = divmod(rem, 60)
    if hours:
        return f"{hours:02d}:{minutes:02d}:{secs:02d}"
    return f"{minutes:02d}:{secs:02d}"


def parse_iso(value: str) -> float:
    if not value:
        return time.time()
    try:
        return datetime.fromisoformat(value).timestamp()
    except Exception:
        return time.time()


class DashboardView:
    def __init__(self, root: tk.Tk, app) -> None:
        self.root = root
        self.app = app
        self._build()

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

    def _card(self, parent: tk.Widget, title: str) -> tuple[tk.Frame, tk.Frame]:
        frame = tk.Frame(parent, bg=CARD, bd=1, relief="solid")
        header = tk.Frame(frame, bg=CARD)
        header.pack(fill="x", padx=12, pady=(10, 6))
        tk.Label(header, text=title, bg=CARD, fg=TEXT, font=("Segoe UI Semibold", 11)).pack(anchor="w")
        body = tk.Frame(frame, bg=CARD)
        body.pack(fill="both", expand=True, padx=12, pady=(0, 12))
        return frame, body

    def _entry(self, parent: tk.Widget, textvariable: tk.Variable | None = None, width: int = 40, show: str | None = None) -> tk.Entry:
        entry = tk.Entry(
            parent,
            textvariable=textvariable,
            width=width,
            bg="#0b1422",
            fg=TEXT,
            insertbackground=TEXT,
            relief="flat",
            bd=0,
        )
        if show is not None:
            entry.configure(show=show)
        return entry

    def _build(self) -> None:
        top = tk.Frame(self.root, bg=BG)
        top.pack(fill="x", padx=18, pady=(16, 10))
        title_block = tk.Frame(top, bg=BG)
        title_block.pack(side="left", fill="x", expand=True)
        tk.Label(title_block, text="CialloClaw Prototype", bg=BG, fg=TEXT, font=("Segoe UI Semibold", 20)).pack(anchor="w")
        tk.Label(title_block, text="Windows local desktop agent demo", bg=BG, fg=MUTED, font=("Segoe UI", 10)).pack(anchor="w", pady=(2, 0))
        button_row = tk.Frame(top, bg=BG)
        button_row.pack(side="right")
        self._button(button_row, "聊天", self.app.show_chat, width=10).pack(side="left", padx=5)
        self._button(button_row, "隐藏", self.app.hide_dashboard, kind="muted", width=10).pack(side="left", padx=5)
        self._button(button_row, "工作区", self.app.open_workspace, kind="muted", width=10).pack(side="left", padx=5)
        self._button(button_row, "退出", self.app.quit_app, kind="danger", width=10).pack(side="left", padx=5)

        status = tk.Frame(self.root, bg=SURFACE)
        status.pack(fill="x", padx=18, pady=(0, 10))
        self.workspace_status_var = tk.StringVar()
        self.session_status_var = tk.StringVar()
        self.task_status_var = tk.StringVar()
        self.scan_status_var = tk.StringVar()
        self.clipboard_status_var = tk.StringVar()
        self.pending_approval_var = tk.StringVar()
        for idx, (label, var) in enumerate(
            [
                ("Workspace", self.workspace_status_var),
                ("Session", self.session_status_var),
                ("Task", self.task_status_var),
                ("Scan", self.scan_status_var),
                ("Clipboard", self.clipboard_status_var),
                ("Approval", self.pending_approval_var),
            ]
        ):
            cell = tk.Frame(status, bg=SURFACE)
            cell.grid(row=0, column=idx, sticky="nsew", padx=10, pady=10)
            tk.Label(cell, text=label, bg=SURFACE, fg=MUTED, font=("Segoe UI", 8)).pack(anchor="w")
            tk.Label(cell, textvariable=var, bg=SURFACE, fg=TEXT, font=("Segoe UI Semibold", 10), wraplength=180, justify="left").pack(anchor="w")
            status.grid_columnconfigure(idx, weight=1)

        self.notebook = ttk.Notebook(self.root)
        self.notebook.pack(fill="both", expand=True, padx=18, pady=(0, 16))
        self.overview_tab = tk.Frame(self.notebook, bg=BG)
        self.tasks_tab = tk.Frame(self.notebook, bg=BG)
        self.logs_tab = tk.Frame(self.notebook, bg=BG)
        self.settings_tab = tk.Frame(self.notebook, bg=BG)
        self.notebook.add(self.overview_tab, text="总览")
        self.notebook.add(self.tasks_tab, text="待办")
        self.notebook.add(self.logs_tab, text="日志")
        self.notebook.add(self.settings_tab, text="设置")

        self._build_overview()
        self._build_tasks()
        self._build_logs()
        self._build_settings()

        bottom = tk.Frame(self.root, bg=BG)
        bottom.pack(fill="x", padx=18, pady=(0, 12))
        self.status_bar_var = tk.StringVar(value="准备就绪")
        tk.Label(bottom, textvariable=self.status_bar_var, bg=BG, fg=MUTED, anchor="w").pack(side="left")
        tk.Label(bottom, text="本地优先 · 不依赖云端存储", bg=BG, fg=MUTED, anchor="e").pack(side="right")

    def _build_overview(self) -> None:
        self.overview_tab.grid_columnconfigure(0, weight=1)
        self.overview_tab.grid_columnconfigure(1, weight=1)
        self.overview_tab.grid_rowconfigure(0, weight=1)
        self.overview_tab.grid_rowconfigure(1, weight=1)

        card_clip, clip_body = self._card(self.overview_tab, "剪贴板")
        card_clip.grid(row=0, column=0, sticky="nsew", padx=(0, 8), pady=(0, 8))
        self.clip_kind_var = tk.StringVar()
        self.clip_note_var = tk.StringVar()
        self.clip_preview_var = tk.StringVar()
        tk.Label(clip_body, text="类型", bg=CARD, fg=MUTED).pack(anchor="w")
        tk.Label(clip_body, textvariable=self.clip_kind_var, bg=CARD, fg=ACCENT, font=("Segoe UI Semibold", 11)).pack(anchor="w")
        tk.Label(clip_body, text="提示", bg=CARD, fg=MUTED).pack(anchor="w", pady=(8, 0))
        tk.Label(clip_body, textvariable=self.clip_note_var, bg=CARD, fg=TEXT, wraplength=460, justify="left").pack(anchor="w")
        tk.Label(clip_body, text="预览", bg=CARD, fg=MUTED).pack(anchor="w", pady=(8, 0))
        tk.Message(clip_body, textvariable=self.clip_preview_var, bg=CARD, fg=TEXT, width=460, anchor="nw", justify="left").pack(fill="x")

        card_task, task_body = self._card(self.overview_tab, "当前任务")
        card_task.grid(row=0, column=1, sticky="nsew", padx=(8, 0), pady=(0, 8))
        self.task_title_var = tk.StringVar()
        self.task_state_var = tk.StringVar()
        self.task_step_var = tk.StringVar()
        self.task_elapsed_var = tk.StringVar()
        self.task_result_var = tk.StringVar()
        for label, var in [
            ("标题", self.task_title_var),
            ("状态", self.task_state_var),
            ("步骤", self.task_step_var),
            ("耗时", self.task_elapsed_var),
            ("结果", self.task_result_var),
        ]:
            tk.Label(task_body, text=label, bg=CARD, fg=MUTED).pack(anchor="w")
            tk.Label(task_body, textvariable=var, bg=CARD, fg=TEXT, wraplength=460, justify="left").pack(anchor="w", pady=(0, 6))
        self.task_timeline_text = ScrolledText(task_body, height=8, bg="#0b1422", fg=TEXT, insertbackground=TEXT, relief="flat", wrap="word")
        self.task_timeline_text.pack(fill="both", expand=True, pady=(8, 0))
        self.task_timeline_text.configure(state="disabled")

        card_quick, quick_body = self._card(self.overview_tab, "快捷动作")
        card_quick.grid(row=1, column=0, sticky="nsew", padx=(0, 8), pady=(0, 8))
        self.quick_action_frame = tk.Frame(quick_body, bg=CARD)
        self.quick_action_frame.pack(fill="both", expand=True)
        self.quick_action_status_var = tk.StringVar()
        tk.Label(quick_body, textvariable=self.quick_action_status_var, bg=CARD, fg=TEXT, wraplength=460, justify="left").pack(anchor="w")

        card_workspace, workspace_body = self._card(self.overview_tab, "工作区与记忆")
        card_workspace.grid(row=1, column=1, sticky="nsew", padx=(8, 0), pady=(0, 8))
        self.workspace_var = tk.StringVar()
        self.scan_roots_var = tk.StringVar()
        self.memory_var = tk.StringVar()
        self.todo_count_var = tk.StringVar()
        self.approval_count_var = tk.StringVar()
        self.scan_summary_var = tk.StringVar()
        for label, var in [
            ("工作区", self.workspace_var),
            ("巡检目录", self.scan_roots_var),
            ("记忆", self.memory_var),
            ("待办", self.todo_count_var),
            ("审批", self.approval_count_var),
            ("巡检摘要", self.scan_summary_var),
        ]:
            tk.Label(workspace_body, text=label, bg=CARD, fg=MUTED).pack(anchor="w")
            tk.Label(workspace_body, textvariable=var, bg=CARD, fg=TEXT, wraplength=460, justify="left").pack(anchor="w", pady=(0, 6))
        actions = tk.Frame(workspace_body, bg=CARD)
        actions.pack(fill="x", pady=(8, 0))
        self._button(actions, "切换工作区", self.app.choose_workspace, width=12).pack(side="left", padx=(0, 6))
        self._button(actions, "打开工作区", self.app.open_workspace, kind="muted", width=12).pack(side="left", padx=(0, 6))
        self._button(actions, "生成记忆", self.app.controller.generate_memory_snapshot, width=12).pack(side="left", padx=(0, 6))
        self._button(actions, "刷新待办", self.app.refresh_tasks_now, kind="muted", width=12).pack(side="left")

        card_actions, action_body = self._card(self.overview_tab, "审批与任务控制")
        card_actions.grid(row=2, column=0, columnspan=2, sticky="nsew")
        self.latest_approval_var = tk.StringVar()
        tk.Label(action_body, text="最新提醒", bg=CARD, fg=MUTED).pack(anchor="w")
        tk.Label(action_body, textvariable=self.latest_approval_var, bg=CARD, fg=TEXT, wraplength=940, justify="left").pack(anchor="w", pady=(0, 8))
        control_row = tk.Frame(action_body, bg=CARD)
        control_row.pack(fill="x")
        self._button(control_row, "打开日志", lambda: self.notebook.select(self.logs_tab), kind="muted", width=12).pack(side="left", padx=(0, 6))
        self._button(control_row, "暂停任务", self.app.controller.pause_active_task, kind="warn", width=12).pack(side="left", padx=(0, 6))
        self._button(control_row, "恢复任务", self.app.controller.resume_active_task, width=12).pack(side="left", padx=(0, 6))
        self._button(control_row, "终止任务", self.app.controller.terminate_active_task, kind="danger", width=12).pack(side="left")

    def _build_tasks(self) -> None:
        self.tasks_tab.grid_columnconfigure(0, weight=2)
        self.tasks_tab.grid_columnconfigure(1, weight=1)
        self.tasks_tab.grid_rowconfigure(1, weight=1)
        header = tk.Frame(self.tasks_tab, bg=BG)
        header.grid(row=0, column=0, columnspan=2, sticky="ew", pady=(0, 10))
        self.todo_summary_var = tk.StringVar()
        tk.Label(header, textvariable=self.todo_summary_var, bg=BG, fg=TEXT, font=("Segoe UI Semibold", 11)).pack(side="left")
        right = tk.Frame(header, bg=BG)
        right.pack(side="right")
        self._button(right, "立即巡检", self.app.refresh_tasks_now, width=12).pack(side="left", padx=4)
        self._button(right, "添加目录", self.app.add_scan_root, kind="muted", width=12).pack(side="left", padx=4)
        self._button(right, "打开目录", self.app.open_tasks_folder, kind="muted", width=12).pack(side="left", padx=4)

        left_card, left_body = self._card(self.tasks_tab, "Markdown 待办")
        left_card.grid(row=1, column=0, sticky="nsew", padx=(0, 8))
        self.todo_tree = ttk.Treeview(left_body, columns=("status", "file", "line", "text"), show="headings", height=18)
        for key, text in [("status", "状态"), ("file", "文件"), ("line", "行"), ("text", "内容")]:
            self.todo_tree.heading(key, text=text)
        self.todo_tree.column("status", width=70, anchor="center")
        self.todo_tree.column("file", width=250, anchor="w")
        self.todo_tree.column("line", width=55, anchor="center")
        self.todo_tree.column("text", width=390, anchor="w")
        self.todo_tree.pack(fill="both", expand=True)
        self.todo_tree.bind("<<TreeviewSelect>>", lambda _evt: self._render_todo_detail())

        right_card, right_body = self._card(self.tasks_tab, "详情与历史")
        right_card.grid(row=1, column=1, sticky="nsew", padx=(8, 0))
        self.todo_detail_text = ScrolledText(right_body, height=10, bg="#0b1422", fg=TEXT, insertbackground=TEXT, relief="flat", wrap="word")
        self.todo_detail_text.pack(fill="both", expand=True)
        self.todo_detail_text.configure(state="disabled")
        tk.Label(right_body, text="任务历史", bg=CARD, fg=MUTED).pack(anchor="w", pady=(10, 4))
        self.task_history_text = ScrolledText(right_body, height=8, bg="#0b1422", fg=TEXT, insertbackground=TEXT, relief="flat", wrap="word")
        self.task_history_text.pack(fill="both", expand=True)
        self.task_history_text.configure(state="disabled")

    def _build_logs(self) -> None:
        self.logs_tab.grid_columnconfigure(0, weight=2)
        self.logs_tab.grid_columnconfigure(1, weight=1)
        self.logs_tab.grid_rowconfigure(1, weight=1)
        header = tk.Frame(self.logs_tab, bg=BG)
        header.grid(row=0, column=0, columnspan=2, sticky="ew", pady=(0, 10))
        self.log_summary_var = tk.StringVar()
        tk.Label(header, textvariable=self.log_summary_var, bg=BG, fg=TEXT, font=("Segoe UI Semibold", 11)).pack(side="left")
        right = tk.Frame(header, bg=BG)
        right.pack(side="right")
        self._button(right, "导出记忆", self.app.controller.generate_memory_snapshot, width=12).pack(side="left", padx=4)
        self._button(right, "清空记忆", self.app.controller.clear_memory, kind="warn", width=12).pack(side="left", padx=4)
        self._button(right, "再次巡检", self.app.refresh_tasks_now, kind="muted", width=12).pack(side="left", padx=4)

        log_card, log_body = self._card(self.logs_tab, "执行日志")
        log_card.grid(row=1, column=0, sticky="nsew", padx=(0, 8))
        self.log_text = ScrolledText(log_body, bg="#0b1422", fg=TEXT, insertbackground=TEXT, relief="flat", wrap="word")
        self.log_text.pack(fill="both", expand=True)
        self.log_text.configure(state="disabled")

        approval_card, approval_body = self._card(self.logs_tab, "审批记录")
        approval_card.grid(row=1, column=1, sticky="nsew", padx=(8, 0))
        self.approval_tree = ttk.Treeview(approval_body, columns=("action", "risk", "status", "desc"), show="headings", height=14)
        for key, text in [("action", "动作"), ("risk", "风险"), ("status", "状态"), ("desc", "说明")]:
            self.approval_tree.heading(key, text=text)
        self.approval_tree.column("action", width=100, anchor="center")
        self.approval_tree.column("risk", width=70, anchor="center")
        self.approval_tree.column("status", width=90, anchor="center")
        self.approval_tree.column("desc", width=320, anchor="w")
        self.approval_tree.pack(fill="both", expand=True)
        buttons = tk.Frame(approval_body, bg=CARD)
        buttons.pack(fill="x", pady=(8, 0))
        self._button(buttons, "批准", self.app.approve_selected_approval, width=12).pack(side="left", padx=(0, 6))
        self._button(buttons, "拒绝", self.app.reject_selected_approval, kind="danger", width=12).pack(side="left", padx=(0, 6))
        self._button(buttons, "视频总结", self.app.request_video_summary_from_clipboard, kind="muted", width=12).pack(side="left")

    def _build_settings(self) -> None:
        self.settings_tab.grid_columnconfigure(0, weight=1)
        card_workspace, workspace_body = self._card(self.settings_tab, "工作区")
        card_workspace.grid(row=0, column=0, sticky="ew", pady=(0, 8))
        self.workspace_edit_var = tk.StringVar()
        row = tk.Frame(workspace_body, bg=CARD)
        row.pack(fill="x")
        self._entry(row, self.workspace_edit_var, width=58).pack(side="left", padx=(0, 8), fill="x", expand=True)
        self._button(row, "浏览", self.app.choose_workspace, kind="muted", width=10).pack(side="left", padx=(0, 8))
        self._button(row, "应用", self.app.apply_settings, width=10).pack(side="left")
        tk.Label(workspace_body, text="扫描目录", bg=CARD, fg=MUTED).pack(anchor="w", pady=(10, 4))
        list_row = tk.Frame(workspace_body, bg=CARD)
        list_row.pack(fill="both", expand=True)
        self.scan_root_listbox = tk.Listbox(list_row, bg="#0b1422", fg=TEXT, selectbackground=ACCENT_2, relief="flat", height=5)
        self.scan_root_listbox.pack(side="left", fill="both", expand=True)
        list_buttons = tk.Frame(list_row, bg=CARD)
        list_buttons.pack(side="left", padx=(10, 0), fill="y")
        self._button(list_buttons, "添加目录", self.app.add_scan_root, kind="muted", width=12).pack(pady=(0, 6))
        self._button(list_buttons, "移除选中", self.app.remove_selected_scan_root, kind="warn", width=12).pack(pady=(0, 6))
        self._button(list_buttons, "打开工作区", self.app.open_workspace, kind="muted", width=12).pack(pady=(0, 6))
        self._button(list_buttons, "重建样例", self.app.reset_demo_tasks, kind="muted", width=12).pack(pady=(0, 6))

        card_flags, flag_body = self._card(self.settings_tab, "巡检与入口")
        card_flags.grid(row=1, column=0, sticky="ew", pady=(0, 8))
        self.memory_enabled_var = tk.BooleanVar()
        self.auto_scan_var = tk.BooleanVar()
        self.show_ball_var = tk.BooleanVar()
        self.scan_interval_var = tk.IntVar()
        scan_row = tk.Frame(flag_body, bg=CARD)
        scan_row.pack(fill="x")
        tk.Label(scan_row, text="巡检间隔（秒）", bg=CARD, fg=MUTED).pack(side="left")
        tk.Spinbox(scan_row, from_=1, to=120, width=6, textvariable=self.scan_interval_var, bg="#0b1422", fg=TEXT, insertbackground=TEXT, relief="flat").pack(side="left", padx=(8, 18))
        tk.Checkbutton(scan_row, text="自动巡检", variable=self.auto_scan_var, bg=CARD, fg=TEXT, selectcolor=CARD, activebackground=CARD).pack(side="left", padx=(0, 18))
        tk.Checkbutton(scan_row, text="开启镜子记忆", variable=self.memory_enabled_var, bg=CARD, fg=TEXT, selectcolor=CARD, activebackground=CARD).pack(side="left", padx=(0, 18))
        tk.Checkbutton(scan_row, text="显示悬浮球", variable=self.show_ball_var, bg=CARD, fg=TEXT, selectcolor=CARD, activebackground=CARD).pack(side="left")

        card_exec, exec_body = self._card(self.settings_tab, "命令演示与记忆")
        card_exec.grid(row=2, column=0, sticky="ew")
        self.command_var = tk.StringVar()
        command_row = tk.Frame(exec_body, bg=CARD)
        command_row.pack(fill="x")
        self._entry(command_row, self.command_var, width=52).pack(side="left", padx=(0, 8), fill="x", expand=True)
        self._button(command_row, "请求审批", self.app.request_command_demo, width=12).pack(side="left", padx=(0, 8))
        self._button(command_row, "生成记忆", self.app.controller.generate_memory_snapshot, width=12).pack(side="left")
        tk.Label(exec_body, text="当前原型把命令执行做成审批演示，便于先验证工作区、审批和日志流转。", bg=CARD, fg=MUTED, wraplength=980, justify="left").pack(anchor="w", pady=(10, 0))

    def refresh(self) -> None:
        state = self.app.controller.state
        self.workspace_status_var.set(short_text(state.workspace_root, 60) or "未设置")
        self.session_status_var.set(f"{state.session.session_id} · {fmt_duration(time.time() - parse_iso(state.session.started_at))}")
        self.task_status_var.set(f"{state.active_task.title} · {state.active_task.status}" if state.active_task else "空闲")
        self.scan_status_var.set(state.last_scan_summary or "尚未巡检")
        self.clipboard_status_var.set(f"{state.clipboard_kind or 'idle'} · {short_text(state.clipboard_hint or '等待剪贴板', 70)}")
        pending = [item for item in state.approvals if item.status == "pending"]
        self.pending_approval_var.set(f"{len(pending)} 项待审批")
        self.latest_approval_var.set(pending[-1].description if pending else "暂无待审批项。")

        self.workspace_var.set(state.workspace_root)
        self.scan_roots_var.set("\n".join(state.scan_roots))
        self.memory_var.set("开启" if state.memory_enabled else "关闭")
        self.todo_count_var.set(f"{len(state.todo_items)} 条待办，未完成 {sum(not item.checked for item in state.todo_items)} 条")
        self.approval_count_var.set(str(len(pending)))
        self.scan_summary_var.set(state.last_scan_summary or "尚未巡检")
        self.todo_summary_var.set(state.last_scan_summary or "尚未巡检")
        self.log_summary_var.set(f"最近 {len(state.logs)} 条日志 · 审批 {len(state.approvals)} 项")

        self.workspace_edit_var.set(state.workspace_root)
        self.scan_interval_var.set(state.scan_interval_seconds)
        self.memory_enabled_var.set(state.memory_enabled)
        self.auto_scan_var.set(state.auto_scan)
        self.show_ball_var.set(state.show_floating_ball)
        self.command_var.set(state.command_text)

        self.scan_root_listbox.delete(0, "end")
        for root in state.scan_roots:
            self.scan_root_listbox.insert("end", root)

        self.clip_kind_var.set(state.clipboard_kind or "idle")
        self.clip_note_var.set(state.clipboard_hint or "等待剪贴板内容")
        self.clip_preview_var.set(short_text(state.clipboard_text, 280) if state.clipboard_text else "复制一些文本、错误信息或视频链接，原型会给出对应提示。")
        self.quick_action_status_var.set(" · ".join(state.clipboard_actions) if state.clipboard_actions else "等待剪贴板内容以生成快捷动作")
        self._rebuild_quick_actions()

        if state.active_task:
            task = state.active_task
            total = max(len(task.steps), 1)
            current = min(task.current_step_index + 1, total)
            self.task_title_var.set(task.title)
            self.task_state_var.set(task.status)
            self.task_step_var.set(f"{current}/{total} · {task.current_step_label}")
            self.task_elapsed_var.set(fmt_duration(time.time() - parse_iso(task.created_at)))
            self.task_result_var.set(short_text(task.result or "任务处理中...", 180))
            self.task_timeline_text.configure(state="normal")
            self.task_timeline_text.delete("1.0", "end")
            for line in task.timeline[-12:]:
                self.task_timeline_text.insert("end", line + "\n")
            self.task_timeline_text.configure(state="disabled")
        else:
            self.task_title_var.set("暂无任务")
            self.task_state_var.set("idle")
            self.task_step_var.set("无")
            self.task_elapsed_var.set("00:00")
            self.task_result_var.set("暂无任务")
            self.task_timeline_text.configure(state="normal")
            self.task_timeline_text.delete("1.0", "end")
            self.task_timeline_text.insert("end", "暂无任务。\n")
            self.task_timeline_text.configure(state="disabled")

        for row in self.todo_tree.get_children():
            self.todo_tree.delete(row)
        for item in state.todo_items:
            self.todo_tree.insert("", "end", values=("完成" if item.checked else "待办", item.relative_path, item.line_no, item.text))
        if not self.todo_tree.selection() and self.todo_tree.get_children():
            self.todo_tree.selection_set(self.todo_tree.get_children()[0])
        self._render_todo_detail()

        self.task_history_text.configure(state="normal")
        self.task_history_text.delete("1.0", "end")
        for task in state.task_history[-8:]:
            self.task_history_text.insert("end", f"{task.title} [{task.status}]\n")
            self.task_history_text.insert("end", f"  {task.summary or short_text(task.result, 120)}\n")
            for line in task.timeline[-3:]:
                self.task_history_text.insert("end", f"  {line}\n")
            self.task_history_text.insert("end", "\n")
        if not state.task_history:
            self.task_history_text.insert("end", "暂无历史任务。\n")
        self.task_history_text.configure(state="disabled")

        self.log_text.configure(state="normal")
        self.log_text.delete("1.0", "end")
        for log in state.logs[-200:]:
            self.log_text.insert("end", f"{log.timestamp} [{log.level.upper()}] {log.category} - {log.message}\n")
            if log.details:
                self.log_text.insert("end", f"    {log.details}\n")
        self.log_text.configure(state="disabled")

        for row in self.approval_tree.get_children():
            self.approval_tree.delete(row)
        for approval in state.approvals[-60:]:
            self.approval_tree.insert("", "end", iid=approval.id, values=(approval.action, approval.risk, approval.status, short_text(approval.description, 48)))

        self.status_bar_var.set(state.last_scan_summary or "准备就绪")

    def _rebuild_quick_actions(self) -> None:
        for child in self.quick_action_frame.winfo_children():
            child.destroy()
        state = self.app.controller.state
        actions = state.clipboard_actions or ["总结", "翻译", "解释", "下一步"]
        mapping = {"总结": "summary", "翻译": "translate", "解释": "explain", "下一步": "next", "排查问题": "troubleshoot"}
        for idx, label in enumerate(actions):
            if label == "开始视频总结" and state.pending_video_approval_id:
                cmd = lambda pid=state.pending_video_approval_id: self.app.controller.respond_to_approval(pid, True)
            elif label == "忽略" and state.pending_video_approval_id:
                cmd = lambda pid=state.pending_video_approval_id: self.app.controller.respond_to_approval(pid, False)
            else:
                cmd = lambda k=mapping.get(label, label): self.app.controller.start_clipboard_action(k)
            self._button(self.quick_action_frame, label, cmd, kind="muted", width=12).grid(row=idx // 3, column=idx % 3, padx=5, pady=5, sticky="ew")

    def _render_todo_detail(self) -> None:
        selected = self.todo_tree.selection()
        state = self.app.controller.state
        self.todo_detail_text.configure(state="normal")
        self.todo_detail_text.delete("1.0", "end")
        if not selected:
            self.todo_detail_text.insert("end", "选择一条待办查看详情。\n")
        else:
            index = self.todo_tree.index(selected[0])
            if 0 <= index < len(state.todo_items):
                item = state.todo_items[index]
                self.todo_detail_text.insert("end", f"文件：{item.relative_path}\n")
                self.todo_detail_text.insert("end", f"行号：{item.line_no}\n")
                self.todo_detail_text.insert("end", f"状态：{'完成' if item.checked else '未完成'}\n")
                self.todo_detail_text.insert("end", f"内容：{item.text}\n")
                self.todo_detail_text.insert("end", f"mtime：{item.mtime}\n")
        self.todo_detail_text.configure(state="disabled")
