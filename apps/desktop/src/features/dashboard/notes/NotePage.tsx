import { useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { Link, NavLink, useNavigate } from "react-router-dom";
import { useMutation, useQueries } from "@tanstack/react-query";
import { AlertTriangle, ArrowLeft, CircleDashed, NotebookPen, RefreshCcw } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { loadDashboardDataMode, saveDashboardDataMode } from "@/features/dashboard/shared/dashboardDataMode";
import { DashboardMockToggle } from "@/features/dashboard/shared/DashboardMockToggle";
import { resolveDashboardModuleRoutePath, resolveDashboardRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { dashboardModules } from "@/features/dashboard/shared/dashboardRoutes";
import { cn } from "@/utils/cn";
import { buildNoteSummary, describeNotePreview, groupClosedNotes, sortClosedNotes, sortNotesByUrgency } from "./notePage.mapper";
import { convertNoteToTask, loadNoteBucket, type NotePageDataMode } from "./notePage.service";
import type { NoteDetailAction, NoteListItem } from "./notePage.types";
import { NoteDetailPanel } from "./components/NoteDetailPanel";
import { NotePreviewCard } from "./components/NotePreviewCard";
import { NotePreviewSection } from "./components/NotePreviewSection";
import "./notePage.css";

export function NotePage() {
  const navigate = useNavigate();
  const [selectedItemId, setSelectedItemId] = useState<string | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [showMoreClosed, setShowMoreClosed] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [dataMode, setDataMode] = useState<NotePageDataMode>(() => loadDashboardDataMode("notes") as NotePageDataMode);
  const feedbackTimeoutRef = useRef<number | null>(null);

  useEffect(() => {
    saveDashboardDataMode("notes", dataMode);
  }, [dataMode]);

  const [upcomingQuery, laterQuery, recurringQuery, closedQuery] = useQueries({
    queries: [
        {
          queryKey: ["dashboard", "notes", "bucket", dataMode, "upcoming"],
          queryFn: () => loadNoteBucket("upcoming", dataMode),
        retry: false,
        refetchOnMount: false,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
      },
        {
          queryKey: ["dashboard", "notes", "bucket", dataMode, "later"],
          queryFn: () => loadNoteBucket("later", dataMode),
        retry: false,
        refetchOnMount: false,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
      },
        {
          queryKey: ["dashboard", "notes", "bucket", dataMode, "recurring_rule"],
          queryFn: () => loadNoteBucket("recurring_rule", dataMode),
        retry: false,
        refetchOnMount: false,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
      },
        {
          queryKey: ["dashboard", "notes", "bucket", dataMode, "closed"],
          queryFn: () => loadNoteBucket("closed", dataMode),
        retry: false,
        refetchOnMount: false,
        refetchOnReconnect: false,
        refetchOnWindowFocus: false,
      },
    ],
  });

  const upcomingItems = sortNotesByUrgency(upcomingQuery.data?.items ?? []);
  const laterItems = sortNotesByUrgency(laterQuery.data?.items ?? []);
  const recurringItems = sortNotesByUrgency(recurringQuery.data?.items ?? []);
  const closedItems = sortClosedNotes(closedQuery.data?.items ?? []);
  const closedGroups = useMemo(() => groupClosedNotes(closedItems, showMoreClosed), [closedItems, showMoreClosed]);
  const summary = useMemo(() => buildNoteSummary({ recurring_rule: recurringItems, upcoming: upcomingItems }), [recurringItems, upcomingItems]);
  const allItems = useMemo(() => [...upcomingItems, ...laterItems, ...recurringItems, ...closedItems], [upcomingItems, laterItems, recurringItems, closedItems]);
  const selectedItem = useMemo(
    () => allItems.find((entry) => entry.item.item_id === selectedItemId) ?? upcomingItems[0] ?? laterItems[0] ?? recurringItems[0] ?? closedItems[0] ?? null,
    [allItems, closedItems, laterItems, recurringItems, selectedItemId, upcomingItems],
  );

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
    feedbackTimeoutRef.current = window.setTimeout(() => setFeedback(null), 2600);
  }

  const convertMutation = useMutation({
    mutationFn: (itemId: string) => convertNoteToTask(itemId, dataMode),
    onSuccess: (outcome) => {
      showFeedback("已为这条事项生成任务，正在跳转到任务页。");
      navigate(resolveDashboardModuleRoutePath("tasks"), { state: { focusTaskId: outcome.result.task.task_id, openDetail: true } });
    },
    onError: (error) => {
      const message = error instanceof Error ? error.message : "转交给 Agent 失败，请稍后再试。";
      showFeedback(`转交给 Agent 失败：${message}`);
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
  }, [allItems, closedItems, laterItems, recurringItems, selectedItemId, upcomingItems]);

  useEffect(() => {
    return () => {
      if (feedbackTimeoutRef.current) {
        window.clearTimeout(feedbackTimeoutRef.current);
      }
    };
  }, []);

  const queryErrors = [
    { label: "近期要做", error: upcomingQuery.error },
    { label: "后续安排", error: laterQuery.error },
    { label: "重复事项", error: recurringQuery.error },
    { label: "已结束", error: closedQuery.error },
  ].filter((item) => item.error);

  const pageNotice =
    selectedItem
      ? `${selectedItem.item.title} · ${describeNotePreview(selectedItem.item, selectedItem.experience)}`
      : "便签协作会把近期要做、后续安排、重复事项和已结束事项整理在这里。";

  return (
    <main className="dashboard-page note-preview-page" style={pageStyle}>
      <>
        <header className="dashboard-page__topbar">
            <Link className="dashboard-page__home-link" to={resolveDashboardRoutePath("home")}>
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
              <div className="dashboard-card__status-row">
                <CircleDashed className="h-4 w-4" />
                <span>{pageNotice}</span>
              </div>
            </div>
        </section>

        <section className="dashboard-page__grid note-preview-page__grid">
            <NotePreviewSection
              activeItemId={selectedItem?.item.item_id ?? null}
              description="快到时间、今天要做、最近几天需要处理的事项。"
              emptyLabel={upcomingQuery.isPending && !upcomingQuery.data ? "加载中" : "无"}
              items={upcomingItems}
              onSelect={(itemId) => {
                setSelectedItemId(itemId);
                setDetailOpen(true);
              }}
              title="近期要做"
              trailing={<span className="note-preview-shell__count">{upcomingQuery.isPending && !upcomingQuery.data ? "..." : upcomingItems.length}</span>}
            />

            <NotePreviewSection
              activeItemId={selectedItem?.item.item_id ?? null}
              description="已经记下，但还没到处理窗口的事项。"
              emptyLabel={laterQuery.isPending && !laterQuery.data ? "加载中" : "无"}
              items={laterItems}
              onSelect={(itemId) => {
                setSelectedItemId(itemId);
                setDetailOpen(true);
              }}
              title="后续安排"
              trailing={<span className="note-preview-shell__count">{laterQuery.isPending && !laterQuery.data ? "..." : laterItems.length}</span>}
            />

            <NotePreviewSection
              activeItemId={selectedItem?.item.item_id ?? null}
              description="展示规则本身，而不是某一次实例。"
              emptyLabel={recurringQuery.isPending && !recurringQuery.data ? "加载中" : "无"}
              items={recurringItems}
              onSelect={(itemId) => {
                setSelectedItemId(itemId);
                setDetailOpen(true);
              }}
              title="重复事项"
              trailing={<span className="note-preview-shell__count">{recurringQuery.isPending && !recurringQuery.data ? "..." : recurringItems.length}</span>}
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
                {closedGroups.length > 0 ? (
                  closedGroups.map((group) => (
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
                  ))
                ) : closedQuery.isPending && !closedQuery.data ? (
                  <div className="note-preview-shell__empty">加载中</div>
                ) : (
                  <div className="note-preview-shell__empty">无</div>
                )}
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

        <AnimatePresence>
            {(feedback || queryErrors.length > 0) ? (
              <motion.aside
                animate={{ opacity: 1, y: 0 }}
                className="note-preview-page__floating-card"
                exit={{ opacity: 0, y: 12 }}
                initial={{ opacity: 0, y: 16 }}
                transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
              >
                <div className="note-preview-page__floating-card-icon">
                  <AlertTriangle className="h-4 w-4" />
                </div>
                <div className="note-preview-page__floating-card-copy">
                  <p className="note-preview-page__floating-card-title">{feedback ? "操作提示" : "便签同步失败"}</p>
                  <p className="note-preview-page__floating-card-text">
                    {feedback ??
                      (queryErrors.length === 1
                        ? `${queryErrors[0].label}：${queryErrors[0].error instanceof Error ? queryErrors[0].error.message : "请求失败"}`
                        : `${queryErrors.length} 个分区加载失败：${queryErrors
                            .map((item) => `${item.label}${item.error instanceof Error ? `(${item.error.message})` : ""}`)
                            .join("、")}`)}
                  </p>
                </div>
                {!feedback ? (
                  <button
                    className="note-preview-page__floating-card-action"
                    onClick={() => {
                      void upcomingQuery.refetch();
                      void laterQuery.refetch();
                      void recurringQuery.refetch();
                      void closedQuery.refetch();
                    }}
                    type="button"
                  >
                    <RefreshCcw className="h-4 w-4" />
                    重试
                  </button>
                ) : null}
              </motion.aside>
            ) : null}
        </AnimatePresence>

        <DashboardMockToggle
          enabled={dataMode === "mock"}
          onToggle={() => {
            setFeedback(null);
            setDataMode((current) => (current === "rpc" ? "mock" : "rpc"));
          }}
        />
      </>
    </main>
  );
}
