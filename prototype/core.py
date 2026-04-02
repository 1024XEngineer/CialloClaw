from __future__ import annotations

import json
import os
import re
import subprocess
import sys
import textwrap
import uuid
from collections import Counter
from dataclasses import asdict, dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any, Callable
from urllib.parse import parse_qs, urlparse


def now_iso() -> str:
    return datetime.now().astimezone().isoformat(timespec="seconds")


def new_id(prefix: str) -> str:
    return f"{prefix}_{uuid.uuid4().hex[:8]}"


def short_text(text: str, width: int = 140) -> str:
    clean = " ".join((text or "").split())
    return textwrap.shorten(clean, width=width, placeholder="...")


def contains_cjk(text: str) -> bool:
    return bool(re.search(r"[\u4e00-\u9fff]", text))


def split_sentences(text: str) -> list[str]:
    parts = re.split(r"(?<=[。！？!?\.])\s+|\n+", text or "")
    return [part.strip() for part in parts if part.strip()]


def extract_urls(text: str) -> list[str]:
    urls = re.findall(r"https?://[^\s<>\"]+", text or "")
    return [u.rstrip(").,;]") for u in urls]


def is_error_text(text: str) -> bool:
    low = (text or "").lower()
    patterns = [
        "traceback",
        "exception",
        "error",
        "failed",
        "panic",
        "错误",
        "异常",
        "失败",
        "报错",
    ]
    return any(p in low for p in patterns)


VIDEO_HOST_HINTS = (
    "youtube.com",
    "youtu.be",
    "bilibili.com/video",
    "b23.tv",
    "vimeo.com",
    "dailymotion.com",
    "twitch.tv/videos",
)


def normalize_url(url: str) -> str:
    return url.strip().rstrip(").,;]")


def is_video_url(text: str) -> bool:
    urls = extract_urls(text) or [text.strip()]
    for raw in urls:
        lower = normalize_url(raw).lower()
        if any(hint in lower for hint in VIDEO_HOST_HINTS):
            return True
        if "watch?v=" in lower or "/video/" in lower or "video" in lower:
            return True
    return False


def video_url_from_text(text: str) -> str:
    urls = extract_urls(text)
    return normalize_url(urls[0]) if urls else normalize_url(text)


@dataclass
class LogEntry:
    timestamp: str
    level: str
    category: str
    message: str
    task_id: str = ""
    details: dict[str, Any] = field(default_factory=dict)


@dataclass
class SessionState:
    session_id: str
    started_at: str
    status: str = "running"
    focus_mode: str = "assistant"


@dataclass
class TaskRecord:
    id: str
    kind: str
    title: str
    source: str = ""
    status: str = "running"
    created_at: str = field(default_factory=now_iso)
    updated_at: str = field(default_factory=now_iso)
    steps: list[str] = field(default_factory=list)
    current_step_index: int = 0
    summary: str = ""
    result: str = ""
    timeline: list[str] = field(default_factory=list)
    details: dict[str, Any] = field(default_factory=dict)

    @property
    def current_step_label(self) -> str:
        if not self.steps:
            return "等待中"
        index = min(max(self.current_step_index, 0), len(self.steps) - 1)
        if self.status in {"completed", "terminated", "failed"} and self.current_step_index >= len(self.steps):
            return "已完成"
        return self.steps[index]


@dataclass
class ApprovalRequest:
    id: str
    task_id: str
    action: str
    risk: str
    description: str
    status: str = "pending"
    requested_at: str = field(default_factory=now_iso)
    decided_at: str = ""
    payload: dict[str, Any] = field(default_factory=dict)
    result_text: str = ""


@dataclass
class TodoItem:
    path: str
    relative_path: str
    line_no: int
    checked: bool
    text: str
    mtime: float = 0.0


@dataclass
class ClipboardAnalysis:
    kind: str
    text: str
    url: str = ""
    note: str = ""
    actions: list[str] = field(default_factory=list)


@dataclass
class ScanOutcome:
    items: list[TodoItem]
    files_scanned: int
    changed_files: list[str]
    pending_count: int
    completed_count: int
    summary: str
    task_index: dict[str, float]


@dataclass
class AppState:
    session: SessionState = field(
        default_factory=lambda: SessionState(
            session_id=new_id("session"),
            started_at=now_iso(),
        )
    )
    workspace_root: str = ""
    scan_roots: list[str] = field(default_factory=list)
    memory_enabled: bool = True
    scan_interval_seconds: int = 8
    auto_scan: bool = True
    show_floating_ball: bool = True
    clipboard_text: str = ""
    clipboard_kind: str = "idle"
    clipboard_hint: str = ""
    clipboard_actions: list[str] = field(default_factory=list)
    pending_video_url: str = ""
    pending_video_approval_id: str = ""
    last_scan_summary: str = ""
    last_scan_at: str = ""
    command_text: str = "dir"
    active_task: TaskRecord | None = None
    task_history: list[TaskRecord] = field(default_factory=list)
    approvals: list[ApprovalRequest] = field(default_factory=list)
    logs: list[LogEntry] = field(default_factory=list)
    todo_items: list[TodoItem] = field(default_factory=list)
    task_mtimes: dict[str, float] = field(default_factory=dict)


def _load_session(data: dict[str, Any]) -> SessionState:
    return SessionState(
        session_id=str(data.get("session_id") or new_id("session")),
        started_at=str(data.get("started_at") or now_iso()),
        status=str(data.get("status") or "running"),
        focus_mode=str(data.get("focus_mode") or "assistant"),
    )


def _load_task(data: dict[str, Any]) -> TaskRecord:
    return TaskRecord(
        id=str(data.get("id") or new_id("task")),
        kind=str(data.get("kind") or "unknown"),
        title=str(data.get("title") or "Untitled"),
        source=str(data.get("source") or ""),
        status=str(data.get("status") or "running"),
        created_at=str(data.get("created_at") or now_iso()),
        updated_at=str(data.get("updated_at") or now_iso()),
        steps=list(data.get("steps") or []),
        current_step_index=int(data.get("current_step_index") or 0),
        summary=str(data.get("summary") or ""),
        result=str(data.get("result") or ""),
        timeline=list(data.get("timeline") or []),
        details=dict(data.get("details") or {}),
    )


def _load_approval(data: dict[str, Any]) -> ApprovalRequest:
    return ApprovalRequest(
        id=str(data.get("id") or new_id("approval")),
        task_id=str(data.get("task_id") or ""),
        action=str(data.get("action") or ""),
        risk=str(data.get("risk") or "yellow"),
        description=str(data.get("description") or ""),
        status=str(data.get("status") or "pending"),
        requested_at=str(data.get("requested_at") or now_iso()),
        decided_at=str(data.get("decided_at") or ""),
        payload=dict(data.get("payload") or {}),
        result_text=str(data.get("result_text") or ""),
    )


def _load_todo(data: dict[str, Any]) -> TodoItem:
    return TodoItem(
        path=str(data.get("path") or ""),
        relative_path=str(data.get("relative_path") or ""),
        line_no=int(data.get("line_no") or 0),
        checked=bool(data.get("checked") or False),
        text=str(data.get("text") or ""),
        mtime=float(data.get("mtime") or 0.0),
    )


def _load_log(data: dict[str, Any]) -> LogEntry:
    return LogEntry(
        timestamp=str(data.get("timestamp") or now_iso()),
        level=str(data.get("level") or "info"),
        category=str(data.get("category") or "misc"),
        message=str(data.get("message") or ""),
        task_id=str(data.get("task_id") or ""),
        details=dict(data.get("details") or {}),
    )


def state_from_dict(data: dict[str, Any], default_state: AppState) -> AppState:
    session_data = data.get("session") or {}
    active_task_data = data.get("active_task")
    task_history_data = data.get("task_history") or []
    approvals_data = data.get("approvals") or []
    logs_data = data.get("logs") or []
    todo_items_data = data.get("todo_items") or []
    return AppState(
        session=_load_session(session_data),
        workspace_root=str(data.get("workspace_root") or default_state.workspace_root),
        scan_roots=list(data.get("scan_roots") or default_state.scan_roots),
        memory_enabled=bool(data.get("memory_enabled", default_state.memory_enabled)),
        scan_interval_seconds=int(data.get("scan_interval_seconds", default_state.scan_interval_seconds)),
        auto_scan=bool(data.get("auto_scan", default_state.auto_scan)),
        show_floating_ball=bool(data.get("show_floating_ball", default_state.show_floating_ball)),
        clipboard_text=str(data.get("clipboard_text") or ""),
        clipboard_kind=str(data.get("clipboard_kind") or "idle"),
        clipboard_hint=str(data.get("clipboard_hint") or ""),
        clipboard_actions=list(data.get("clipboard_actions") or []),
        pending_video_url=str(data.get("pending_video_url") or ""),
        pending_video_approval_id=str(data.get("pending_video_approval_id") or ""),
        last_scan_summary=str(data.get("last_scan_summary") or ""),
        last_scan_at=str(data.get("last_scan_at") or ""),
        command_text=str(data.get("command_text") or "dir"),
        active_task=_load_task(active_task_data) if isinstance(active_task_data, dict) else None,
        task_history=[_load_task(item) for item in task_history_data if isinstance(item, dict)],
        approvals=[_load_approval(item) for item in approvals_data if isinstance(item, dict)],
        logs=[_load_log(item) for item in logs_data if isinstance(item, dict)],
        todo_items=[_load_todo(item) for item in todo_items_data if isinstance(item, dict)],
        task_mtimes={str(k): float(v) for k, v in dict(data.get("task_mtimes") or {}).items()},
    )


def state_to_dict(state: AppState) -> dict[str, Any]:
    return asdict(state)


class EventBus:
    def __init__(self) -> None:
        self._listeners: dict[str, list[Callable[[str, dict[str, Any]], None]]] = {}

    def on(self, event: str, handler: Callable[[str, dict[str, Any]], None]) -> None:
        self._listeners.setdefault(event, []).append(handler)

    def emit(self, event: str, **payload: Any) -> None:
        handlers = list(self._listeners.get(event, [])) + list(self._listeners.get("*", []))
        for handler in handlers:
            try:
                handler(event, payload)
            except Exception as exc:  # pragma: no cover
                print(f"[event-bus] {event} handler failed: {exc}", file=sys.stderr)


class WorkspaceManager:
    def __init__(self, project_root: Path, workspace_root: Path | None = None) -> None:
        self.project_root = Path(project_root).resolve()
        self.workspace_root = Path(workspace_root or (self.project_root / "workspace")).resolve()
        self._sync_paths()

    def _sync_paths(self) -> None:
        self.runtime_root = self.workspace_root / ".ciallo"
        self.memory_root = self.runtime_root / "memory"
        self.logs_root = self.runtime_root / "logs"
        self.tasks_root = self.workspace_root / "tasks"
        self.state_file = self.runtime_root / "state.json"
        self.log_file = self.logs_root / "events.jsonl"

    def set_workspace_root(self, workspace_root: Path) -> None:
        self.workspace_root = Path(workspace_root).resolve()
        self._sync_paths()

    def ensure_layout(self) -> None:
        self.workspace_root.mkdir(parents=True, exist_ok=True)
        self.runtime_root.mkdir(parents=True, exist_ok=True)
        self.memory_root.mkdir(parents=True, exist_ok=True)
        self.logs_root.mkdir(parents=True, exist_ok=True)
        self.tasks_root.mkdir(parents=True, exist_ok=True)

    def default_state(self) -> AppState:
        self.ensure_layout()
        return AppState(
            workspace_root=str(self.workspace_root),
            scan_roots=[str(self.tasks_root)],
            memory_enabled=True,
            scan_interval_seconds=8,
            auto_scan=True,
            show_floating_ball=True,
            command_text="dir",
        )

    def load_state(self) -> AppState:
        default = self.default_state()
        if not self.state_file.exists():
            return default
        try:
            data = json.loads(self.state_file.read_text(encoding="utf-8"))
        except Exception:
            return default
        return state_from_dict(data, default)

    def save_state(self, state: AppState) -> None:
        self.ensure_layout()
        self.state_file.write_text(json.dumps(state_to_dict(state), ensure_ascii=False, indent=2), encoding="utf-8")

    def append_log(self, entry: LogEntry) -> None:
        self.ensure_layout()
        with self.log_file.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps(asdict(entry), ensure_ascii=False) + "\n")

    def write_markdown(self, relative_path: str | Path, content: str) -> Path:
        self.ensure_layout()
        target = self.workspace_root / Path(relative_path)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content, encoding="utf-8")
        return target

    def open_path(self, path: Path) -> None:
        target = Path(path)
        if not target.exists():
            target.mkdir(parents=True, exist_ok=True)
        if hasattr(os, "startfile"):
            os.startfile(str(target))  # type: ignore[attr-defined]
            return
        subprocess.Popen(["explorer", str(target)])

    def ensure_demo_content(self, force: bool = False) -> Path:
        self.ensure_layout()
        sample = self.tasks_root / "sample_tasks.md"
        if force or not any(self.tasks_root.rglob("*.md")):
            sample.write_text(
                "\n".join(
                    [
                        "# Demo tasks",
                        "",
                        "- [ ] 整理今天的需求反馈",
                        "- [ ] 检查复制内容触发是否正常",
                        "- [x] 准备待办巡检样例",
                    ]
                )
                + "\n",
                encoding="utf-8",
            )
        return sample

    def clear_memory_files(self) -> list[Path]:
        removed: list[Path] = []
        for target in (self.memory_root / "summary.md", self.memory_root / "USER.md", self.memory_root / "memory.json"):
            if target.exists():
                target.unlink()
                removed.append(target)
        return removed


TASK_LINE = re.compile(r"^\s*-\s*\[(?P<mark>[ xX])\]\s+(?P<text>.+?)\s*$")
SKIP_DIRS = {".git", ".idea", ".vscode", "__pycache__", "node_modules", "dist", "build", ".ciallo"}


def _relative_display(path: Path, workspace_root: Path | None) -> str:
    if workspace_root is None:
        return path.name
    try:
        return str(path.relative_to(workspace_root))
    except Exception:
        return str(path)


def scan_markdown_tasks(
    scan_roots: list[str] | list[Path],
    workspace_root: Path | None = None,
    known_mtimes: dict[str, float] | None = None,
) -> ScanOutcome:
    known_mtimes = known_mtimes or {}
    files_scanned = 0
    changed_files: list[str] = []
    items: list[TodoItem] = []
    task_index: dict[str, float] = {}
    seen: set[str] = set()

    for root_raw in scan_roots:
        root = Path(root_raw)
        if not root.exists():
            continue
        for path in root.rglob("*.md"):
            if any(part in SKIP_DIRS for part in path.parts):
                continue
            absolute = str(path.resolve())
            if absolute in seen:
                continue
            seen.add(absolute)
            files_scanned += 1
            try:
                mtime = path.stat().st_mtime
            except OSError:
                continue
            task_index[absolute] = mtime
            if abs(known_mtimes.get(absolute, -1.0) - mtime) > 0.0001:
                changed_files.append(_relative_display(path, workspace_root))
            try:
                text = path.read_text(encoding="utf-8")
            except UnicodeDecodeError:
                text = path.read_text(encoding="utf-8", errors="ignore")
            rel = _relative_display(path, workspace_root)
            for line_no, line in enumerate(text.splitlines(), start=1):
                match = TASK_LINE.match(line)
                if not match:
                    continue
                checked = match.group("mark").lower() == "x"
                items.append(
                    TodoItem(
                        path=absolute,
                        relative_path=rel,
                        line_no=line_no,
                        checked=checked,
                        text=match.group("text").strip(),
                        mtime=mtime,
                    )
                )

    items.sort(key=lambda item: (item.relative_path.lower(), item.line_no))
    pending_count = sum(not item.checked for item in items)
    completed_count = sum(item.checked for item in items)
    summary = (
        f"扫描 {files_scanned} 个 Markdown 文件，识别 {len(items)} 条待办，"
        f"未完成 {pending_count} 条，已完成 {completed_count} 条"
    )
    return ScanOutcome(
        items=items,
        files_scanned=files_scanned,
        changed_files=changed_files,
        pending_count=pending_count,
        completed_count=completed_count,
        summary=summary,
        task_index=task_index,
    )


class LocalAssistant:
    def __init__(self) -> None:
        self._glossary = {
            "第一阶段功能文档": "phase-1 functional spec",
            "控制面板": "dashboard",
            "悬浮球": "floating ball",
            "聊天弹层": "chat popup",
            "待办管家": "todo manager",
            "待办": "to-do",
            "任务": "task",
            "工作区": "workspace",
            "剪贴板": "clipboard",
            "视频总结": "video summary",
            "总结": "summary",
            "翻译": "translate",
            "解释": "explain",
            "下一步建议": "next-step suggestion",
            "下一步": "next step",
            "审批": "approval",
            "暂停": "pause",
            "恢复": "resume",
            "终止": "terminate",
            "日志": "log",
            "记忆": "memory",
            "本地": "local",
            "执行": "execute",
            "沙盒": "sandbox",
            "文件": "file",
            "目录": "folder",
            "扫描": "scan",
            "自动化": "automation",
            "提示": "prompt",
            "确认": "confirm",
            "消息": "message",
            "结果": "result",
            "摘要": "summary",
            "任务文件": "task file",
            "用户": "user",
            "总结当前内容": "summarize current content",
            "翻译复制内容": "translate copied content",
            "解释当前内容": "explain current content",
            "记录为待办": "turn into to-do",
            "问下一步做什么": "ask for the next step",
            "轻沙盒": "light sandbox",
            "工作区边界": "workspace boundary",
            "短期记忆": "short-term memory",
            "长期记忆": "long-term memory",
            "在线视频链接": "online video link",
        }
        self._stop_words = {
            "the",
            "and",
            "for",
            "with",
            "that",
            "this",
            "from",
            "into",
            "about",
            "your",
            "you",
            "are",
            "have",
            "has",
            "will",
            "can",
            "not",
            "should",
            "would",
            "could",
            "they",
            "their",
            "been",
            "there",
            "what",
            "when",
            "where",
            "which",
            "because",
            "while",
            "into",
            "over",
            "after",
            "before",
            "than",
            "then",
            "does",
            "done",
        }

    def _lines(self, text: str) -> list[str]:
        return [line.strip() for line in (text or "").splitlines() if line.strip()]

    def _keywords(self, text: str, limit: int = 5) -> list[str]:
        words = re.findall(r"[A-Za-z]{4,}", text.lower())
        if not words:
            return []
        counts = Counter(word for word in words if word not in self._stop_words)
        return [word for word, _ in counts.most_common(limit)]

    def _lead(self, text: str) -> str:
        lines = self._lines(text)
        if lines:
            return short_text(lines[0], 120)
        sentences = split_sentences(text)
        if sentences:
            return short_text(sentences[0], 120)
        return short_text(text, 120)

    def _translate_with_glossary(self, text: str) -> str:
        translated = text
        for source, target in sorted(self._glossary.items(), key=lambda item: len(item[0]), reverse=True):
            translated = translated.replace(source, target)
        translated = re.sub(r"\s+", " ", translated)
        return translated.strip()

    def summarize(self, text: str) -> str:
        if not text.strip():
            return "摘要\n- 当前没有可处理的内容。"
        bullets = [line[1:].strip() for line in self._lines(text) if line.startswith(("-", "•", "*"))]
        key_points = bullets[:4] if bullets else split_sentences(text)[:3]
        keywords = self._keywords(text)
        lines = [
            "摘要",
            f"- 核心：{self._lead(text)}",
        ]
        if keywords:
            lines.append(f"- 关键词：{', '.join(keywords)}")
        if key_points:
            for point in key_points[:4]:
                lines.append(f"- 重点：{short_text(point, 120)}")
        lines.append("- 建议：如果要更精确的结论，继续补充上下文或原文链接。")
        return "\n".join(lines)

    def translate(self, text: str) -> str:
        if not text.strip():
            return "翻译草稿\n- 当前没有可处理的内容。"
        if contains_cjk(text):
            translated = self._translate_with_glossary(text)
            return "\n".join(
                [
                    "翻译草稿（离线原型）",
                    "- 当前使用本地术语词表做结构化翻译，不是完整机器翻译。",
                    f"- 原文预览：{short_text(text, 180)}",
                    f"- 草稿：{short_text(translated, 220)}",
                ]
            )
        return "\n".join(
            [
                "Translation Draft",
                "- The source text looks like English.",
                f"- Preview: {short_text(text, 220)}",
                "- The prototype keeps translation lightweight and local.",
            ]
        )

    def explain(self, text: str) -> str:
        if not text.strip():
            return "解释\n- 当前没有可解释的内容。"
        lead = self._lead(text)
        keywords = self._keywords(text)
        lines = [
            "解释",
            f"- 这段内容主要在讲：{lead}",
            f"- 它的重点词：{', '.join(keywords) if keywords else '暂无明显英文关键词'}",
            "- 如果这是任务或方案，先确认目标、输入和约束。",
            "- 如果这是知识内容，先提炼概念、关系和结论。",
        ]
        return "\n".join(lines)

    def next_steps(self, text: str) -> str:
        lines = [
            "下一步建议",
            "- 先确认你真正想达成的结果。",
            "- 补齐缺失的上下文、样例或约束。",
            "- 把目标拆成一个最小可执行动作。",
            "- 完成后再决定是否继续扩展或自动化。",
        ]
        if text.strip():
            lines.insert(1, f"- 当前上下文：{short_text(text, 160)}")
        return "\n".join(lines)

    def troubleshoot(self, text: str) -> str:
        low = (text or "").lower()
        hints = []
        if "traceback" in low:
            hints.append("这是 Python 回溯，优先看最后一行异常类型和触发位置。")
        if "exception" in low or "error" in low:
            hints.append("这是通用错误信息，先确认输入参数、权限和依赖。")
        if "failed" in low:
            hints.append("失败通常意味着前置条件没满足，检查最近一次变更。")
        if not hints:
            hints.append("先判断是输入错误、环境问题，还是业务逻辑问题。")
        return "\n".join(
            [
                "问题排查",
                f"- 错误预览：{short_text(text, 180)}",
                *[f"- {hint}" for hint in hints],
                "- 建议：先复现，再定位，再修复，最后补一个最小回归测试。",
            ]
        )

    def video_summary(self, url: str) -> str:
        parsed = urlparse(url)
        host = parsed.netloc or "unknown-host"
        video_id = parse_qs(parsed.query).get("v", [""])[0]
        slug = Path(parsed.path).name or "video"
        title_hint = video_id or slug or host
        return "\n".join(
            [
                "视频摘要",
                f"- 来源：{host}",
                f"- 主题推断：{title_hint}",
                "- 说明：当前原型未抓取真实视频正文，这里输出结构化总结草稿。",
                "- 关键点：开场背景、主要观点、示例论据、结论与行动建议。",
                "- 分章节：引入 / 展开 / 结论 / 下一步。",
                "- 可选问答：如果你愿意，我可以继续把某一部分展开成问答笔记。",
            ]
        )

    def reply(self, text: str) -> str:
        raw = (text or "").strip()
        if not raw:
            return "我已准备好。你可以复制一段文本，然后点总结、翻译、解释或下一步。"
        if is_error_text(raw):
            return self.troubleshoot(raw)
        if is_video_url(raw):
            return self.video_summary(video_url_from_text(raw))
        if contains_cjk(raw):
            if "翻译" in raw:
                return self.translate(raw)
            if "解释" in raw:
                return self.explain(raw)
            if "下一步" in raw:
                return self.next_steps(raw)
            return self.summarize(raw)
        if "summary" in raw.lower():
            return self.summarize(raw)
        if "translate" in raw.lower():
            return self.translate(raw)
        return self.next_steps(raw)


def format_memory_summary(state: AppState) -> tuple[str, str]:
    active = state.active_task
    recent_logs = state.logs[-12:]
    todos_pending = [item for item in state.todo_items if not item.checked]
    todos_done = [item for item in state.todo_items if item.checked]
    summary_lines = [
        "# summary",
        "",
        f"- Session: {state.session.session_id}",
        f"- Workspace: {state.workspace_root}",
        f"- Memory enabled: {'yes' if state.memory_enabled else 'no'}",
        f"- Scan interval: {state.scan_interval_seconds}s",
        f"- Clipboard kind: {state.clipboard_kind}",
        f"- Last scan: {state.last_scan_at or 'never'}",
        "",
        "## Active Task",
        f"- Title: {active.title if active else 'none'}",
        f"- Status: {active.status if active else 'idle'}",
        f"- Step: {active.current_step_label if active else 'n/a'}",
        "",
        "## Todo Snapshot",
        f"- Pending: {len(todos_pending)}",
        f"- Completed: {len(todos_done)}",
        "",
        "## Recent Logs",
    ]
    for log in recent_logs:
        summary_lines.append(f"- [{log.level}] {log.category}: {short_text(log.message, 100)}")

    user_lines = [
        "# USER",
        "",
        f"- workspace_root: {state.workspace_root}",
        f"- scan_roots: {', '.join(state.scan_roots) if state.scan_roots else '[]'}",
        f"- memory_enabled: {state.memory_enabled}",
        f"- auto_scan: {state.auto_scan}",
        f"- scan_interval_seconds: {state.scan_interval_seconds}",
        f"- show_floating_ball: {state.show_floating_ball}",
        f"- command_text: {state.command_text}",
        "",
        "## Frequent Actions",
    ]
    counts = Counter(task.kind for task in state.task_history)
    for kind, count in counts.most_common(8):
        user_lines.append(f"- {kind}: {count}")
    if not counts:
        user_lines.append("- none yet")
    return "\n".join(summary_lines).strip() + "\n", "\n".join(user_lines).strip() + "\n"

