import { useMemo } from "react";
import { Archive, ArrowRight, BadgeAlert, BarChart3, BellOff, BrainCircuit, CalendarCheck2, Check, Clock3, Eye, FileText, Flag, History, Info, Lightbulb, Link2, LoaderCircle, Lock, Mail, MessageCircleMore, NotebookPen, PauseCircle, PencilLine, RefreshCcw, Repeat2, Search, Send, ShieldCheck, Sparkles, Target, UserRound, X } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { resolveDashboardModuleRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import type { DashboardHomeContextItem, DashboardHomeEventStateKey, DashboardHomeModuleKey, DashboardHomeSignalItem, DashboardHomeStateData, DashboardHomeStateGroup } from "../dashboardHome.types";

type DashboardEventPanelProps = {
  activeState: DashboardHomeStateData | null;
  onClose: () => void;
  onStateChange: (stateKey: DashboardHomeEventStateKey) => void;
  stateGroups: DashboardHomeStateGroup[];
  stateMap: Record<DashboardHomeEventStateKey, DashboardHomeStateData>;
};

const contextIcons = {
  archive: Archive,
  alert: BadgeAlert,
  brain: BrainCircuit,
  bulb: Lightbulb,
  calendar: CalendarCheck2,
  chart: BarChart3,
  chat: MessageCircleMore,
  check: Check,
  edit: PencilLine,
  eye: Eye,
  file: FileText,
  flag: Flag,
  focus: Target,
  history: History,
  info: Info,
  link: Link2,
  loader: LoaderCircle,
  lock: Lock,
  mail: Mail,
  mute: BellOff,
  note: NotebookPen,
  pause: PauseCircle,
  question: Info,
  refresh: RefreshCcw,
  repeat: Repeat2,
  search: Search,
  send: Send,
  shield: ShieldCheck,
  sparkles: Sparkles,
  time: Clock3,
  user: UserRound,
};

function renderContext(items: DashboardHomeContextItem[]) {
  return items.map((item) => (
    <article key={`${item.iconKey}-${item.text}`} className="dashboard-orbit-panel__context-item">
      <div className="dashboard-orbit-panel__context-icon-shell">
        {(() => {
          const Icon = contextIcons[item.iconKey as keyof typeof contextIcons] ?? Sparkles;
          return <Icon className="h-4 w-4" />;
        })()}
      </div>
      <div>
        <p className="dashboard-orbit-panel__context-text">{item.text}</p>
        {item.time ? <p className="dashboard-orbit-panel__context-meta">{item.time}</p> : null}
      </div>
    </article>
  ));
}

function renderSignalCards(signals: DashboardHomeSignalItem[]) {
  return signals.map((signal) => (
    <article key={signal.label} className="dashboard-orbit-panel__signal-card">
      <div className="dashboard-orbit-panel__signal-header">
        <div className="dashboard-orbit-panel__context-icon-shell">
          {(() => {
            const Icon = contextIcons[signal.iconKey as keyof typeof contextIcons] ?? Sparkles;
            return <Icon className="h-4 w-4" />;
          })()}
        </div>
        <div>
          <p className="dashboard-orbit-panel__signal-label">{signal.label}</p>
          <p className="dashboard-orbit-panel__signal-value">{signal.value}</p>
        </div>
      </div>
      {signal.translation ? <p className="dashboard-orbit-panel__context-meta">{signal.translation}</p> : null}
    </article>
  ));
}

function modulePrimaryLabel(module: DashboardHomeModuleKey) {
  const labels = {
    tasks: "进入任务页",
    notes: "进入便签页",
    memory: "进入镜子页",
    safety: "进入安全页",
  } as const;

  return labels[module];
}

export function DashboardEventPanel({ activeState, onClose, onStateChange, stateGroups, stateMap }: DashboardEventPanelProps) {
  const navigate = useNavigate();

  function handleOpenModule(module: DashboardHomeModuleKey) {
    onClose();
    window.setTimeout(() => {
      navigate(resolveDashboardModuleRoutePath(module));
    }, 0);
  }

  const moduleStates = useMemo(() => {
    if (!activeState) {
      return [];
    }

    return stateGroups.find((group) => group.key === activeState.module)?.states ?? [];
  }, [activeState, stateGroups]);

  return (
    <AnimatePresence>
      {activeState ? (
        <>
          <motion.div
            animate={{ opacity: 1 }}
            className="dashboard-orbit-panel__backdrop"
            exit={{ opacity: 0 }}
            initial={{ opacity: 0 }}
            onClick={onClose}
          />
          <motion.aside
            animate={{ opacity: 1, x: 0, scale: 1 }}
            className="dashboard-orbit-panel"
            exit={{ opacity: 0, x: 22, scale: 0.98 }}
            initial={{ opacity: 0, x: 30, scale: 0.98 }}
            transition={{ duration: 0.34, ease: [0.22, 1, 0.36, 1] }}
          >
            <div className="dashboard-orbit-panel__hero">
              <div>
                <p className="dashboard-orbit-panel__eyebrow">event focus</p>
                <h2 className="dashboard-orbit-panel__title">{activeState.headline}</h2>
                <p className="dashboard-orbit-panel__lede">{activeState.subline}</p>
              </div>

              <Button className="dashboard-orbit-panel__close" onClick={onClose} size="icon-sm" variant="ghost">
                <X className="h-4 w-4" />
                <span className="sr-only">关闭</span>
              </Button>
            </div>

            <div className="dashboard-orbit-panel__meta-row">
              <span className="dashboard-orbit-panel__pill" style={{ boxShadow: `0 0 0 1px ${activeState.accentColor}32 inset` }}>
                {activeState.tag}
              </span>
              {activeState.progressLabel ? <span className="dashboard-orbit-panel__pill">{activeState.progressLabel}</span> : null}
              <span className="dashboard-orbit-panel__pill">{stateGroups.find((group) => group.key === activeState.module)?.label}</span>
            </div>

            {moduleStates.length > 1 ? (
              <div className="dashboard-orbit-panel__state-row">
                {moduleStates.map((stateKey) => (
                  <button
                    key={stateKey}
                    className="dashboard-orbit-panel__state-chip"
                    data-active={stateKey === activeState.key ? "true" : "false"}
                    onClick={() => onStateChange(stateKey)}
                    type="button"
                  >
                    {stateMap[stateKey].label}
                  </button>
                ))}
              </div>
            ) : null}

            {activeState.progressSteps?.length ? (
              <div className="dashboard-orbit-panel__progress-track">
                {activeState.progressSteps.map((step) => (
                  <div key={step.label} className="dashboard-orbit-panel__progress-step">
                    <span className="dashboard-orbit-panel__progress-dot" data-status={step.status} />
                    <span className="dashboard-orbit-panel__progress-label">{step.label}</span>
                  </div>
                ))}
              </div>
            ) : null}

            <div className="dashboard-orbit-panel__section-grid">
              <section className="dashboard-orbit-panel__section">
                <h3 className="dashboard-orbit-panel__section-title">当前上下文</h3>
                <div className="dashboard-orbit-panel__context-list">{renderContext(activeState.context)}</div>
              </section>

              {activeState.notes?.length ? (
                <section className="dashboard-orbit-panel__section">
                  <h3 className="dashboard-orbit-panel__section-title">便签片段</h3>
                  <div className="dashboard-orbit-panel__context-list">
                    {activeState.notes.map((note) => (
                      <article key={note.id} className="dashboard-orbit-panel__context-item">
                        <div className="dashboard-orbit-panel__context-icon-shell">
                          <NotebookPen className="h-4 w-4" />
                        </div>
                        <div>
                          <p className="dashboard-orbit-panel__context-text">{note.text}</p>
                          <p className="dashboard-orbit-panel__context-meta">
                            {note.tag ? `${note.tag} · ` : ""}
                            {note.time ?? note.status}
                          </p>
                        </div>
                      </article>
                    ))}
                  </div>
                </section>
              ) : null}

              {activeState.insights?.length ? (
                <section className="dashboard-orbit-panel__section">
                  <h3 className="dashboard-orbit-panel__section-title">镜子观察</h3>
                  <div className="dashboard-orbit-panel__context-list">
                    {activeState.insights.map((insight) => (
                      <article key={insight.text} className="dashboard-orbit-panel__context-item">
                        <div className="dashboard-orbit-panel__context-icon-shell">
                          {(() => {
                            const Icon = contextIcons[insight.iconKey as keyof typeof contextIcons] ?? Sparkles;
                            return <Icon className="h-4 w-4" />;
                          })()}
                        </div>
                        <p className="dashboard-orbit-panel__context-text" data-emphasis={insight.emphasis ? "true" : "false"}>
                          {insight.text}
                        </p>
                      </article>
                    ))}
                  </div>
                </section>
              ) : null}

              {activeState.signals?.length ? (
                <section className="dashboard-orbit-panel__section">
                  <h3 className="dashboard-orbit-panel__section-title">边界摘要</h3>
                  <div className="dashboard-orbit-panel__signal-grid">{renderSignalCards(activeState.signals)}</div>
                </section>
              ) : null}

              {activeState.anomaly ? (
                <section className="dashboard-orbit-panel__section dashboard-orbit-panel__section--wide">
                  <h3 className="dashboard-orbit-panel__section-title">当前提醒</h3>
                  <div className="dashboard-orbit-panel__alert-box" data-severity={activeState.anomaly.severity}>
                    <div>
                      <p className="dashboard-orbit-panel__alert-title">{activeState.anomaly.title}</p>
                      <p className="dashboard-orbit-panel__lede">{activeState.anomaly.desc}</p>
                    </div>
                    <div className="dashboard-orbit-panel__alert-actions">
                       <Button className="dashboard-orbit-panel__action-button" onClick={() => handleOpenModule(activeState.module)} onPointerDown={(event) => event.stopPropagation()} type="button" variant="ghost">
                          {activeState.anomaly.actionLabel}
                          <ArrowRight className="h-4 w-4" />
                        </Button>
                       <Button className="dashboard-orbit-panel__action-button dashboard-orbit-panel__action-button--mute" onClick={onClose} type="button" variant="ghost">
                         {activeState.anomaly.dismissLabel}
                       </Button>
                    </div>
                  </div>
                </section>
              ) : null}
            </div>

            <div className="dashboard-orbit-panel__footer">
              <Button className="dashboard-orbit-panel__primary-button" onClick={() => handleOpenModule(activeState.module)} onPointerDown={(event) => event.stopPropagation()} type="button">
                {modulePrimaryLabel(activeState.module)}
                <ArrowRight className="h-4 w-4" />
              </Button>
              <div className="dashboard-orbit-panel__footer-note">
                <Sparkles className="h-4 w-4" />
                这是首页事件舱，点击主按钮可进入对应子页面
              </div>
            </div>
          </motion.aside>
        </>
      ) : null}
    </AnimatePresence>
  );
}
