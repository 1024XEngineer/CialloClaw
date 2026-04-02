from __future__ import annotations

import json
from pathlib import Path
from typing import Any, Callable

from .core import (
    ApprovalRequest,
    AppState,
    ClipboardAnalysis,
    EventBus,
    LocalAssistant,
    LogEntry,
    TaskRecord,
    WorkspaceManager,
    extract_urls,
    format_memory_summary,
    is_error_text,
    is_video_url,
    new_id,
    now_iso,
    scan_markdown_tasks,
    short_text,
    video_url_from_text,
)


class PrototypeController:
    def __init__(
        self,
        project_root: Path,
        schedule: Callable[[int, Callable[[], None]], Any],
        cancel: Callable[[Any], None],
    ) -> None:
        self.project_root = Path(project_root).resolve()
        self.workspace = WorkspaceManager(self.project_root)
        self.schedule = schedule
        self.cancel = cancel
        self.bus = EventBus()
        self.assistant = LocalAssistant()
        self.state: AppState = self.workspace.default_state()
        self._task_timer: Any = None
        self._task_builder: Callable[[TaskRecord, AppState], str] | None = None
        self._step_delay_ms = 1100

    def start(self) -> None:
        self.workspace.ensure_layout()
        self.workspace.ensure_demo_content()
        loaded = self.workspace.load_state()
        loaded.session = type(loaded.session)(
            session_id=new_id("session"),
            started_at=now_iso(),
            status="running",
            focus_mode="assistant",
        )
        loaded.workspace_root = str(self.workspace.workspace_root)
        if not loaded.scan_roots:
            loaded.scan_roots = [str(self.workspace.tasks_root)]
        loaded.task_mtimes = dict(loaded.task_mtimes)
        self.state = loaded
        self.workspace.save_state(self.state)
        self.log("info", "system.startup", "原型已启动")
        self.scan_todos(initial=True)
        self.start_boot_task()
        self.persist()
        self.bus.emit("state.changed", state=self.state)

    def persist(self) -> None:
        self.workspace.save_state(self.state)

    def log(
        self,
        level: str,
        category: str,
        message: str,
        task_id: str = "",
        details: dict[str, object] | None = None,
    ) -> LogEntry:
        entry = LogEntry(
            timestamp=now_iso(),
            level=level,
            category=category,
            message=message,
            task_id=task_id,
            details=details or {},
        )
        self.state.logs.append(entry)
        self.state.logs = self.state.logs[-300:]
        self.workspace.append_log(entry)
        self.persist()
        self.bus.emit("log.appended", log=entry)
        return entry

    def _normalize_scan_roots(self, roots: list[str]) -> list[str]:
        cleaned: list[str] = []
        for root in roots:
            path = str(Path(root).expanduser().resolve())
            if path not in cleaned:
                cleaned.append(path)
        return cleaned

    def set_workspace(self, workspace_root: Path) -> None:
        self.workspace.set_workspace_root(workspace_root)
        self.workspace.ensure_layout()
        self.workspace.ensure_demo_content()
        self.state.workspace_root = str(self.workspace.workspace_root)
        self.state.scan_roots = [str(self.workspace.tasks_root)]
        self.state.todo_items = []
        self.state.task_mtimes = {}
        self.state.last_scan_summary = ""
        self.state.last_scan_at = ""
        self.log("info", "workspace.changed", f"工作区已切换到 {self.state.workspace_root}")
        self.persist()
        self.bus.emit("workspace.changed", state=self.state)

    def set_scan_interval(self, seconds: int) -> None:
        self.state.scan_interval_seconds = max(1, int(seconds))
        self.log("info", "settings.changed", f"巡检间隔设置为 {self.state.scan_interval_seconds} 秒")
        self.persist()
        self.bus.emit("state.changed", state=self.state)

    def set_memory_enabled(self, enabled: bool) -> None:
        self.state.memory_enabled = bool(enabled)
        self.log("info", "settings.changed", f"镜子记忆已{'开启' if enabled else '关闭'}")
        self.persist()
        self.bus.emit("state.changed", state=self.state)

    def set_auto_scan(self, enabled: bool) -> None:
        self.state.auto_scan = bool(enabled)
        self.log("info", "settings.changed", f"自动巡检已{'开启' if enabled else '关闭'}")
        self.persist()
        self.bus.emit("state.changed", state=self.state)

    def set_show_floating_ball(self, enabled: bool) -> None:
        self.state.show_floating_ball = bool(enabled)
        self.log("info", "settings.changed", f"悬浮球已{'显示' if enabled else '隐藏'}")
        self.persist()
        self.bus.emit("state.changed", state=self.state)

    def set_command_text(self, command: str) -> None:
        self.state.command_text = command.strip() or "dir"
        self.persist()
        self.bus.emit("state.changed", state=self.state)

    def add_scan_root(self, root: Path) -> None:
        path = str(Path(root).expanduser().resolve())
        if path not in self.state.scan_roots:
            self.state.scan_roots.append(path)
            self.state.scan_roots = self._normalize_scan_roots(self.state.scan_roots)
            self.persist()
            self.log("info", "todo.file.detected", f"新增巡检目录：{path}")
            self.bus.emit("state.changed", state=self.state)

    def remove_scan_root(self, root: Path) -> None:
        path = str(Path(root).expanduser().resolve())
        if path in self.state.scan_roots and len(self.state.scan_roots) > 1:
            self.state.scan_roots = [item for item in self.state.scan_roots if item != path]
            self.persist()
            self.log("info", "todo.file.changed", f"移除巡检目录：{path}")
            self.bus.emit("state.changed", state=self.state)

    def scan_todos(self, initial: bool = False) -> Any:
        roots = self._normalize_scan_roots(self.state.scan_roots or [str(self.workspace.tasks_root)])
        self.state.scan_roots = roots
        outcome = scan_markdown_tasks(
            roots,
            workspace_root=self.workspace.workspace_root,
            known_mtimes=self.state.task_mtimes,
        )
        self.state.todo_items = outcome.items
        self.state.task_mtimes = outcome.task_index
        self.state.last_scan_summary = outcome.summary
        self.state.last_scan_at = now_iso()
        if outcome.files_scanned:
            self.log("info", "todo.scan.started", "开始巡检 Markdown 任务文件")
        for changed in outcome.changed_files:
            self.log("info", "todo.file.changed", f"任务文件已变更：{changed}")
        self.log(
            "info",
            "todo.tasks.parsed",
            outcome.summary,
            details={"pending": outcome.pending_count, "completed": outcome.completed_count},
        )
        self.persist()
        self.bus.emit("todo.changed", outcome=outcome, initial=initial)
        return outcome

    def _start_task(
        self,
        kind: str,
        title: str,
        source: str = "",
        steps: list[str] | None = None,
        result_builder: Callable[[TaskRecord, AppState], str] | None = None,
        details: dict[str, object] | None = None,
        replace_existing: bool = True,
    ) -> TaskRecord:
        if replace_existing and self.state.active_task and self.state.active_task.status in {"running", "paused"}:
            self.terminate_active_task("被新任务替换")
        task = TaskRecord(
            id=new_id("task"),
            kind=kind,
            title=title,
            source=source,
            status="running",
            steps=steps or ["分析输入", "整理结果", "写入输出"],
            current_step_index=0,
            details=details or {},
        )
        task.timeline.append(f"{now_iso()} | 已创建任务：{title}")
        task.timeline.append(f"{now_iso()} | 当前步骤：{task.current_step_label}")
        self.state.active_task = task
        self.state.task_history.append(task)
        self.state.task_history = self.state.task_history[-40:]
        self._task_builder = result_builder
        self.log("info", f"{kind}.created", f"任务已创建：{title}", task_id=task.id, details=details or {})
        self.persist()
        self.bus.emit("task.changed", task=task)
        self._schedule_task_step()
        return task

    def _schedule_task_step(self) -> None:
        self._cancel_task_timer()
        self._task_timer = self.schedule(self._step_delay_ms, self._advance_task)

    def _cancel_task_timer(self) -> None:
        if self._task_timer is not None:
            try:
                self.cancel(self._task_timer)
            except Exception:
                pass
            self._task_timer = None

    def _advance_task(self) -> None:
        task = self.state.active_task
        if not task or task.status != "running":
            return
        if task.current_step_index < len(task.steps):
            step = task.steps[task.current_step_index]
            task.timeline.append(f"{now_iso()} | 执行步骤：{step}")
            self.log("info", "tool.call", f"{task.title}：{step}", task_id=task.id, details={"step": step})
            task.current_step_index += 1
            task.updated_at = now_iso()
            self.persist()
            self.bus.emit("task.changed", task=task)
        if task.current_step_index >= len(task.steps):
            self._complete_task()
        else:
            self._schedule_task_step()

    def _complete_task(self) -> None:
        task = self.state.active_task
        if not task or task.status != "running":
            return
        result_text = ""
        if self._task_builder is not None:
            try:
                result_text = str(self._task_builder(task, self.state))
            except Exception as exc:
                task.status = "failed"
                task.result = f"任务生成失败：{exc}"
                task.updated_at = now_iso()
                task.timeline.append(f"{now_iso()} | 失败：{exc}")
                self.log("error", f"{task.kind}.failed", task.result, task_id=task.id, details={"error": str(exc)})
                self.persist()
                self.bus.emit("task.changed", task=task)
                return
        task.status = "completed"
        task.result = result_text
        task.summary = short_text(result_text or task.title, 120)
        task.updated_at = now_iso()
        task.timeline.append(f"{now_iso()} | 任务完成")
        self.log(
            "info",
            f"{task.kind}.completed",
            f"任务完成：{task.title}",
            task_id=task.id,
            details={"result": short_text(result_text, 180)},
        )
        self.persist()
        self.bus.emit("task.changed", task=task)

    def pause_active_task(self) -> None:
        task = self.state.active_task
        if not task or task.status != "running":
            return
        self.log("warn", "task.pause.requested", f"请求暂停任务：{task.title}", task_id=task.id)
        self._cancel_task_timer()
        task.status = "paused"
        task.updated_at = now_iso()
        task.timeline.append(f"{now_iso()} | 已暂停")
        self.log("warn", "task.pause.completed", f"任务已暂停：{task.title}", task_id=task.id)
        self.persist()
        self.bus.emit("task.changed", task=task)

    def resume_active_task(self) -> None:
        task = self.state.active_task
        if not task or task.status != "paused":
            return
        self.log("info", "task.resume.requested", f"请求恢复任务：{task.title}", task_id=task.id)
        task.status = "running"
        task.updated_at = now_iso()
        task.timeline.append(f"{now_iso()} | 已恢复")
        self.log("info", "task.resume.completed", f"任务已恢复：{task.title}", task_id=task.id)
        self.persist()
        self.bus.emit("task.changed", task=task)
        self._schedule_task_step()

    def terminate_active_task(self, reason: str = "用户终止") -> None:
        task = self.state.active_task
        if not task or task.status in {"terminated", "completed", "failed"}:
            return
        self.log("warn", "task.terminate.requested", f"请求终止任务：{task.title}", task_id=task.id)
        self._cancel_task_timer()
        task.status = "terminated"
        task.updated_at = now_iso()
        task.timeline.append(f"{now_iso()} | 已终止：{reason}")
        task.result = task.result or f"任务被终止：{reason}"
        self.log("warn", "task.terminate.completed", f"任务已终止：{task.title}", task_id=task.id)
        self.persist()
        self.bus.emit("task.changed", task=task)

    def start_boot_task(self) -> TaskRecord:
        return self._start_task(
            "boot",
            "启动检查",
            steps=["加载工作区", "扫描待办", "准备入口"],
            result_builder=lambda task, state: "原型已完成启动检查，桌面入口和数据层已就绪。",
            details={"source": "startup"},
        )

    def _normalize_action(self, action: str) -> str:
        raw = action.strip().lower()
        aliases = {
            "总结": "summary",
            "summary": "summary",
            "翻译": "translate",
            "translate": "translate",
            "解释": "explain",
            "explain": "explain",
            "下一步": "next",
            "next": "next",
            "排查问题": "troubleshoot",
            "troubleshoot": "troubleshoot",
            "问题排查": "troubleshoot",
        }
        return aliases.get(action, aliases.get(raw, raw))

    def start_clipboard_action(self, action: str) -> TaskRecord | None:
        text = self.state.clipboard_text.strip()
        if not text:
            self.log("warn", "clipboard.empty", "当前剪贴板没有可处理的文本")
            return None
        action_key = self._normalize_action(action)
        mapping = {
            "summary": ("总结当前内容", "task.content_summary.created", self.assistant.summarize),
            "translate": ("翻译复制内容", "task.translation.created", self.assistant.translate),
            "explain": ("解释当前内容", "task.explanation.created", self.assistant.explain),
            "next": ("问下一步做什么", "task.next_step.created", self.assistant.next_steps),
            "troubleshoot": ("排查问题", "task.troubleshooting.created", self.assistant.troubleshoot),
        }
        if action_key not in mapping:
            self.log("warn", "clipboard.unknown_action", f"未知的快捷动作：{action}")
            return None
        title, event_name, builder = mapping[action_key]
        self.log("info", event_name, f"开始处理：{title}")
        return self._start_task(
            action_key,
            title,
            source=text,
            steps=["识别上下文", "生成草稿", "输出结果"],
            result_builder=lambda task, state: builder(text),
            details={"source": "clipboard", "action": action_key},
        )

    def start_video_summary(self, url: str, approval_id: str = "") -> TaskRecord:
        title = "在线视频总结"
        self.log("info", "video.summary.started", f"启动视频总结：{url}")
        return self._start_task(
            "video",
            title,
            source=url,
            steps=["校验链接", "整理章节", "生成摘要", "输出结构化结果"],
            result_builder=lambda task, state: self.assistant.video_summary(url),
            details={"url": url, "approval_id": approval_id},
        )

    def generate_memory_snapshot(self) -> TaskRecord:
        self.log("info", "memory.extraction.started", "开始生成镜子记忆")

        def build(task: TaskRecord, state: AppState) -> str:
            summary_md, user_md = format_memory_summary(state)
            summary_path = self.workspace.write_markdown("memory/summary.md", summary_md)
            user_path = self.workspace.write_markdown("memory/USER.md", user_md)
            self.workspace.write_markdown(
                "memory/memory.json",
                json.dumps(
                    {
                        "summary_path": str(summary_path),
                        "user_path": str(user_path),
                        "session": state.session.session_id,
                        "workspace": state.workspace_root,
                        "generated_at": now_iso(),
                    },
                    ensure_ascii=False,
                    indent=2,
                )
                + "\n",
            )
            return "\n".join(
                [
                    "镜子记忆已生成",
                    f"- summary.md: {summary_path}",
                    f"- USER.md: {user_path}",
                    "- 说明：文件已写入本地工作区的 memory 目录。",
                ]
            )

        return self._start_task(
            "memory",
            "生成镜子记忆",
            steps=["提炼近期上下文", "生成 summary.md", "生成 USER.md"],
            result_builder=build,
            details={"workspace": self.state.workspace_root},
        )

    def clear_memory(self) -> None:
        removed = self.workspace.clear_memory_files()
        self.log("info", "memory.cleared", f"已删除 {len(removed)} 个记忆文件")
        self.bus.emit("memory.changed", removed=removed)

    def request_command_demo(self, command: str) -> ApprovalRequest | None:
        command = command.strip() or self.state.command_text or "dir"
        if not command:
            return None
        self.state.command_text = command
        approval = self.request_approval(
            action="command_demo",
            risk="yellow",
            description=f"是否记录命令演示：{short_text(command, 120)}",
            payload={"command": command},
        )
        self.persist()
        self.bus.emit("approval.changed", approval=approval)
        return approval

    def request_approval(
        self,
        action: str,
        risk: str,
        description: str,
        payload: dict[str, object] | None = None,
        task_id: str = "",
    ) -> ApprovalRequest:
        approval = ApprovalRequest(
            id=new_id("approval"),
            task_id=task_id,
            action=action,
            risk=risk,
            description=description,
            payload=payload or {},
        )
        self.state.approvals.append(approval)
        self.state.approvals = self.state.approvals[-60:]
        self.log("warn" if risk != "green" else "info", "approval.requested", description, task_id=task_id, details=approval.payload)
        self.persist()
        self.bus.emit("approval.changed", approval=approval)
        return approval

    def respond_to_approval(self, approval_id: str, accepted: bool) -> ApprovalRequest | None:
        approval = next((item for item in self.state.approvals if item.id == approval_id), None)
        if approval is None:
            return None
        approval.status = "approved" if accepted else "rejected"
        approval.decided_at = now_iso()
        approval.result_text = "已批准" if accepted else "已拒绝"
        event_name = "approval.approved" if accepted else "approval.rejected"
        self.log(
            "info" if accepted else "warn",
            event_name,
            f"{approval.result_text}：{approval.description}",
            task_id=approval.task_id,
            details=approval.payload,
        )
        self.persist()
        self.bus.emit("approval.changed", approval=approval)
        if accepted:
            if approval.action == "video_summary":
                self.state.pending_video_approval_id = ""
                self.state.pending_video_url = ""
                self.start_video_summary(str(approval.payload.get("url") or approval.description), approval_id=approval.id)
            elif approval.action == "command_demo":
                command = str(approval.payload.get("command") or self.state.command_text or "dir")
                self.start_command_demo_task(command)
        else:
            if approval.action == "video_summary":
                self.state.pending_video_approval_id = ""
                self.state.pending_video_url = ""
        self.persist()
        self.bus.emit("task.changed", task=self.state.active_task)
        return approval

    def start_command_demo_task(self, command: str) -> TaskRecord:
        def build(task: TaskRecord, state: AppState) -> str:
            return "\n".join(
                [
                    "命令演示",
                    f"- 工作区：{state.workspace_root}",
                    f"- 命令：{command}",
                    "- 说明：当前原型只记录命令审批和执行意图，不会自动执行危险命令。",
                    "- 后续可以把这里替换为轻沙盒执行层。",
                ]
            )

        return self._start_task(
            "command",
            "命令演示",
            source=command,
            steps=["确认工作区", "记录命令", "输出执行记录"],
            result_builder=build,
            details={"command": command, "workspace": self.state.workspace_root},
        )

    def analyze_clipboard(self, text: str) -> ClipboardAnalysis:
        raw = (text or "").strip()
        if not raw:
            return ClipboardAnalysis(kind="empty", text="", note="剪贴板为空", actions=["总结", "翻译", "解释", "下一步"])
        if is_video_url(raw):
            url = video_url_from_text(raw)
            return ClipboardAnalysis(
                kind="video",
                text=raw,
                url=url,
                note=f"检测到视频链接：{short_text(url, 120)}",
                actions=["开始视频总结", "忽略"],
            )
        if extract_urls(raw):
            return ClipboardAnalysis(
                kind="url",
                text=raw,
                url=video_url_from_text(raw),
                note="检测到普通链接，可打开或摘要。",
                actions=["总结", "翻译", "解释", "下一步"],
            )
        if is_error_text(raw):
            return ClipboardAnalysis(
                kind="error",
                text=raw,
                note="检测到错误文本，可直接排查。",
                actions=["排查问题", "解释", "总结", "下一步"],
            )
        return ClipboardAnalysis(
            kind="text",
            text=raw,
            note="检测到可处理的剪贴板文本。",
            actions=["总结", "翻译", "解释", "下一步"],
        )

    def handle_clipboard_text(self, text: str) -> ClipboardAnalysis:
        analysis = self.analyze_clipboard(text)
        self.state.clipboard_text = analysis.text
        self.state.clipboard_kind = analysis.kind
        self.state.clipboard_hint = analysis.note
        self.state.clipboard_actions = analysis.actions
        self.state.pending_video_url = analysis.url if analysis.kind == "video" else ""
        if analysis.kind == "video":
            approval = self.request_approval(
                action="video_summary",
                risk="yellow",
                description=f"检测到视频链接，是否生成视频总结？{short_text(analysis.url, 100)}",
                payload={"url": analysis.url},
            )
            self.state.pending_video_approval_id = approval.id
            self.log("info", "video.link.detected", analysis.note, details={"url": analysis.url})
        elif analysis.kind == "error":
            self.log("warn", "problem.error_text.detected", analysis.note)
        elif analysis.kind == "url":
            self.log("info", "clipboard.url.detected", analysis.note, details={"url": analysis.url})
        else:
            self.log("info", "clipboard.text.captured", analysis.note, details={"length": len(analysis.text)})
        self.persist()
        self.bus.emit("clipboard.changed", analysis=analysis, state=self.state)
        return analysis

    def generate_chat_reply(self, message: str) -> str:
        return self.assistant.reply(message)

    # Compatibility helper for UI
    def request_video_summary(self, url: str) -> ApprovalRequest:
        approval = self.request_approval(
            action="video_summary",
            risk="yellow",
            description=f"检测到视频链接，是否生成视频总结？{short_text(url, 100)}",
            payload={"url": url},
        )
        self.state.pending_video_url = url
        self.state.pending_video_approval_id = approval.id
        self.persist()
        self.bus.emit("approval.changed", approval=approval)
        return approval

