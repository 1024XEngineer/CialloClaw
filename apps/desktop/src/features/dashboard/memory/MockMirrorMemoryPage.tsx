import { useMemo, useState } from "react";
import { format } from "date-fns";
import { zhCN } from "date-fns/locale";
import { ArrowRight, Calendar as CalendarIcon, Copy, X } from "lucide-react";
import memoryBackground from "@/assets/background_memory.png";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useDashboardEscapeHandler } from "@/features/dashboard/shared/dashboardEscapeCoordinator";
import {
  findMockRecentMemory,
  findMockSessionSummary,
  findMockSessionSummaryByDate,
  mockDailySummary,
  mockHeatmap,
  mockPhaseSummary,
  mockProfileAxes,
  mockProfileMetrics,
  mockRecentMemories,
  type MockRecentMemory,
  type MockSessionSummary,
} from "./mockMirrorMemoryData";
import "./mockMirrorMemory.css";

type MainTab = "profile" | "period";
type HistoryDate = string;
type MemoryModalState =
  | { kind: "recent"; memory: MockRecentMemory }
  | { kind: "session"; session: MockSessionSummary }
  | null;

const defaultHistoryDate = new Date(2026, 4, 14);
const historyDataDate = new Date(2026, 4, 14);

function formatHistoryDate(date: Date) {
  return format(date, "yyyy.MM.dd");
}

/**
 * Renders the new mock-only mirror memory workspace used by the dashboard.
 */
export function MockMirrorMemoryOverview() {
  const [mainTab, setMainTab] = useState<MainTab>("profile");
  const [selectedHistoryDate, setSelectedHistoryDate] = useState<Date | undefined>(defaultHistoryDate);
  const [historyCalendarMonth, setHistoryCalendarMonth] = useState(defaultHistoryDate);
  const [activeModal, setActiveModal] = useState<MemoryModalState>(null);
  const recentMemory = mockRecentMemories[0] ?? null;
  const selectedHistoryDateKey = useMemo<HistoryDate>(() => (selectedHistoryDate ? formatHistoryDate(selectedHistoryDate) : formatHistoryDate(defaultHistoryDate)), [selectedHistoryDate]);
  const selectedSession = useMemo(() => findMockSessionSummaryByDate(selectedHistoryDateKey), [selectedHistoryDateKey]);

  useDashboardEscapeHandler({
    enabled: activeModal !== null,
    handleEscape: () => setActiveModal(null),
    priority: 220,
  });

  function openRecent(memoryId: string) {
    const memory = findMockRecentMemory(memoryId);
    if (memory) {
      setActiveModal({ kind: "recent", memory });
    }
  }

  function openSession(sessionId: string) {
    const session = findMockSessionSummary(sessionId);
    if (session) {
      setActiveModal({ kind: "session", session });
    }
  }

  function handleHistoryDateSelect(date: Date | undefined) {
    if (!date) {
      return;
    }

    setSelectedHistoryDate(date);
    setHistoryCalendarMonth(date);
  }

  return (
    <MemoryScene>
      <div className="mirror-memory-layout">
        <aside className="mirror-memory-left-column">
          <section className="mirror-memory-card mirror-memory-card--recent">
            <div className="mirror-memory-card-header mirror-memory-card-header--split">
              <div>
                <div className="mirror-memory-eyebrow">近期被调用记忆</div>
                <h2>近期被调用记忆</h2>
                <p>最近被引用的记忆片段</p>
              </div>
              <span className="mirror-memory-pill">1 条引用</span>
            </div>

            {recentMemory ? (
              <div className="mirror-memory-scroll-shell">
                <button className="mirror-memory-list-link" onClick={() => openRecent(recentMemory.id)} type="button">
                  <div className="mirror-memory-list-copy">
                    <div className="mirror-memory-list-title">{recentMemory.title}</div>
                    <div className="mirror-memory-list-meta">上次引用：{recentMemory.lastReferencedAt}</div>
                    <div className="mirror-memory-list-meta">来源任务：{recentMemory.sourceTask}</div>
                  </div>
                  <div className="mirror-memory-list-side mirror-memory-list-side--stacked">
                    <span>1 次</span>
                    <ArrowRight className="mirror-memory-list-arrow" />
                  </div>
                </button>
              </div>
            ) : null}
          </section>

          <section className="mirror-memory-card mirror-memory-card--history">
            <div className="mirror-memory-card-header">
              <div className="mirror-memory-eyebrow">历史概要</div>
              <h2>历史概要</h2>
              <p>按日期查看当天的会话摘要</p>
            </div>

            <div className="mirror-memory-date-picker-shell">
              <Popover>
                <PopoverTrigger
                  render={
                    <Button
                      id="memory-history-date-button"
                      variant="outline"
                      data-empty={!selectedHistoryDate}
                      className="mirror-memory-date-picker-button justify-start text-left font-normal data-[empty=true]:text-muted-foreground"
                    />
                  }
                >
                  <CalendarIcon />
                  {selectedHistoryDate ? formatHistoryDate(selectedHistoryDate) : <span>Pick a date</span>}
                </PopoverTrigger>
                <PopoverContent align="start" className="mirror-memory-date-picker-popover" side="top" sideOffset={8}>
                  <Calendar
                    className="mirror-memory-calendar"
                    defaultMonth={defaultHistoryDate}
                    locale={zhCN}
                    mode="single"
                    month={historyCalendarMonth}
                    modifiers={{ hasData: historyDataDate }}
                    modifiersClassNames={{ hasData: "mirror-memory-calendar-day--has-data" }}
                    onMonthChange={setHistoryCalendarMonth}
                    onSelect={handleHistoryDateSelect}
                    selected={selectedHistoryDate}
                    weekStartsOn={1}
                  />
                </PopoverContent>
              </Popover>
            </div>

            <div className="mirror-memory-scroll-shell mirror-memory-scroll-shell--history">
              {selectedSession ? (
                <article className="mirror-memory-session-card">
                  <div className="mirror-memory-session-top">
                    <div>
                      <div className="mirror-memory-session-title">{selectedSession.title}</div>
                      <p className="mirror-memory-session-meta">时间：{selectedSession.timeRange}</p>
                      <p className="mirror-memory-session-meta">消息数：{selectedSession.messageCount} 条</p>
                      <p className="mirror-memory-session-meta">使用时长：{selectedSession.durationLabel}</p>
                      <p className="mirror-memory-session-meta">原始文件：{selectedSession.sourceFile}</p>
                    </div>
                    <button className="mirror-memory-inline-button" onClick={() => openSession(selectedSession.id)} type="button">
                      查看详情
                    </button>
                  </div>
                </article>
              ) : (
                <div className="mirror-memory-empty-state">这一天之前还没有会话记录。</div>
              )}
            </div>
          </section>
        </aside>

        <section className="mirror-memory-card mirror-memory-card--spotlight">
          <div className="mirror-memory-main-tabs" role="tablist" aria-label="镜子记忆主视图切换">
            <TabButton active={mainTab === "profile"} label="用户画像" onClick={() => setMainTab("profile")} />
            <TabButton active={mainTab === "period"} label="周期总结" onClick={() => setMainTab("period")} />
          </div>

          {mainTab === "profile" ? (
            <div className="mirror-memory-profile-panel">
              <div className="mirror-memory-profile-top">
                <div className="mirror-memory-radar-wrap">
                  <RadarChart />
                </div>

                <div className="mirror-memory-profile-side">
                  <div className="mirror-memory-identity-card">
                    <div className="mirror-memory-identity-row">
                      <div>
                        <div className="mirror-memory-eyebrow">PROFILE ID</div>
                        <h2>初次协作样本</h2>
                      </div>
                      <span className="mirror-memory-pill">观察中</span>
                    </div>
                  </div>

                  <div className="mirror-memory-metric-grid">
                    {mockProfileMetrics.map((metric) => (
                      <div key={metric.label} className="mirror-memory-metric-item">
                        <span>{metric.label}</span>
                        <strong>{metric.value}</strong>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          ) : (
            <div className="mirror-memory-period-panel">
              <div className="mirror-memory-card-header">
                <div className="mirror-memory-eyebrow">日报与阶段总结</div>
                <h2>日报与阶段总结</h2>
              </div>

              <section className="mirror-memory-summary-card">
                <div className="mirror-memory-summary-block">
                  <div className="mirror-memory-summary-title">{mockDailySummary.title}</div>
                  <ul className="mirror-memory-copy-list mirror-memory-copy-list--clean">
                    {mockDailySummary.lines.map((line) => (
                      <li key={line}>{line}</li>
                    ))}
                  </ul>
                </div>
                <div className="mirror-memory-summary-block">
                  <div className="mirror-memory-summary-title">{mockPhaseSummary.title}</div>
                  <ul className="mirror-memory-copy-list mirror-memory-copy-list--clean">
                    {mockPhaseSummary.lines.map((line) => (
                      <li key={line}>{line}</li>
                    ))}
                  </ul>
                </div>
              </section>

              <section className="mirror-memory-heatmap-card">
                <div className="mirror-memory-eyebrow">活跃时段热力图</div>
                <HeatmapGrid />
              </section>
            </div>
          )}
        </section>
      </div>

      {activeModal ? <MemoryModalOverlay modal={activeModal} onClose={() => setActiveModal(null)} /> : null}
    </MemoryScene>
  );
}

function MemoryModalOverlay({ modal, onClose }: { modal: Exclude<MemoryModalState, null>; onClose: () => void }) {
  return (
    <div aria-modal="true" className="mirror-memory-modal-layer" role="dialog">
      <button aria-label="关闭镜子记忆弹窗" className="mirror-memory-modal-backdrop" onClick={onClose} type="button" />
      <div className="mirror-memory-modal-shell">
        <div className="mirror-memory-detail-card mirror-memory-detail-card--modal">
          <div className="mirror-memory-modal-actions">
            <span className="mirror-memory-pill">{modal.kind === "recent" ? "记忆详情" : "会话摘要详情"}</span>
            <button aria-label="关闭弹窗" className="mirror-memory-modal-close" onClick={onClose} type="button">
              <X className="mirror-memory-modal-close-icon" />
            </button>
          </div>
          {modal.kind === "recent" ? <RecentMemoryModalBody memory={modal.memory} /> : <SessionModalBody session={modal.session} />}
        </div>
      </div>
    </div>
  );
}

function RecentMemoryModalBody({ memory }: { memory: MockRecentMemory }) {
  const [feedback, setFeedback] = useState<string | null>(null);

  const copyPayload = useMemo(() => {
    return [
      memory.title,
      `来源：${memory.source}`,
      `引用时间：${memory.lastReferencedAt}`,
      `来自任务：${memory.sourceTask}`,
      `引用次数：${memory.referenceCount}`,
      `记忆摘要：${memory.summary}`,
      `适用范围：${memory.scope}`,
      `关键要点：${memory.highlights.join(" / ")}`,
      `原始记录：${memory.rawRecord}`,
      `关联偏好：${memory.relatedPreference}`,
      `相关历史：${memory.historyLinks.join(" / ")}`,
    ].join("\n");
  }, [memory]);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(copyPayload);
      setFeedback("已复制到剪贴板");
    } catch {
      setFeedback("当前环境无法直接复制，但内容仍然保留在这页");
    }
  }

  return (
    <>
      <header className="mirror-memory-detail-header">
        <div>
          <div className="mirror-memory-eyebrow">记忆详情</div>
          <h1>{memory.title}</h1>
          <p>{memory.summary}</p>
        </div>
        <div className="mirror-memory-detail-counter">
          <span>引用次数</span>
          <strong>{memory.referenceCount}</strong>
        </div>
      </header>

      <div className="mirror-memory-detail-grid">
        <DetailBlock label="来源" value={memory.source} />
        <DetailBlock label="引用时间" value={memory.lastReferencedAt} />
        <DetailBlock label="来自任务" value={memory.sourceTask} />
        <DetailBlock label="适用范围" value={memory.scope} />
      </div>

      <div className="mirror-memory-detail-body mirror-memory-scroll-shell">
        <DetailSection title="关键要点">
          <div className="mirror-memory-highlight-row">
            {memory.highlights.map((highlight) => (
              <span key={highlight} className="mirror-memory-highlight-chip">{highlight}</span>
            ))}
          </div>
        </DetailSection>
        <DetailSection title="原始记录">
          <p>{memory.rawRecord}</p>
        </DetailSection>
        <DetailSection title="关联偏好">
          <p>{memory.relatedPreference}</p>
        </DetailSection>
        <DetailSection title="相关历史">
          <ul className="mirror-memory-copy-list mirror-memory-copy-list--clean">
            {memory.historyLinks.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </DetailSection>
      </div>

      <footer className="mirror-memory-action-bar">
        <button className="mirror-memory-action-button mirror-memory-action-button--primary" onClick={handleCopy} type="button">
          <Copy className="mirror-memory-action-icon" />
          复制内容
        </button>
        <button className="mirror-memory-action-button" onClick={() => setFeedback("这是一条 mock 操作：后续可接入不再引用流程")} type="button">
          不再引用
        </button>
        <button className="mirror-memory-action-button" onClick={() => setFeedback("这是一条 mock 操作：后续可接入当前任务上下文")} type="button">
          应用到当前任务
        </button>
      </footer>

      {feedback ? <div className="mirror-memory-feedback">{feedback}</div> : null}
    </>
  );
}

function SessionModalBody({ session }: { session: MockSessionSummary }) {
  return (
    <>
      <header className="mirror-memory-detail-header">
        <div>
          <div className="mirror-memory-eyebrow">会话摘要详情</div>
          <h1>{session.title}</h1>
          <p>{session.summary}</p>
        </div>
        <div className="mirror-memory-detail-counter">
          <span>消息数</span>
          <strong>{session.messageCount}</strong>
        </div>
      </header>

      <div className="mirror-memory-detail-grid">
        <DetailBlock label="日期" value={session.date} />
        <DetailBlock label="时间" value={session.timeRange} />
        <DetailBlock label="使用时长" value={session.durationLabel} />
        <DetailBlock label="原始文件" value={session.sourceFile} />
      </div>

      <div className="mirror-memory-detail-body mirror-memory-scroll-shell">
        <DetailSection title="摘要">
          <p>{session.summary}</p>
        </DetailSection>
        <DetailSection title="会话要点">
          <ul className="mirror-memory-copy-list mirror-memory-copy-list--clean">
            {session.detailLines.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        </DetailSection>
      </div>
    </>
  );
}

function MemoryScene({ children }: { children: React.ReactNode }) {
  return <main className="mirror-memory-page" style={{ backgroundImage: `url(${memoryBackground})` , backgroundPosition: "left" }}>{children}</main>;
}

function DetailBlock({ label, value }: { label: string; value: string }) {
  return (
    <div className="mirror-memory-detail-block">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function DetailSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mirror-memory-detail-section">
      <div className="mirror-memory-eyebrow">{title}</div>
      <div>{children}</div>
    </section>
  );
}

function TabButton({ active, label, onClick }: { active: boolean; label: string; onClick: () => void }) {
  return (
    <button className={`mirror-memory-tab-button${active ? " is-active" : ""}`} onClick={onClick} type="button">
      {label}
    </button>
  );
}

function RadarChart() {
  const center = 86;
  const radius = 62;
  const levels = [0.2, 0.4, 0.6, 0.8];
  const angleStep = (Math.PI * 2) / mockProfileAxes.length;
  const points = mockProfileAxes.map((axis, index) => {
    const angle = -Math.PI / 2 + index * angleStep;
    const x = center + Math.cos(angle) * radius * axis.value;
    const y = center + Math.sin(angle) * radius * axis.value;
    return `${x},${y}`;
  }).join(" ");

  return (
    <svg aria-hidden="true" className="mirror-memory-radar" viewBox="0 0 172 172">
      {levels.map((level) => {
        const polygon = mockProfileAxes.map((_, index) => {
          const angle = -Math.PI / 2 + index * angleStep;
          const x = center + Math.cos(angle) * radius * level;
          const y = center + Math.sin(angle) * radius * level;
          return `${x},${y}`;
        }).join(" ");
        return <polygon key={level} className="mirror-memory-radar-grid" points={polygon} />;
      })}
      {mockProfileAxes.map((axis, index) => {
        const angle = -Math.PI / 2 + index * angleStep;
        const x = center + Math.cos(angle) * radius;
        const y = center + Math.sin(angle) * radius;
        const labelX = center + Math.cos(angle) * (radius + 18);
        const labelY = center + Math.sin(angle) * (radius + 18);
        return (
          <g key={axis.label}>
            <line className="mirror-memory-radar-axis" x1={center} x2={x} y1={center} y2={y} />
            <text className="mirror-memory-radar-label" textAnchor="middle" x={labelX} y={labelY}>{axis.label}</text>
          </g>
        );
      })}
      <polygon className="mirror-memory-radar-shape mirror-memory-radar-shape--weak" points={points} />
      {mockProfileAxes.map((axis, index) => {
        const angle = -Math.PI / 2 + index * angleStep;
        const x = center + Math.cos(angle) * radius * axis.value;
        const y = center + Math.sin(angle) * radius * axis.value;
        return <circle key={axis.label} className="mirror-memory-radar-point" cx={x} cy={y} r="3" />;
      })}
    </svg>
  );
}

function HeatmapGrid() {
  const columns = ["一", "二", "三", "四", "五", "六", "日"];

  return (
    <div className="mirror-memory-heatmap-grid">
      <div className="mirror-memory-heatmap-label-row">
        {columns.map((column) => <span key={column}>{column}</span>)}
      </div>
      <div className="mirror-memory-heatmap-cells">
        {mockHeatmap.flatMap((row, rowIndex) => row.map((value, columnIndex) => (
          <span
            key={`${rowIndex}-${columnIndex}`}
            className="mirror-memory-heatmap-cell"
            style={{
              opacity: value === 0 ? 0.18 : 1,
              background: value === 0
                ? "rgba(182, 182, 182, 0.42)"
                : "linear-gradient(145deg, rgba(233, 197, 121, 0.96), rgba(205, 171, 112, 0.84))",
            }}
          />
        )))}
      </div>
    </div>
  );
}
