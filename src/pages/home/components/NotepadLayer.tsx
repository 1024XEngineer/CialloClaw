import { useEffect, useState } from 'react';
import type { AgentStateData, NotepadItem } from '@/mocks/agentStates';

interface NotepadLayerProps {
  stateData: AgentStateData;
  visible: boolean;
}

const statusConfig: Record<NotepadItem['status'], { color: string; icon: string; label: string }> = {
  pending:    { color: '#64748b', icon: 'ri-time-line',     label: '待执行' },
  processing: { color: '#34d399', icon: 'ri-loader-4-line', label: '进行中' },
  done:       { color: '#94a3b8', icon: 'ri-check-line',    label: '已完成' },
  recurring:  { color: '#a78bfa', icon: 'ri-repeat-line',   label: '重复任务' },
};

// ── MD File Inspector ──
function MdFileInspector({ color, visible }: { color: string; visible: boolean }) {
  const [show, setShow] = useState(false);
  const [expandedFile, setExpandedFile] = useState<string | null>('tasks.md');

  useEffect(() => {
    if (visible) { const t = setTimeout(() => setShow(true), 80); return () => clearTimeout(t); }
    setShow(false);
  }, [visible]);

  const files = [
    {
      name: 'tasks.md',
      lastScan: '09:00 今日',
      taskCount: 7,
      urgent: 2,
      tasks: [
        { text: '完成 Q3 竞品分析报告', date: '今日', priority: 'high', status: 'pending', tags: ['文档', '优先'] },
        { text: '联系设计师确认 UI 排期', date: '今日', priority: 'high', status: 'pending', tags: ['沟通'] },
        { text: '整理上周会议纪要', date: '明日', priority: 'normal', status: 'processing', tags: ['文档'] },
        { text: '更新产品路线图', date: '本周五', priority: 'normal', status: 'pending', tags: ['规划'] },
        { text: '回复客户邮件', date: '已完成', priority: 'normal', status: 'done', tags: ['沟通'] },
      ],
    },
    {
      name: 'weekly-review.md',
      lastScan: '周一 09:00',
      taskCount: 3,
      urgent: 0,
      tasks: [
        { text: '整理本周完成事项', date: '每周一', priority: 'normal', status: 'recurring', tags: ['周报'] },
        { text: '更新 OKR 进度', date: '每周一', priority: 'normal', status: 'recurring', tags: ['规划'] },
        { text: '发送周报给团队', date: '每周一', priority: 'normal', status: 'pending', tags: ['沟通'] },
      ],
    },
  ];

  const priorityColor = (p: string) => p === 'high' ? '#fb923c' : 'rgba(100,116,139,0.6)';

  return (
    <div style={{ opacity: show ? 1 : 0, transform: show ? 'translateY(0)' : 'translateY(6px)', transition: 'opacity 0.4s ease, transform 0.4s ease' }}>
      {/* Section header */}
      <div className="flex items-center gap-2 mb-3">
        <div className="w-4 h-px" style={{ background: `linear-gradient(to right, ${color}60, transparent)` }} />
        <i className="ri-file-text-line" style={{ fontSize: 9, color }} />
        <span style={{ fontSize: 9.5, color, letterSpacing: '0.18em', fontWeight: 600, opacity: 0.85 }}>
          .MD 任务文件巡检
        </span>
        <div className="flex items-center gap-1 px-1.5 py-0.5 rounded" style={{ background: `${color}14`, border: `1px solid ${color}25` }}>
          <div className="rounded-full" style={{ width: 4, height: 4, background: color, animation: 'notifPulse 2s ease-in-out infinite' }} />
          <span style={{ fontSize: 8, color, letterSpacing: '0.1em' }}>定时巡检中</span>
        </div>
      </div>

      {/* File list */}
      <div className="flex flex-col gap-2">
        {files.map((file, fi) => {
          const isExpanded = expandedFile === file.name;
          return (
            <div
              key={file.name}
              className="rounded-lg overflow-hidden"
              style={{
                background: 'rgba(255,255,255,0.025)',
                border: `1px solid ${isExpanded ? color + '30' : 'rgba(255,255,255,0.06)'}`,
                opacity: show ? 1 : 0,
                transition: `opacity 0.35s ease ${fi * 0.08}s, border-color 0.2s ease`,
              }}
            >
              {/* File header */}
              <button
                className="w-full flex items-center justify-between px-3 py-2.5 cursor-pointer"
                onClick={() => setExpandedFile(isExpanded ? null : file.name)}
                style={{ background: 'transparent' }}
              >
                <div className="flex items-center gap-2.5">
                  <div className="flex items-center justify-center rounded" style={{ width: 22, height: 22, background: `${color}12`, border: `1px solid ${color}20` }}>
                    <i className="ri-markdown-line" style={{ fontSize: 10, color }} />
                  </div>
                  <div className="text-left">
                    <div style={{ fontSize: 11.5, color: 'rgba(226,232,240,0.85)', fontFamily: 'monospace', letterSpacing: '0.02em' }}>
                      {file.name}
                    </div>
                    <div style={{ fontSize: 9, color: 'rgba(71,85,105,0.55)', marginTop: 1 }}>
                      上次巡检 {file.lastScan} · {file.taskCount} 项任务
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  {file.urgent > 0 && (
                    <div className="px-1.5 py-0.5 rounded" style={{ background: 'rgba(251,146,60,0.12)', border: '1px solid rgba(251,146,60,0.25)' }}>
                      <span style={{ fontSize: 8.5, color: '#fb923c' }}>{file.urgent} 紧急</span>
                    </div>
                  )}
                  <i className={isExpanded ? 'ri-arrow-up-s-line' : 'ri-arrow-down-s-line'} style={{ fontSize: 13, color: 'rgba(71,85,105,0.5)' }} />
                </div>
              </button>

              {/* Expanded task list */}
              {isExpanded && (
                <div className="px-3 pb-3 flex flex-col gap-1.5" style={{ borderTop: `1px solid ${color}10` }}>
                  <div className="flex items-center gap-2 py-2">
                    <span style={{ fontSize: 8.5, color: 'rgba(71,85,105,0.45)', letterSpacing: '0.12em' }}>AGENT 识别结果</span>
                    <div className="flex-1 h-px" style={{ background: 'rgba(255,255,255,0.04)' }} />
                  </div>
                  {file.tasks.map((task, ti) => {
                    const sc = statusConfig[task.status as NotepadItem['status']];
                    return (
                      <div
                        key={ti}
                        className="flex items-start gap-2.5 px-2.5 py-2 rounded-lg"
                        style={{
                          background: task.status === 'done' ? 'rgba(255,255,255,0.015)' : 'rgba(255,255,255,0.03)',
                          border: `1px solid ${task.priority === 'high' ? 'rgba(251,146,60,0.12)' : 'rgba(255,255,255,0.04)'}`,
                        }}
                      >
                        {/* Status dot */}
                        <div className="flex items-center justify-center rounded-full flex-shrink-0 mt-0.5" style={{ width: 18, height: 18, background: `${sc.color}15`, border: `1px solid ${sc.color}25` }}>
                          <i className={sc.icon} style={{ fontSize: 9, color: sc.color, animation: task.status === 'processing' ? 'spin 1.5s linear infinite' : 'none' }} />
                        </div>
                        {/* Content */}
                        <div className="flex-1 min-w-0">
                          <div style={{ fontSize: 11.5, color: task.status === 'done' ? 'rgba(100,116,139,0.45)' : 'rgba(226,232,240,0.82)', textDecoration: task.status === 'done' ? 'line-through' : 'none', lineHeight: 1.4 }}>
                            {task.text}
                          </div>
                          <div className="flex items-center gap-2 mt-1">
                            <span style={{ fontSize: 9, color: priorityColor(task.priority) }}>
                              {task.date}
                            </span>
                            {task.tags.map(tag => (
                              <span key={tag} className="px-1.5 py-0.5 rounded" style={{ fontSize: 8, background: `${color}10`, color: `${color}90`, letterSpacing: '0.06em' }}>
                                {tag}
                              </span>
                            ))}
                          </div>
                        </div>
                        {/* Priority indicator */}
                        {task.priority === 'high' && (
                          <div className="flex-shrink-0" style={{ width: 3, height: 3, borderRadius: '50%', background: '#fb923c', boxShadow: '0 0 4px #fb923c', marginTop: 6 }} />
                        )}
                      </div>
                    );
                  })}

                  {/* Agent actions */}
                  <div className="flex items-center gap-1.5 mt-1.5 pt-2" style={{ borderTop: `1px solid ${color}08` }}>
                    <span style={{ fontSize: 8.5, color: 'rgba(71,85,105,0.4)', letterSpacing: '0.1em' }}>Agent 可执行</span>
                    {['提醒用户', '打开文件', '标记完成'].map(action => (
                      <button key={action} className="px-2 py-0.5 rounded cursor-pointer transition-all duration-150 whitespace-nowrap" style={{ fontSize: 8.5, background: `${color}0c`, border: `1px solid ${color}20`, color: `${color}90`, letterSpacing: '0.06em' }}
                        onMouseEnter={e => { (e.currentTarget as HTMLButtonElement).style.background = `${color}1a`; (e.currentTarget as HTMLButtonElement).style.color = color; }}
                        onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.background = `${color}0c`; (e.currentTarget as HTMLButtonElement).style.color = `${color}90`; }}
                      >
                        {action}
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ── Workflow Engine ──
function WorkflowEngine({ color, visible }: { color: string; visible: boolean }) {
  const [show, setShow] = useState(false);

  useEffect(() => {
    if (visible) { const t = setTimeout(() => setShow(true), 150); return () => clearTimeout(t); }
    setShow(false);
  }, [visible]);

  const workflows = [
    {
      name: '每日晨间巡检',
      schedule: '每天 09:00',
      status: 'active',
      lastRun: '今日 09:00',
      steps: [
        { label: '读取 tasks.md', done: true },
        { label: '识别今日任务', done: true },
        { label: '发送摘要提醒', done: true },
        { label: '打开优先文件', done: false },
      ],
    },
    {
      name: '周报自动整理',
      schedule: '每周一 09:30',
      status: 'scheduled',
      lastRun: '上周一',
      steps: [
        { label: '汇总本周任务', done: false },
        { label: '生成周报草稿', done: false },
        { label: '等待用户确认', done: false },
      ],
    },
  ];

  const statusStyle: Record<string, { color: string; label: string; icon: string }> = {
    active:    { color: '#34d399', label: '运行中', icon: 'ri-play-circle-line' },
    scheduled: { color: '#a78bfa', label: '已排期', icon: 'ri-calendar-line' },
    paused:    { color: '#64748b', label: '已暂停', icon: 'ri-pause-circle-line' },
  };

  return (
    <div style={{ opacity: show ? 1 : 0, transform: show ? 'translateY(0)' : 'translateY(6px)', transition: 'opacity 0.4s ease, transform 0.4s ease' }}>
      <div className="flex items-center gap-2 mb-3">
        <div className="w-4 h-px" style={{ background: `linear-gradient(to right, ${color}60, transparent)` }} />
        <i className="ri-flow-chart" style={{ fontSize: 9, color }} />
        <span style={{ fontSize: 9.5, color, letterSpacing: '0.18em', fontWeight: 600, opacity: 0.85 }}>
          工作流巡检
        </span>
      </div>

      <div className="flex flex-col gap-2">
        {workflows.map((wf, wi) => {
          const ss = statusStyle[wf.status];
          const doneCount = wf.steps.filter(s => s.done).length;
          return (
            <div key={wi} className="px-3 py-3 rounded-lg" style={{ background: 'rgba(255,255,255,0.025)', border: `1px solid rgba(255,255,255,0.06)`, opacity: show ? 1 : 0, transition: `opacity 0.35s ease ${wi * 0.1}s` }}>
              <div className="flex items-center justify-between mb-2.5">
                <div className="flex items-center gap-2">
                  <i className={ss.icon} style={{ fontSize: 11, color: ss.color }} />
                  <span style={{ fontSize: 11.5, color: 'rgba(226,232,240,0.85)', fontWeight: 500 }}>{wf.name}</span>
                </div>
                <div className="flex items-center gap-1.5">
                  <span style={{ fontSize: 8.5, color: ss.color, letterSpacing: '0.08em' }}>{ss.label}</span>
                  <div className="rounded-full" style={{ width: 4, height: 4, background: ss.color, boxShadow: `0 0 4px ${ss.color}`, animation: wf.status === 'active' ? 'notifPulse 2s ease-in-out infinite' : 'none' }} />
                </div>
              </div>

              {/* Schedule */}
              <div className="flex items-center gap-3 mb-2.5">
                <div className="flex items-center gap-1">
                  <i className="ri-time-line" style={{ fontSize: 9, color: 'rgba(71,85,105,0.5)' }} />
                  <span style={{ fontSize: 9, color: 'rgba(71,85,105,0.55)' }}>{wf.schedule}</span>
                </div>
                <div className="flex items-center gap-1">
                  <i className="ri-history-line" style={{ fontSize: 9, color: 'rgba(71,85,105,0.5)' }} />
                  <span style={{ fontSize: 9, color: 'rgba(71,85,105,0.55)' }}>上次 {wf.lastRun}</span>
                </div>
              </div>

              {/* Steps pipeline */}
              <div className="flex items-center gap-1">
                {wf.steps.map((step, si) => (
                  <div key={si} className="flex items-center flex-1">
                    <div className="flex flex-col items-center gap-1" style={{ flex: 1 }}>
                      <div className="rounded-full" style={{
                        width: step.done ? 8 : 6,
                        height: step.done ? 8 : 6,
                        background: step.done ? ss.color : 'rgba(255,255,255,0.1)',
                        boxShadow: step.done ? `0 0 6px ${ss.color}` : 'none',
                        border: step.done ? 'none' : '1px solid rgba(255,255,255,0.12)',
                        transition: 'all 0.3s ease',
                      }} />
                      <span style={{ fontSize: 7.5, color: step.done ? `${ss.color}90` : 'rgba(71,85,105,0.4)', textAlign: 'center', whiteSpace: 'nowrap', maxWidth: 52, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {step.label}
                      </span>
                    </div>
                    {si < wf.steps.length - 1 && (
                      <div style={{ height: 1, width: 8, background: step.done ? `${ss.color}40` : 'rgba(255,255,255,0.06)', marginBottom: 14, flexShrink: 0 }} />
                    )}
                  </div>
                ))}
              </div>

              {/* Progress */}
              <div className="mt-2.5">
                <div className="flex items-center justify-between mb-1">
                  <span style={{ fontSize: 8.5, color: 'rgba(71,85,105,0.45)', letterSpacing: '0.1em' }}>步骤进度</span>
                  <span style={{ fontSize: 8.5, color: ss.color }}>{doneCount}/{wf.steps.length}</span>
                </div>
                <div className="rounded-full overflow-hidden" style={{ height: 2, background: 'rgba(255,255,255,0.05)' }}>
                  <div style={{ height: '100%', width: `${(doneCount / wf.steps.length) * 100}%`, background: `linear-gradient(to right, ${ss.color}60, ${ss.color})`, borderRadius: 2, transition: 'width 0.8s ease' }} />
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ── Recurring Tasks ──
function RecurringTasks({ color, visible }: { color: string; visible: boolean }) {
  const [show, setShow] = useState(false);

  useEffect(() => {
    if (visible) { const t = setTimeout(() => setShow(true), 200); return () => clearTimeout(t); }
    setShow(false);
  }, [visible]);

  const recurring = [
    { text: '周报整理', freq: '每周一', streak: 6, nextRun: '下周一', keepAfterDone: false },
    { text: '月度复盘', freq: '每月 1 日', streak: 3, nextRun: '5 月 1 日', keepAfterDone: true },
    { text: '邮件归档', freq: '每天 09:00', streak: 14, nextRun: '明日 09:00', keepAfterDone: true },
  ];

  return (
    <div style={{ opacity: show ? 1 : 0, transform: show ? 'translateY(0)' : 'translateY(6px)', transition: 'opacity 0.4s ease, transform 0.4s ease' }}>
      <div className="flex items-center gap-2 mb-3">
        <div className="w-4 h-px" style={{ background: `linear-gradient(to right, ${color}60, transparent)` }} />
        <i className="ri-repeat-2-line" style={{ fontSize: 9, color }} />
        <span style={{ fontSize: 9.5, color, letterSpacing: '0.18em', fontWeight: 600, opacity: 0.85 }}>
          重复任务 · Agent 已记住
        </span>
      </div>

      <div className="flex flex-col gap-1.5">
        {recurring.map((item, i) => (
          <div
            key={i}
            className="flex items-center gap-3 px-3 py-2.5 rounded-lg"
            style={{
              background: 'rgba(255,255,255,0.025)',
              border: `1px solid rgba(255,255,255,0.05)`,
              opacity: show ? 1 : 0,
              transition: `opacity 0.35s ease ${i * 0.07}s`,
            }}
          >
            {/* Streak badge */}
            <div className="flex flex-col items-center flex-shrink-0" style={{ width: 32 }}>
              <div style={{ fontSize: 14, color, fontWeight: 700, lineHeight: 1 }}>{item.streak}</div>
              <div style={{ fontSize: 7.5, color: 'rgba(71,85,105,0.5)', letterSpacing: '0.06em' }}>连续</div>
            </div>

            <div className="flex-1 min-w-0">
              <div style={{ fontSize: 11.5, color: 'rgba(226,232,240,0.82)' }}>{item.text}</div>
              <div className="flex items-center gap-2 mt-0.5">
                <span style={{ fontSize: 9, color: 'rgba(71,85,105,0.55)' }}>{item.freq}</span>
                <span style={{ fontSize: 9, color: `${color}70` }}>→ {item.nextRun}</span>
              </div>
            </div>

            {/* Keep/done toggle */}
            <div
              className="flex items-center gap-1 px-2 py-1 rounded cursor-pointer"
              style={{ background: item.keepAfterDone ? `${color}10` : 'rgba(255,255,255,0.03)', border: `1px solid ${item.keepAfterDone ? color + '25' : 'rgba(255,255,255,0.06)'}` }}
            >
              <i className={item.keepAfterDone ? 'ri-bookmark-line' : 'ri-check-line'} style={{ fontSize: 9, color: item.keepAfterDone ? color : 'rgba(71,85,105,0.5)' }} />
              <span style={{ fontSize: 8, color: item.keepAfterDone ? color : 'rgba(71,85,105,0.5)', letterSpacing: '0.06em' }}>
                {item.keepAfterDone ? '保留' : '完成删除'}
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Async Collaboration ──
function AsyncCollab({ color, items, visible }: { color: string; items: NotepadItem[]; visible: boolean }) {
  const [show, setShow] = useState(false);

  useEffect(() => {
    if (visible) { const t = setTimeout(() => setShow(true), 120); return () => clearTimeout(t); }
    setShow(false);
  }, [visible, items]);

  if (!items.length) return null;

  return (
    <div style={{ opacity: show ? 1 : 0, transform: show ? 'translateY(0)' : 'translateY(6px)', transition: 'opacity 0.4s ease, transform 0.4s ease' }}>
      <div className="flex items-center gap-2 mb-3">
        <div className="w-4 h-px" style={{ background: `linear-gradient(to right, ${color}60, transparent)` }} />
        <i className="ri-sticky-note-line" style={{ fontSize: 9, color }} />
        <span style={{ fontSize: 9.5, color, letterSpacing: '0.18em', fontWeight: 600, opacity: 0.85 }}>
          便签 · 异步协作队列
        </span>
      </div>

      <div className="flex flex-col gap-2">
        {items.map((item, i) => {
          const cfg = statusConfig[item.status];
          return (
            <div
              key={item.id}
              className="flex items-center gap-3 px-3 py-2.5 rounded-lg cursor-pointer"
              style={{
                background: 'rgba(255,255,255,0.03)',
                border: `1px solid rgba(255,255,255,0.05)`,
                opacity: show ? 1 : 0,
                transform: show ? 'translateX(0)' : 'translateX(-8px)',
                transition: `opacity 0.35s ease ${i * 0.07}s, transform 0.35s ease ${i * 0.07}s`,
              }}
              onMouseEnter={e => { (e.currentTarget as HTMLDivElement).style.background = 'rgba(255,255,255,0.05)'; }}
              onMouseLeave={e => { (e.currentTarget as HTMLDivElement).style.background = 'rgba(255,255,255,0.03)'; }}
            >
              <div className="flex items-center justify-center rounded-full flex-shrink-0" style={{ width: 24, height: 24, background: `${cfg.color}18`, border: `1px solid ${cfg.color}30` }}>
                <i className={cfg.icon} style={{ fontSize: 11, color: cfg.color, animation: item.status === 'processing' ? 'spin 1.5s linear infinite' : 'none' }} />
              </div>
              <div className="flex-1 min-w-0">
                <div style={{ fontSize: 12, color: item.status === 'done' ? 'rgba(100,116,139,0.45)' : 'rgba(226,232,240,0.85)', textDecoration: item.status === 'done' ? 'line-through' : 'none', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  {item.text}
                </div>
                {item.time && <div style={{ fontSize: 9.5, color: 'rgba(71,85,105,0.6)', marginTop: 1 }}>{item.time}</div>}
              </div>
              {item.tag && (
                <div className="px-2 py-0.5 rounded-full flex-shrink-0" style={{ background: `${cfg.color}15`, fontSize: 9, color: cfg.color, letterSpacing: '0.06em' }}>
                  {item.tag}
                </div>
              )}
            </div>
          );
        })}
      </div>

      <style>{`
        @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
      `}</style>
    </div>
  );
}

// ── Main NotepadLayer ──
export default function NotepadLayer({ stateData, visible }: NotepadLayerProps) {
  const items = stateData.notepadItems ?? [];
  const isScheduled = stateData.key === 'scheduled_task';
  const isProcessing = stateData.key === 'notepad_processing';
  const isReminder = stateData.key === 'notepad_reminder';

  return (
    <div className="flex flex-col gap-4">
      {/* Scheduled task: show MD inspector + workflow */}
      {isScheduled && (
        <>
          <MdFileInspector color={stateData.accentColor} visible={visible} />
          <div style={{ height: 1, background: `${stateData.accentColor}10` }} />
          <WorkflowEngine color={stateData.accentColor} visible={visible} />
          <div style={{ height: 1, background: `${stateData.accentColor}10` }} />
          <RecurringTasks color={stateData.accentColor} visible={visible} />
        </>
      )}

      {/* Processing: show async collab queue + MD inspector */}
      {isProcessing && (
        <>
          <AsyncCollab color={stateData.accentColor} items={items} visible={visible} />
          <div style={{ height: 1, background: `${stateData.accentColor}10` }} />
          <MdFileInspector color={stateData.accentColor} visible={visible} />
        </>
      )}

      {/* Reminder: show recurring tasks + async collab */}
      {isReminder && (
        <>
          <RecurringTasks color={stateData.accentColor} visible={visible} />
          <div style={{ height: 1, background: `${stateData.accentColor}10` }} />
          <AsyncCollab color={stateData.accentColor} items={items} visible={visible} />
        </>
      )}

      {/* Fallback for other states */}
      {!isScheduled && !isProcessing && !isReminder && items.length > 0 && (
        <AsyncCollab color={stateData.accentColor} items={items} visible={visible} />
      )}
    </div>
  );
}
