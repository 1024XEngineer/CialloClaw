import { useMemo, useRef, useState } from "react";
import { useEffect } from "react";
import type { CSSProperties } from "react";
import { Link, NavLink, useNavigate } from "react-router-dom";
import { useMutation, useQuery } from "@tanstack/react-query";
import { ArrowLeft, CircleDashed, NotebookPen } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { dashboardModules } from "@/features/dashboard/shared/dashboardRoutes";
import { cn } from "@/utils/cn";
import { buildNoteSummary, describeNotePreview, getNoteBucketLabel, groupClosedNotes, sortClosedNotes, sortNotesByUrgency } from "./notePage.mapper";
import { convertNoteToTask, loadNoteBuckets } from "./notePage.service";
import type { NoteDetailAction, NoteListItem } from "./notePage.types";
import { NoteDetailPanel } from "./components/NoteDetailPanel";
import { NoteEmptyState } from "./components/NoteEmptyState";
import { NotePreviewCard } from "./components/NotePreviewCard";
import { NotePreviewSection } from "./components/NotePreviewSection";
import "./notePage.css";

export function NotePage() {
  const navigate = useNavigate();
  const [selectedItemId, setSelectedItemId] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [showMoreClosed, setShowMoreClosed] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const feedbackTimeoutRef = useRef<number | null>(null);

  const notesQuery = useQuery({
    queryKey: ["dashboard", "notes", "buckets"],
    queryFn: loadNoteBuckets,
  });

  const upcomingItems = sortNotesByUrgency(notesQuery.data?.upcoming ?? []);
  const laterItems = sortNotesByUrgency(notesQuery.data?.later ?? []);
  const recurringItems = sortNotesByUrgency(notesQuery.data?.recurring_rule ?? []);
  const closedItems = sortClosedNotes(notesQuery.data?.closed ?? []);
  const closedGroups = useMemo(() => groupClosedNotes(closedItems, showMoreClosed), [closedItems, showMoreClosed]);
  const summary = useMemo(() => buildNoteSummary({ recurring_rule: recurringItems, upcoming: upcomingItems }), [recurringItems, upcomingItems]);
  const selectedItem = useMemo(() => {
    const allItems = [...upcomingItems, ...laterItems, ...recurringItems, ...closedItems];
    return allItems.find((entry) => entry.item.item_id === selectedItemId) ?? upcomingItems[0] ?? laterItems[0] ?? recurringItems[0] ?? closedItems[0] ?? null;
  }, [closedItems, laterItems, recurringItems, selectedItemId, upcomingItems]);

  const pageStyle = {
    "--note-accent": "#F4B183",
    "--note-accent-glow": "rgba(244, 177, 131, 0.2)",
    "--note-accent-soft": "rgba(247, 225, 203, 0.68)",
    "--note-accent-surface": "rgba(250, 236, 220, 0.62)",
    "--note-accent-border": "rgba(244, 177, 131, 0.24)",
    "--note-accent-shadow": "rgba(166, 120, 86, 0.12)",
    "--note-paper": "rgba(255, 250, 244, 0.8)",
    "--note-paper-strong": "rgba(255, 247, 238, 0.9)",
    "--note-line": "rgba(156, 133, 113, 0.16)",
    "--note-ink": "#5f544b",
    "--note-copy": "rgba(95, 84, 75, 0.68)",
  } as CSSProperties;

  function showFeedback(message: string) {
    setFeedback(message);
    if (feedbackTimeoutRef.current) {
      window.clearTimeout(feedbackTimeoutRef.current);
    }
    feedbackTimeoutRef.current = window.setTimeout(() => setFeedback(null), 2400);
  }

  const convertMutation = useMutation({
    mutationFn: (itemId: string) => convertNoteToTask(itemId),
    onSuccess: (outcome) => {
      showFeedback("已为这条事项生成任务，正在跳转到任务页。");
      navigate("/tasks", { state: { focusTaskId: outcome.result.task.task_id, openDetail: true } });
    },
    onError: () => {
      showFeedback("转交给 Agent 失败，请稍后再试。");
    },
  });

  function handleDetailAction(action: NoteDetailAction) {
    if (!selectedItem) {
      return;
    }

    if (action === "convert-to-task") {
      convertMutation.mutate(selectedItem.item.item_id);
      return;
    }

    const placeholders: Record<Exclude<NoteDetailAction, "convert-to-task">, string> = {
      cancel: "取消本次事项的真实动作稍后接入。",
      "cancel-recurring": "取消整个重复事项的真实动作稍后接入。",
      complete: "标记完成的真实动作稍后接入。",
      delete: "删除记录的真实动作稍后接入。",
      edit: "编辑能力稍后接入。",
      "move-upcoming": "提前到近期要做的真实动作稍后接入。",
      "open-resource": "当前先展示相关资料入口，后续再接稳定的打开能力。",
      restore: "恢复为未完成的真实动作稍后接入。",
      "skip-once": "跳过本次的真实动作稍后接入。",
      "toggle-recurring": "重复规则开关的真实动作稍后接入。",
    };

    showFeedback(placeholders[action]);
  }

  useEffect(() => {
    const allItems = [...upcomingItems, ...laterItems, ...recurringItems, ...closedItems];
    if (allItems.length === 0) {
      return;
    }

    const selectedExists = selectedItemId ? allItems.some((entry) => entry.item.item_id === selectedItemId) : false;
    if (selectedExists) {
      return;
    }

    const nextItem = upcomingItems[0] ?? laterItems[0] ?? recurringItems[0] ?? closedItems[0];
    if (nextItem) {
      setSelectedItemId(nextItem.item.item_id);
    }
  }, [closedItems, laterItems, recurringItems, selectedItemId, upcomingItems]);

  useEffect(() => {
    return () => {
      if (feedbackTimeoutRef.current) {
        window.clearTimeout(feedbackTimeoutRef.current);
      }
    };
  }, []);

  if (notesQuery.isLoading && !notesQuery.data) {
    return (
      <main className="dashboard-page note-preview-page">
        <div className="note-preview-page__frame">
          <div className="note-preview-page__header note-preview-page__header--loading" />
          <div className="note-preview-page__grid note-preview-page__grid--loading" />
        </div>
      </main>
    );
  }

  if (!notesQuery.data || [upcomingItems, laterItems, recurringItems, closedItems].every((items) => items.length === 0)) {
    return (
      <main className="dashboard-page note-preview-page" style={pageStyle}>
        <div className="note-preview-page__frame">
          <NoteEmptyState />
        </div>
      </main>
    );
  }

  return (
    <main className="dashboard-page note-preview-page" style={pageStyle}>
      <header className="dashboard-page__topbar">
        <Link className="dashboard-page__home-link" to="/">
          <ArrowLeft className="h-4 w-4" />
          返回首页
        </Link>

        <nav aria-label="Dashboard modules" className="dashboard-page__module-nav">
          {dashboardModules.map((item) => (
            <NavLink key={item.route} className={({ isActive }) => cn("dashboard-page__module-link", isActive && "is-active")} to={item.path}>
              {item.title}
            </NavLink>
          ))}
        </nav>
      </header>

      <section className="dashboard-page__hero">
        <div className="dashboard-page__hero-copy">
          <p className="dashboard-page__eyebrow">Notepad Collaboration</p>
          <div className="dashboard-page__title-row">
            <NotebookPen className="dashboard-page__title-icon" />
            <h1>便签</h1>
          </div>
          <p className="dashboard-page__description">便签协作负责整理未来安排、重复规则与尚未开始但需要记住的事情。正式进入执行后，再转交给 Agent 生成任务。</p>
        </div>

        <div className="dashboard-card dashboard-card--status note-preview-page__hero-status">
          <p className="dashboard-card__kicker">今日摘要</p>
          <div className="note-preview-page__summary-grid">
            <div className="note-preview-page__summary-item">
              <span>今天待处理</span>
              <strong>{summary.dueToday}</strong>
            </div>
            <div className="note-preview-page__summary-item">
              <span>已逾期</span>
              <strong>{summary.overdue}</strong>
            </div>
            <div className="note-preview-page__summary-item">
              <span>重复事项今日落地</span>
              <strong>{summary.recurringToday}</strong>
            </div>
            <div className="note-preview-page__summary-item">
              <span>适合交给 Agent</span>
              <strong>{summary.readyForAgent}</strong>
            </div>
          </div>
          {selectedItem ? (
            <div className="dashboard-card__status-row">
              <CircleDashed className="h-4 w-4" />
              <span>{selectedItem.item.title} · {describeNotePreview(selectedItem.item, selectedItem.experience)}</span>
            </div>
          ) : null}
        </div>
      </section>

      <section className="dashboard-page__grid note-preview-page__grid">
        <NotePreviewSection
          activeItemId={selectedItem?.item.item_id ?? null}
          description="快到时间、今天要做、最近几天需要处理的事项。"
          items={upcomingItems}
          onSelect={(itemId) => {
            setSelectedItemId(itemId);
            setDetailOpen(true);
          }}
          title="近期要做"
          trailing={<span className="note-preview-shell__count">{upcomingItems.length}</span>}
        />

        <NotePreviewSection
          activeItemId={selectedItem?.item.item_id ?? null}
          description="已经记下，但还没到处理窗口的事项。"
          items={laterItems}
          onSelect={(itemId) => {
            setSelectedItemId(itemId);
            setDetailOpen(true);
          }}
          title="后续安排"
          trailing={<span className="note-preview-shell__count">{laterItems.length}</span>}
        />

        <NotePreviewSection
          activeItemId={selectedItem?.item.item_id ?? null}
          description="展示规则本身，而不是某一次实例。"
          items={recurringItems}
          onSelect={(itemId) => {
            setSelectedItemId(itemId);
            setDetailOpen(true);
          }}
          title="重复事项"
          trailing={<span className="note-preview-shell__count">{recurringItems.length}</span>}
        />

        <article className="dashboard-card note-preview-shell">
          <div className="note-preview-shell__header">
            <div>
              <p className="dashboard-card__kicker">已结束</p>
              <p className="note-preview-shell__description">默认展示近 3 天，可展开到近 7 天与更多。</p>
            </div>
            <button className="note-preview-shell__toggle" onClick={() => setShowMoreClosed((current) => !current)} type="button">
              {showMoreClosed ? "收起" : "更多"}
            </button>
          </div>
          <div className="note-preview-finished-groups">
            {closedGroups.map((group) => (
              <section key={group.key} className="note-preview-finished-group">
                <div>
                  <p className="note-preview-finished-group__title">{group.title}</p>
                  <p className="note-preview-finished-group__description">{group.description}</p>
                </div>
                <div className="note-preview-shell__list">
                  {group.items.map((entry) => (
                    <NotePreviewCard
                      key={entry.item.item_id}
                      isActive={entry.item.item_id === selectedItem?.item.item_id}
                      item={entry}
                      onSelect={(itemId: string) => {
                        setSelectedItemId(itemId);
                        setDetailOpen(true);
                      }}
                    />
                  ))}
                </div>
              </section>
            ))}
          </div>
        </article>
      </section>

      <AnimatePresence>
        {detailOpen && selectedItem ? (
          <>
            <motion.button
              animate={{ opacity: 1 }}
              className="note-detail-modal__backdrop"
              exit={{ opacity: 0 }}
              initial={{ opacity: 0 }}
              onClick={() => setDetailOpen(false)}
              type="button"
            />
            <motion.div
              animate={{ opacity: 1, scale: 1, y: 0 }}
              className="note-detail-modal"
              exit={{ opacity: 0, scale: 0.98, y: 20 }}
              initial={{ opacity: 0, scale: 0.98, y: 16 }}
              transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
            >
              <NoteDetailPanel feedback={feedback} item={selectedItem} onAction={handleDetailAction} onClose={() => setDetailOpen(false)} />
            </motion.div>
          </>
        ) : null}
      </AnimatePresence>
    </main>
  );
}
