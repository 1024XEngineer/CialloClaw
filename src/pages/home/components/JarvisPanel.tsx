import { useEffect, useState, useRef, useCallback } from 'react';
import type { AgentStateData, AgentStateKey, ModuleKey } from '@/mocks/agentStates';
import { agentStates, moduleGroups } from '@/mocks/agentStates';
import type { PlanetConfig } from './PlanetNode';
import FocusCard from './FocusCard';
import ProgressTrack from './ProgressTrack';
import ContextStream from './ContextStream';
import AnomalyLayer from './AnomalyLayer';
import NotepadLayer from './NotepadLayer';
import MirrorLayer from './MirrorLayer';
import SenseLayer from './SenseLayer';

interface JarvisPanelProps {
  planets: PlanetConfig[];
  activePlanetKey: ModuleKey;
  planetScreenX: number;
  planetScreenY: number;
  stateData: AgentStateData;
  currentStateKey: AgentStateKey;
  onClose: () => void;
  onStateChange: (key: AgentStateKey) => void;
  onModuleChange: (key: ModuleKey) => void;
  transitioning: boolean;
  onDetach: (moduleKey: ModuleKey, stateKey: AgentStateKey, x: number, y: number) => void;
}

const stateLabelMap: Record<AgentStateKey, string> = {
  standby: '待机', idle_present: '空闲在场', working: '推进中', highlight: '新进展',
  completing: '接近完成', done: '已完成', error_permission: '缺少权限',
  error_blocked: '步骤阻塞', error_missing_info: '缺少信息',
  notepad_processing: '便签处理', notepad_reminder: '重复提醒', scheduled_task: '定时巡检',
  mirror_summary: '周期总结', mirror_habit: '习惯洞察',
  sense_alert: '系统预警', sense_suggestion: '系统建议',
};

const PANEL_MIN_W = 380;
const PANEL_MAX_W = 720;
const PANEL_MIN_H = 320;
const PANEL_MAX_H = 820;

// ── Quick action buttons per module ──
const moduleActions: Record<ModuleKey, { icon: string; label: string; color?: string }[]> = {
  task: [
    { icon: 'ri-play-circle-line', label: '继续推进' },
    { icon: 'ri-file-text-line', label: '查看草稿' },
    { icon: 'ri-share-line', label: '分享结果' },
  ],
  notepad: [
    { icon: 'ri-add-line', label: '新建便签' },
    { icon: 'ri-robot-line', label: '让我处理' },
    { icon: 'ri-calendar-line', label: '排期' },
  ],
  mirror: [
    { icon: 'ri-download-line', label: '导出报告' },
    { icon: 'ri-share-line', label: '分享洞察' },
    { icon: 'ri-settings-3-line', label: '调整偏好' },
  ],
  sense: [
    { icon: 'ri-pause-circle-line', label: '暂缓任务' },
    { icon: 'ri-refresh-line', label: '刷新数据' },
    { icon: 'ri-notification-line', label: '设置提醒' },
  ],
};

// ── Status timeline for task module ──
function StatusTimeline({ planet, moduleStates, currentStateKey, onStateChange }: {
  planet: PlanetConfig;
  moduleStates: AgentStateKey[];
  currentStateKey: AgentStateKey;
  onStateChange: (key: AgentStateKey) => void;
}) {
  const currentIdx = moduleStates.indexOf(currentStateKey);
  const taskFlow = moduleStates.filter(k => !k.startsWith('error'));
  const errorStates = moduleStates.filter(k => k.startsWith('error'));

  return (
    <div className="px-5 py-3" style={{ borderBottom: `1px solid ${planet.color}10` }}>
      {/* Main flow */}
      <div className="flex items-center gap-1 mb-2">
        {taskFlow.map((key, i) => {
          const isActive = key === currentStateKey;
          const isDone = taskFlow.indexOf(currentStateKey) > i && !currentStateKey.startsWith('error');
          const isErr = key.startsWith('error');
          if (isErr) return null;
          return (
            <div key={key} className="flex items-center flex-1">
              <button
                onClick={() => onStateChange(key)}
                className="flex flex-col items-center gap-1 cursor-pointer group"
                style={{ flex: 1 }}
              >
                <div
                  className="rounded-full transition-all duration-300"
                  style={{
                    width: isActive ? 10 : 6,
                    height: isActive ? 10 : 6,
                    background: isActive
                      ? planet.color
                      : isDone
                      ? `${planet.color}60`
                      : 'rgba(255,255,255,0.1)',
                    boxShadow: isActive ? `0 0 10px ${planet.color}` : 'none',
                    animation: isActive ? 'notifPulse 2s ease-in-out infinite' : 'none',
                  }}
                />
                <span
                  style={{
                    fontSize: 8,
                    color: isActive ? planet.color : isDone ? `${planet.color}60` : 'rgba(71,85,105,0.4)',
                    letterSpacing: '0.04em',
                    whiteSpace: 'nowrap',
                    transition: 'color 0.3s ease',
                  }}
                >
                  {stateLabelMap[key]}
                </span>
              </button>
              {i < taskFlow.length - 1 && (
                <div
                  style={{
                    height: 1,
                    width: 12,
                    background: isDone ? `${planet.color}40` : 'rgba(255,255,255,0.06)',
                    marginBottom: 14,
                    flexShrink: 0,
                    transition: 'background 0.4s ease',
                  }}
                />
              )}
            </div>
          );
        })}
      </div>

      {/* Error states row */}
      {errorStates.length > 0 && (
        <div className="flex items-center gap-2 mt-1">
          <span style={{ fontSize: 8, color: 'rgba(71,85,105,0.4)', letterSpacing: '0.1em' }}>异常</span>
          <div style={{ width: 1, height: 10, background: 'rgba(255,255,255,0.06)' }} />
          {errorStates.map(key => {
            const isActive = key === currentStateKey;
            return (
              <button
                key={key}
                onClick={() => onStateChange(key)}
                className="px-2 py-0.5 rounded cursor-pointer transition-all duration-200 whitespace-nowrap"
                style={{
                  fontSize: 8.5,
                  background: isActive ? 'rgba(251,113,133,0.14)' : 'rgba(255,255,255,0.03)',
                  color: isActive ? '#fb7185' : 'rgba(71,85,105,0.45)',
                  border: isActive ? '1px solid rgba(251,113,133,0.3)' : '1px solid rgba(255,255,255,0.05)',
                  letterSpacing: '0.06em',
                }}
              >
                {stateLabelMap[key]}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

// ── Module switcher tabs ──
function ModuleTabs({ planets, activePlanetKey, onModuleChange }: {
  planets: PlanetConfig[];
  activePlanetKey: ModuleKey;
  onModuleChange: (key: ModuleKey) => void;
}) {
  return (
    <div className="flex items-center gap-1 px-4 py-2.5" style={{ borderBottom: `1px solid rgba(255,255,255,0.05)` }}>
      {moduleGroups.map(g => {
        const p = planets.find(pl => pl.key === g.key)!;
        const isAct = g.key === activePlanetKey;
        return (
          <button
            key={g.key}
            onClick={() => onModuleChange(g.key)}
            className="flex items-center gap-1.5 px-2.5 py-1.5 rounded cursor-pointer transition-all duration-200 whitespace-nowrap"
            style={{
              background: isAct ? `${p.color}14` : 'transparent',
              border: isAct ? `1px solid ${p.color}30` : '1px solid transparent',
              color: isAct ? p.color : 'rgba(71,85,105,0.5)',
              fontSize: 10,
              letterSpacing: '0.08em',
            }}
          >
            <i className={p.icon} style={{ fontSize: 10 }} />
            <span>{g.label}</span>
          </button>
        );
      })}
      <div className="flex-1" />
      <div className="flex items-center gap-1" style={{ opacity: 0.25 }}>
        <i className="ri-time-line" style={{ fontSize: 9, color: 'rgba(148,163,184,0.5)' }} />
        <span style={{ fontSize: 8, color: 'rgba(148,163,184,0.5)', letterSpacing: '0.1em' }}>长按拖出</span>
      </div>
    </div>
  );
}

// ── Quick actions bar ──
function QuickActions({ planet, moduleKey, visible }: {
  planet: PlanetConfig;
  moduleKey: ModuleKey;
  visible: boolean;
}) {
  const actions = moduleActions[moduleKey] ?? [];
  const [show, setShow] = useState(false);

  useEffect(() => {
    if (visible) {
      const t = setTimeout(() => setShow(true), 500);
      return () => clearTimeout(t);
    }
    setShow(false);
  }, [visible, moduleKey]);

  return (
    <div
      className="flex items-center gap-2 px-5 py-3"
      style={{
        borderTop: `1px solid ${planet.color}10`,
        opacity: show ? 1 : 0,
        transform: show ? 'translateY(0)' : 'translateY(4px)',
        transition: 'opacity 0.4s ease, transform 0.4s ease',
      }}
    >
      {actions.map((action, i) => (
        <button
          key={i}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded cursor-pointer transition-all duration-200 whitespace-nowrap"
          style={{
            background: 'rgba(255,255,255,0.04)',
            border: `1px solid ${planet.color}18`,
            color: 'rgba(148,163,184,0.7)',
            fontSize: 10.5,
            letterSpacing: '0.06em',
          }}
          onMouseEnter={e => {
            (e.currentTarget as HTMLButtonElement).style.background = `${planet.color}14`;
            (e.currentTarget as HTMLButtonElement).style.color = planet.color;
            (e.currentTarget as HTMLButtonElement).style.borderColor = `${planet.color}35`;
          }}
          onMouseLeave={e => {
            (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.04)';
            (e.currentTarget as HTMLButtonElement).style.color = 'rgba(148,163,184,0.7)';
            (e.currentTarget as HTMLButtonElement).style.borderColor = `${planet.color}18`;
          }}
        >
          <i className={action.icon} style={{ fontSize: 11 }} />
          <span>{action.label}</span>
        </button>
      ))}
    </div>
  );
}

// ── State selector for non-task modules ──
function StateSelector({ planet, moduleStates, currentStateKey, onStateChange }: {
  planet: PlanetConfig;
  moduleStates: AgentStateKey[];
  currentStateKey: AgentStateKey;
  onStateChange: (key: AgentStateKey) => void;
}) {
  return (
    <div className="flex items-center gap-2 px-5 py-2.5" style={{ borderBottom: `1px solid ${planet.color}10` }}>
      {moduleStates.map(key => {
        const isAct = key === currentStateKey;
        return (
          <button
            key={key}
            onClick={() => onStateChange(key)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded cursor-pointer transition-all duration-200 whitespace-nowrap"
            style={{
              background: isAct ? `${planet.color}16` : 'rgba(255,255,255,0.03)',
              border: isAct ? `1px solid ${planet.color}35` : '1px solid rgba(255,255,255,0.05)',
              color: isAct ? planet.color : 'rgba(100,116,139,0.55)',
              fontSize: 10.5,
              letterSpacing: '0.08em',
            }}
          >
            {isAct && (
              <div
                className="rounded-full"
                style={{ width: 4, height: 4, background: planet.color, boxShadow: `0 0 5px ${planet.color}`, animation: 'notifPulse 2s ease-in-out infinite' }}
              />
            )}
            <span>{stateLabelMap[key]}</span>
          </button>
        );
      })}
    </div>
  );
}

export default function JarvisPanel({
  planets, activePlanetKey, planetScreenX, planetScreenY,
  stateData, currentStateKey, onClose, onStateChange, onModuleChange, transitioning, onDetach,
}: JarvisPanelProps) {
  const planet = planets.find(p => p.key === activePlanetKey)!;

  const [phase, setPhase] = useState<'entering' | 'visible' | 'exiting'>('entering');
  const [contentVisible, setContentVisible] = useState(false);
  const [scanLine, setScanLine] = useState(0);
  const [glitchActive, setGlitchActive] = useState(false);

  // Resize state — with per-module memory
  const STORAGE_KEY = `jarvis_panel_size_${activePlanetKey}`;
  const savedSize = (() => {
    try { return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? 'null'); } catch (_e) { return null; }
  })();
  const [panelW, setPanelW] = useState<number>(savedSize?.w ?? 520);
  const [panelH, setPanelH] = useState<number | null>(savedSize?.h ?? null);
  const resizingRef = useRef<{ edge: 'right' | 'left' | 'bottom' | 'br' | 'bl'; startX: number; startY: number; startW: number; startH: number } | null>(null);
  const [isResizing, setIsResizing] = useState(false);
  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Long-press detach state
  const [longPressProgress, setLongPressProgress] = useState(0);
  const [detachDrag, setDetachDrag] = useState(false);
  const [detachPos, setDetachPos] = useState({ x: 0, y: 0 });
  const [detachPreview, setDetachPreview] = useState(false);
  const longPressTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const longPressProgressRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const isLongPressRef = useRef(false);
  const detachStartRef = useRef({ x: 0, y: 0 });
  const detachActiveRef = useRef(false);

  // Swipe for module/state switching
  const [swipeDeltaY, setSwipeDeltaY] = useState(0);
  const [isSwipingY, setIsSwipingY] = useState(false);
  const [slideOutY, setSlideOutY] = useState<'up' | 'down' | null>(null);
  const [swipeDeltaX, setSwipeDeltaX] = useState(0);
  const [isSwipingX, setIsSwipingX] = useState(false);
  const [slideOutX, setSlideOutX] = useState<'left' | 'right' | null>(null);

  const panelRef = useRef<HTMLDivElement>(null);
  const scanRef = useRef<number>(0);
  const startRef = useRef(Date.now());
  const swipeStartX = useRef(0);
  const swipeStartY = useRef(0);
  const swipeLocked = useRef<'h' | 'v' | null>(null);
  const swipeActive = useRef(false);

  const moduleOrder = moduleGroups.map(g => g.key);
  const moduleIndex = moduleOrder.indexOf(activePlanetKey);
  const moduleStates = moduleGroups.find(g => g.key === activePlanetKey)?.states ?? [];
  const stateIndex = moduleStates.indexOf(currentStateKey);

  const isError = currentStateKey.startsWith('error');
  const isTask = stateData.module === 'task';
  const isNotepad = stateData.module === 'notepad';
  const isMirror = stateData.module === 'mirror';
  const isSense = stateData.module === 'sense';

  const screenW = window.innerWidth;
  const isOnLeft = planetScreenX < screenW / 2;
  const panelSide: 'left' | 'right' = isOnLeft ? 'right' : 'left';

  useEffect(() => {
    const t1 = setTimeout(() => setPhase('visible'), 80);
    const t2 = setTimeout(() => setContentVisible(true), 320);
    setGlitchActive(true);
    const tg = setTimeout(() => setGlitchActive(false), 400);
    return () => { clearTimeout(t1); clearTimeout(t2); clearTimeout(tg); };
  }, []);

  useEffect(() => {
    setGlitchActive(true);
    const t = setTimeout(() => setGlitchActive(false), 300);
    return () => clearTimeout(t);
  }, [activePlanetKey]);

  useEffect(() => {
    const animate = () => {
      const elapsed = (Date.now() - startRef.current) / 1000;
      setScanLine((elapsed * 0.35) % 1);
      scanRef.current = requestAnimationFrame(animate);
    };
    scanRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(scanRef.current);
  }, []);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (isResizing) return;
      if (panelRef.current && !panelRef.current.contains(e.target as Node)) {
        handleClose();
      }
    };
    const t = setTimeout(() => document.addEventListener('mousedown', handler), 120);
    return () => { clearTimeout(t); document.removeEventListener('mousedown', handler); };
  }, [isResizing]);

  // Resize handlers
  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (!resizingRef.current) return;
      const { edge, startX, startY, startW, startH } = resizingRef.current;
      const dx = e.clientX - startX;
      const dy = e.clientY - startY;
      let newW = panelW;
      let newH = panelH;
      if (edge === 'right' || edge === 'br') {
        newW = Math.max(PANEL_MIN_W, Math.min(PANEL_MAX_W, startW + dx));
        setPanelW(newW);
      }
      if (edge === 'left' || edge === 'bl') {
        newW = Math.max(PANEL_MIN_W, Math.min(PANEL_MAX_W, startW - dx));
        setPanelW(newW);
      }
      if (edge === 'bottom' || edge === 'br' || edge === 'bl') {
        const curH = startH === 0 ? (panelRef.current?.offsetHeight ?? 500) : startH;
        newH = Math.max(PANEL_MIN_H, Math.min(PANEL_MAX_H, curH + dy));
        setPanelH(newH);
      }
      // Debounced save
      if (saveTimerRef.current) clearTimeout(saveTimerRef.current);
      saveTimerRef.current = setTimeout(() => {
        try { localStorage.setItem(STORAGE_KEY, JSON.stringify({ w: newW, h: newH })); } catch (_e) { /* ignore */ }
      }, 400);
    };
    const onUp = () => {
      if (!resizingRef.current) return;
      resizingRef.current = null;
      setIsResizing(false);
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    return () => { window.removeEventListener('mousemove', onMove); window.removeEventListener('mouseup', onUp); };
  }, []);

  const startResize = useCallback((e: React.MouseEvent, edge: 'right' | 'left' | 'bottom' | 'br' | 'bl') => {
    e.preventDefault();
    e.stopPropagation();
    resizingRef.current = {
      edge,
      startX: e.clientX,
      startY: e.clientY,
      startW: panelW,
      startH: panelRef.current?.offsetHeight ?? 0,
    };
    setIsResizing(true);
  }, [panelW]);

  const handleClose = () => {
    setPhase('exiting');
    setContentVisible(false);
    setTimeout(onClose, 350);
  };

  const handleStateChange = useCallback((key: AgentStateKey) => {
    if (key === currentStateKey) return;
    setGlitchActive(true);
    setTimeout(() => setGlitchActive(false), 280);
    onStateChange(key);
  }, [currentStateKey, onStateChange]);

  const navigateState = useCallback((offset: number) => {
    const next = stateIndex + offset;
    if (next < 0 || next >= moduleStates.length) return false;
    handleStateChange(moduleStates[next]);
    return true;
  }, [stateIndex, moduleStates, handleStateChange]);

  const navigateModule = useCallback((offset: number) => {
    const next = moduleIndex + offset;
    if (next < 0 || next >= moduleOrder.length) return false;
    onModuleChange(moduleOrder[next]);
    return true;
  }, [moduleIndex, moduleOrder, onModuleChange]);

  // Long press
  const startLongPress = useCallback((e: React.PointerEvent) => {
    isLongPressRef.current = false;
    setLongPressProgress(0);
    longPressProgressRef.current = setInterval(() => {
      setLongPressProgress(prev => {
        const next = prev + 100 / 8;
        if (next >= 100) {
          if (longPressProgressRef.current) clearInterval(longPressProgressRef.current);
          return 100;
        }
        return next;
      });
    }, 100);
    longPressTimerRef.current = setTimeout(() => {
      isLongPressRef.current = true;
      detachStartRef.current = { x: e.clientX, y: e.clientY };
      detachActiveRef.current = true;
    }, 800);
  }, []);

  const cancelLongPress = useCallback(() => {
    if (longPressTimerRef.current) clearTimeout(longPressTimerRef.current);
    if (longPressProgressRef.current) clearInterval(longPressProgressRef.current);
    setLongPressProgress(0);
    if (!isLongPressRef.current) {
      detachActiveRef.current = false;
    }
  }, []);

  const onSwipePointerDown = useCallback((e: React.PointerEvent) => {
    if ((e.target as HTMLElement).closest('button')) return;
    if (isLongPressRef.current) return;
    swipeStartX.current = e.clientX;
    swipeStartY.current = e.clientY;
    swipeLocked.current = null;
    swipeActive.current = true;
    setSwipeDeltaX(0);
    setSwipeDeltaY(0);
    setIsSwipingX(false);
    setIsSwipingY(false);
    startLongPress(e);
  }, [startLongPress]);

  const onSwipePointerMove = useCallback((e: React.PointerEvent) => {
    if (detachActiveRef.current && isLongPressRef.current) {
      const dx = e.clientX - detachStartRef.current.x;
      const dy = e.clientY - detachStartRef.current.y;
      const dist = Math.sqrt(dx * dx + dy * dy);
      if (dist > 8) {
        setDetachDrag(true);
        setDetachPos({ x: e.clientX, y: e.clientY });
        setDetachPreview(dist > 80);
      }
      return;
    }
    if (!swipeActive.current) return;
    const dx = e.clientX - swipeStartX.current;
    const dy = e.clientY - swipeStartY.current;
    const dist = Math.sqrt(dx * dx + dy * dy);
    if (dist > 10) cancelLongPress();
    if (!swipeLocked.current) {
      if (dist > 8) swipeLocked.current = Math.abs(dx) > Math.abs(dy) ? 'h' : 'v';
      return;
    }
    e.preventDefault();
    const maxDrag = 100;
    if (swipeLocked.current === 'h') {
      setIsSwipingX(true);
      const canL = stateIndex < moduleStates.length - 1;
      const canR = stateIndex > 0;
      let c = dx;
      if (dx < 0 && !canL) c = dx * 0.15;
      if (dx > 0 && !canR) c = dx * 0.15;
      setSwipeDeltaX(Math.max(-maxDrag, Math.min(maxDrag, c)));
    } else {
      setIsSwipingY(true);
      const canUp = moduleIndex < moduleOrder.length - 1;
      const canDown = moduleIndex > 0;
      let c = dy;
      if (dy < 0 && !canUp) c = dy * 0.15;
      if (dy > 0 && !canDown) c = dy * 0.15;
      setSwipeDeltaY(Math.max(-maxDrag, Math.min(maxDrag, c)));
    }
  }, [stateIndex, moduleStates.length, moduleIndex, moduleOrder.length, cancelLongPress]);

  const onSwipePointerUp = useCallback((e: React.PointerEvent) => {
    cancelLongPress();
    if (detachActiveRef.current && isLongPressRef.current && detachPreview) {
      onDetach(activePlanetKey, currentStateKey, e.clientX - 240, e.clientY - 40);
      setDetachDrag(false);
      setDetachPreview(false);
      detachActiveRef.current = false;
      isLongPressRef.current = false;
      swipeActive.current = false;
      return;
    }
    setDetachDrag(false);
    setDetachPreview(false);
    detachActiveRef.current = false;
    isLongPressRef.current = false;
    if (!swipeActive.current) return;
    swipeActive.current = false;
    const threshold = 48;
    if (isSwipingX && Math.abs(swipeDeltaX) >= threshold) {
      const dir = swipeDeltaX < 0 ? 'left' : 'right';
      const ok = navigateState(dir === 'left' ? 1 : -1);
      if (ok) {
        setSlideOutX(dir);
        setTimeout(() => { setSlideOutX(null); setSwipeDeltaX(0); setIsSwipingX(false); }, 220);
        return;
      }
    }
    if (isSwipingY && Math.abs(swipeDeltaY) >= threshold) {
      const dir = swipeDeltaY < 0 ? 'up' : 'down';
      const ok = navigateModule(dir === 'up' ? 1 : -1);
      if (ok) {
        setSlideOutY(dir);
        setTimeout(() => { setSlideOutY(null); setSwipeDeltaY(0); setIsSwipingY(false); }, 220);
        return;
      }
    }
    setSwipeDeltaX(0);
    setSwipeDeltaY(0);
    setIsSwipingX(false);
    setIsSwipingY(false);
  }, [isSwipingX, swipeDeltaX, isSwipingY, swipeDeltaY, navigateState, navigateModule,
      detachPreview, activePlanetKey, currentStateKey, onDetach, cancelLongPress]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'ArrowRight') navigateState(1);
      if (e.key === 'ArrowLeft') navigateState(-1);
      if (e.key === 'ArrowDown') navigateModule(1);
      if (e.key === 'ArrowUp') navigateModule(-1);
      if (e.key === 'Escape') handleClose();
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [navigateState, navigateModule]);

  const contentTransformX = (() => {
    if (slideOutX === 'left') return 'translateX(-55px)';
    if (slideOutX === 'right') return 'translateX(55px)';
    if (isSwipingX) return `translateX(${swipeDeltaX * 0.3}px)`;
    return 'translateX(0)';
  })();

  const contentTransformY = (() => {
    if (slideOutY === 'up') return 'translateY(-40px)';
    if (slideOutY === 'down') return 'translateY(40px)';
    if (isSwipingY) return `translateY(${swipeDeltaY * 0.3}px)`;
    if (transitioning) return 'translateY(5px)';
    return 'translateY(0)';
  })();

  const contentOpacity = (() => {
    if (slideOutX || slideOutY) return 0;
    if (transitioning) return 0;
    if (isSwipingX) return 1 - Math.abs(swipeDeltaX) / 160;
    if (isSwipingY) return 1 - Math.abs(swipeDeltaY) / 140;
    return 1;
  })();

  const entryTransform = phase === 'entering'
    ? `scale(0.9) translateX(${panelSide === 'right' ? '-20px' : '20px'})`
    : phase === 'exiting'
    ? `scale(0.94) translateX(${panelSide === 'right' ? '-12px' : '12px'})`
    : 'scale(1) translateX(0)';

  const panelLeft = panelSide === 'right'
    ? Math.min(planetScreenX + 44, screenW - panelW - 28)
    : Math.max(planetScreenX - panelW - 44, 28);

  const hasPrevModule = moduleIndex > 0;
  const hasNextModule = moduleIndex < moduleOrder.length - 1;

  return (
    <>
      {/* Detach ghost preview */}
      {detachDrag && (
        <div
          className="pointer-events-none"
          style={{
            position: 'fixed',
            left: detachPos.x - 200,
            top: detachPos.y - 36,
            width: 400,
            zIndex: 200,
            opacity: detachPreview ? 0.9 : 0.45,
            transform: `scale(${detachPreview ? 0.97 : 0.9})`,
            transition: 'opacity 0.15s ease, transform 0.15s ease',
          }}
        >
          <div style={{
            background: 'rgba(3,5,12,0.97)',
            border: `1px solid ${planet.color}45`,
            borderRadius: 4,
            padding: '12px 16px',
            boxShadow: `0 0 50px ${planet.glow}`,
          }}>
            <div className="flex items-center gap-2 mb-2">
              <div className="rounded-full" style={{ width: 7, height: 7, background: planet.color, boxShadow: `0 0 8px ${planet.color}` }} />
              <span style={{ fontSize: 10, color: planet.color, letterSpacing: '0.18em', fontWeight: 600 }}>
                {planet.label.toUpperCase()} · {stateLabelMap[currentStateKey]}
              </span>
            </div>
            <div style={{ fontSize: 12, color: 'rgba(203,213,225,0.6)', lineHeight: 1.5 }}>
              {detachPreview ? '松手以独立展示' : '继续拖动以分离'}
            </div>
            {detachPreview && (
              <div className="flex items-center gap-1.5 mt-2">
                <i className="ri-external-link-line" style={{ fontSize: 11, color: planet.color }} />
                <span style={{ fontSize: 9.5, color: planet.color, letterSpacing: '0.1em' }}>将在此处创建独立窗口</span>
              </div>
            )}
          </div>
        </div>
      )}

      <div
        ref={panelRef}
        style={{
          position: 'fixed',
          top: '50%',
          left: panelLeft,
          transform: 'translateY(-50%)',
          zIndex: 100,
          pointerEvents: 'auto',
          width: panelW,
          height: panelH ?? undefined,
          userSelect: isResizing ? 'none' : undefined,
        }}
      >
        <div style={{
          position: 'relative',
          opacity: phase === 'entering' ? 0 : phase === 'exiting' ? 0 : 1,
          transform: entryTransform,
          transition: 'opacity 0.35s cubic-bezier(0.16,1,0.3,1), transform 0.35s cubic-bezier(0.16,1,0.3,1)',
          filter: glitchActive ? `drop-shadow(0 0 16px ${planet.color}90)` : 'none',
        }}>
          {/* Corner brackets */}
          {(['tl','tr','bl','br'] as const).map(corner => (
            <div key={corner} className="absolute pointer-events-none" style={{
              top: corner.startsWith('t') ? -1 : undefined,
              bottom: corner.startsWith('b') ? -1 : undefined,
              left: corner.endsWith('l') ? -1 : undefined,
              right: corner.endsWith('r') ? -1 : undefined,
              zIndex: 10,
            }}>
              {corner === 'tl' && <><div style={{ width: 20, height: 2, background: planet.color, opacity: 0.9 }} /><div style={{ width: 2, height: 20, background: planet.color, opacity: 0.9 }} /></>}
              {corner === 'tr' && <><div style={{ width: 20, height: 2, background: planet.color, opacity: 0.9, marginLeft: 'auto' }} /><div style={{ width: 2, height: 20, background: planet.color, opacity: 0.9, marginLeft: 'auto' }} /></>}
              {corner === 'bl' && <><div style={{ width: 2, height: 20, background: planet.color, opacity: 0.9 }} /><div style={{ width: 20, height: 2, background: planet.color, opacity: 0.9 }} /></>}
              {corner === 'br' && <><div style={{ width: 2, height: 20, background: planet.color, opacity: 0.9, marginLeft: 'auto' }} /><div style={{ width: 20, height: 2, background: planet.color, opacity: 0.9, marginLeft: 'auto' }} /></>}
            </div>
          ))}

          <div style={{
            background: 'rgba(3,5,12,0.97)',
            border: `1px solid ${planet.color}28`,
            borderRadius: 4,
            backdropFilter: 'blur(60px)',
            overflow: 'hidden',
            position: 'relative',
            boxShadow: `0 0 0 1px ${planet.color}10 inset, 0 40px 80px rgba(0,0,0,0.9), 0 0 70px ${planet.glow}`,
          }}>
            {/* Scan line */}
            <div className="absolute inset-0 pointer-events-none" style={{
              background: `linear-gradient(to bottom, transparent ${scanLine * 100 - 2}%, ${planet.color}08 ${scanLine * 100}%, transparent ${scanLine * 100 + 3}%)`,
              zIndex: 5,
            }} />

            {/* Grid lines */}
            <div className="absolute inset-0 pointer-events-none" style={{
              backgroundImage: `repeating-linear-gradient(to bottom, transparent, transparent 39px, ${planet.color}04 39px, ${planet.color}04 40px)`,
              zIndex: 2,
            }} />

            {/* Top ambient */}
            <div className="absolute top-0 left-0 right-0 pointer-events-none" style={{
              height: 80, background: `linear-gradient(to bottom, ${planet.glow} 0%, transparent 100%)`,
              opacity: 0.1, zIndex: 3,
            }} />

            {/* ── Module tabs ── */}
            <ModuleTabs
              planets={planets}
              activePlanetKey={activePlanetKey}
              onModuleChange={onModuleChange}
            />

            {/* ── Header ── */}
            <div className="relative flex items-center justify-between px-5 py-3" style={{ borderBottom: `1px solid ${planet.color}18`, zIndex: 10 }}>
              <div className="flex items-center gap-3">
                <div className="flex items-center justify-center rounded-full" style={{
                  width: 36, height: 36,
                  background: `radial-gradient(circle at 35% 30%, ${planet.color}cc 0%, ${planet.color}33 100%)`,
                  border: `1px solid ${planet.color}55`,
                  boxShadow: `0 0 18px ${planet.glow}`,
                }}>
                  <i className={planet.icon} style={{ fontSize: 15, color: 'rgba(255,255,255,0.95)' }} />
                </div>
                <div>
                  <div style={{ fontSize: 13, color: planet.color, letterSpacing: '0.2em', fontWeight: 600 }}>
                    {planet.label.toUpperCase()}
                  </div>
                  <div style={{ fontSize: 9.5, color: 'rgba(148,163,184,0.5)', letterSpacing: '0.1em', marginTop: 1 }}>
                    {planet.description}
                  </div>
                </div>
              </div>

              <div className="flex items-center gap-3">
                {/* Live indicator */}
                <div className="flex items-center gap-1.5 px-2.5 py-1 rounded" style={{
                  background: isError ? 'rgba(251,113,133,0.08)' : `${planet.color}08`,
                  border: `1px solid ${isError ? 'rgba(251,113,133,0.2)' : planet.color + '20'}`,
                }}>
                  <div className="rounded-full" style={{
                    width: 5, height: 5,
                    background: isError ? '#fb7185' : planet.color,
                    boxShadow: `0 0 6px ${isError ? '#fb7185' : planet.color}`,
                    animation: 'notifPulse 2s ease-in-out infinite',
                  }} />
                  <span style={{ fontSize: 9, color: isError ? '#fb7185' : planet.color, letterSpacing: '0.16em' }}>
                    {isError ? 'BLOCKED' : 'LIVE'}
                  </span>
                </div>

                {/* Module nav arrows */}
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => navigateModule(-1)}
                    disabled={!hasPrevModule}
                    className="flex items-center justify-center rounded cursor-pointer transition-all duration-150"
                    style={{ width: 22, height: 22, background: hasPrevModule ? `${planet.color}12` : 'transparent', border: `1px solid ${hasPrevModule ? planet.color + '30' : 'transparent'}`, color: hasPrevModule ? planet.color : 'rgba(71,85,105,0.2)', opacity: hasPrevModule ? 1 : 0.3 }}
                  >
                    <i className="ri-arrow-up-s-line" style={{ fontSize: 13 }} />
                  </button>
                  <button
                    onClick={() => navigateModule(1)}
                    disabled={!hasNextModule}
                    className="flex items-center justify-center rounded cursor-pointer transition-all duration-150"
                    style={{ width: 22, height: 22, background: hasNextModule ? `${planet.color}12` : 'transparent', border: `1px solid ${hasNextModule ? planet.color + '30' : 'transparent'}`, color: hasNextModule ? planet.color : 'rgba(71,85,105,0.2)', opacity: hasNextModule ? 1 : 0.3 }}
                  >
                    <i className="ri-arrow-down-s-line" style={{ fontSize: 13 }} />
                  </button>
                </div>

                <button
                  onClick={handleClose}
                  className="flex items-center justify-center rounded cursor-pointer transition-all duration-200"
                  style={{ width: 26, height: 26, background: 'rgba(255,255,255,0.04)', border: `1px solid ${planet.color}22`, color: 'rgba(148,163,184,0.6)' }}
                  onMouseEnter={e => { (e.currentTarget as HTMLButtonElement).style.background = `${planet.color}18`; (e.currentTarget as HTMLButtonElement).style.color = planet.color; }}
                  onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.04)'; (e.currentTarget as HTMLButtonElement).style.color = 'rgba(148,163,184,0.6)'; }}
                >
                  <i className="ri-close-line" style={{ fontSize: 14 }} />
                </button>
              </div>
            </div>

            {/* ── State navigation: task uses timeline, others use pills ── */}
            {isTask ? (
              <StatusTimeline
                planet={planet}
                moduleStates={moduleStates}
                currentStateKey={currentStateKey}
                onStateChange={handleStateChange}
              />
            ) : (
              <StateSelector
                planet={planet}
                moduleStates={moduleStates}
                currentStateKey={currentStateKey}
                onStateChange={handleStateChange}
              />
            )}

            {/* Long press progress bar */}
            {longPressProgress > 0 && longPressProgress < 100 && (
              <div style={{ height: 2, background: 'rgba(255,255,255,0.04)', position: 'relative', zIndex: 10 }}>
                <div style={{
                  height: '100%',
                  width: `${longPressProgress}%`,
                  background: `linear-gradient(to right, ${planet.color}60, ${planet.color})`,
                  transition: 'width 0.1s linear',
                }} />
              </div>
            )}

            {/* Swipe edge hints */}
            {isSwipingY && swipeDeltaY < -20 && hasNextModule && (
              <div className="absolute top-0 left-0 right-0 pointer-events-none" style={{ height: 36, background: `linear-gradient(to bottom, ${planet.color}18, transparent)`, zIndex: 8, opacity: Math.min(Math.abs(swipeDeltaY) / 70, 1) }} />
            )}
            {isSwipingY && swipeDeltaY > 20 && hasPrevModule && (
              <div className="absolute bottom-0 left-0 right-0 pointer-events-none" style={{ height: 36, background: `linear-gradient(to top, ${planet.color}18, transparent)`, zIndex: 8, opacity: Math.min(Math.abs(swipeDeltaY) / 70, 1) }} />
            )}

            {/* ── Content body ── */}
            <div
              className="relative"
              style={{ zIndex: 10, cursor: (isSwipingX || isSwipingY) ? 'grabbing' : 'default', userSelect: 'none', touchAction: 'none' }}
              onPointerDown={onSwipePointerDown}
              onPointerMove={onSwipePointerMove}
              onPointerUp={onSwipePointerUp}
              onPointerLeave={onSwipePointerUp}
            >
              {/* Swipe preview badges */}
              {isSwipingY && Math.abs(swipeDeltaY) > 28 && (
                <div className="absolute inset-0 flex items-center justify-center pointer-events-none" style={{ zIndex: 20 }}>
                  <div className="flex items-center gap-2 px-3 py-1.5 rounded-full" style={{
                    background: `${planet.color}18`,
                    border: `1px solid ${planet.color}35`,
                    opacity: Math.min((Math.abs(swipeDeltaY) - 28) / 35, 1),
                  }}>
                    <i className={swipeDeltaY < 0 ? 'ri-arrow-down-line' : 'ri-arrow-up-line'} style={{ fontSize: 11, color: planet.color }} />
                    <span style={{ fontSize: 10, color: planet.color, letterSpacing: '0.1em' }}>
                      {swipeDeltaY < 0
                        ? (hasNextModule ? moduleGroups[moduleIndex + 1]?.label : '已是最后')
                        : (hasPrevModule ? moduleGroups[moduleIndex - 1]?.label : '已是第一')}
                    </span>
                  </div>
                </div>
              )}

              {isSwipingX && Math.abs(swipeDeltaX) > 28 && (
                <div className="absolute inset-0 flex items-center pointer-events-none"
                  style={{ justifyContent: swipeDeltaX < 0 ? 'flex-end' : 'flex-start', padding: '0 16px', zIndex: 20 }}
                >
                  <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-full" style={{
                    background: `${planet.color}18`,
                    border: `1px solid ${planet.color}35`,
                    opacity: Math.min((Math.abs(swipeDeltaX) - 28) / 35, 1),
                  }}>
                    <i className={swipeDeltaX < 0 ? 'ri-arrow-right-line' : 'ri-arrow-left-line'} style={{ fontSize: 11, color: planet.color }} />
                    <span style={{ fontSize: 10, color: planet.color, letterSpacing: '0.1em' }}>
                      {swipeDeltaX < 0
                        ? (stateIndex < moduleStates.length - 1 ? stateLabelMap[moduleStates[stateIndex + 1]] : '已是最后')
                        : (stateIndex > 0 ? stateLabelMap[moduleStates[stateIndex - 1]] : '已是第一')}
                    </span>
                  </div>
                </div>
              )}

              <div
                className="px-5 pt-4 pb-2 flex flex-col gap-4"
                style={{
                  opacity: contentOpacity,
                  transform: `${contentTransformX} ${contentTransformY}`,
                  transition: (isSwipingX || isSwipingY)
                    ? 'none'
                    : (slideOutX || slideOutY)
                    ? 'opacity 0.22s ease, transform 0.22s ease'
                    : 'opacity 0.22s ease, transform 0.28s cubic-bezier(0.34,1.56,0.64,1)',
                  maxHeight: panelH ? panelH - 200 : 520,
                  overflowY: 'auto',
                  pointerEvents: (isSwipingX || isSwipingY) ? 'none' : 'auto',
                }}
              >
                <FocusCard stateData={stateData} visible={contentVisible} />

                {isTask && stateData.progressSteps && stateData.progressSteps.length > 0 && (
                  <><div style={{ height: 1, background: `${planet.color}12` }} /><ProgressTrack stateData={stateData} visible={contentVisible} /></>
                )}
                {isTask && stateData.context.length > 0 && (
                  <><div style={{ height: 1, background: `${planet.color}12` }} /><ContextStream stateData={stateData} visible={contentVisible} /></>
                )}
                {isNotepad && (
                  <>
                    <div style={{ height: 1, background: `${planet.color}12` }} />
                    <NotepadLayer stateData={stateData} visible={contentVisible} />
                    {stateData.context.length > 0 && <><div style={{ height: 1, background: `${planet.color}12` }} /><ContextStream stateData={stateData} visible={contentVisible} /></>}
                  </>
                )}
                {isMirror && (
                  <><div style={{ height: 1, background: `${planet.color}12` }} /><MirrorLayer stateData={stateData} visible={contentVisible} /></>
                )}
                {isSense && (
                  <>
                    <div style={{ height: 1, background: `${planet.color}12` }} />
                    <SenseLayer stateData={stateData} visible={contentVisible} />
                    {stateData.context.length > 0 && <><div style={{ height: 1, background: `${planet.color}12` }} /><ContextStream stateData={stateData} visible={contentVisible} /></>}
                  </>
                )}
                {stateData.anomaly && (
                  <AnomalyLayer stateData={stateData} visible={contentVisible} onAction={() => {}} onDismiss={() => {}} />
                )}

                {/* Footer breadcrumb */}
                <div className="flex items-center justify-center gap-2 pb-1" style={{ opacity: contentVisible ? 0.2 : 0, transition: 'opacity 0.5s ease 0.8s', marginTop: -4 }}>
                  <div className="flex-1 h-px" style={{ background: `linear-gradient(to right, transparent, ${planet.color}30)` }} />
                  <span style={{ fontSize: 8.5, color: planet.color, letterSpacing: '0.2em', opacity: 0.7 }}>
                    {moduleIndex + 1}/{moduleOrder.length} · {stateIndex + 1}/{moduleStates.length} · {planet.label.toUpperCase()}
                  </span>
                  <div className="flex-1 h-px" style={{ background: `linear-gradient(to left, transparent, ${planet.color}30)` }} />
                </div>
              </div>
            </div>

            {/* ── Quick actions ── */}
            <QuickActions planet={planet} moduleKey={activePlanetKey} visible={contentVisible} />

            <div style={{ height: 2, background: `linear-gradient(to right, transparent, ${planet.color}60, transparent)`, opacity: 0.6 }} />
          </div>

          {/* ── Resize handles ── */}
          {/* Right edge */}
          <div
            onMouseDown={e => startResize(e, 'right')}
            style={{
              position: 'absolute', top: 16, right: -4, bottom: 16,
              width: 8, cursor: 'ew-resize', zIndex: 20,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}
          >
            <div style={{
              width: 2, height: 32, borderRadius: 2,
              background: isResizing && resizingRef.current?.edge === 'right' ? planet.color : `${planet.color}30`,
              transition: 'background 0.2s ease',
            }} />
          </div>
          {/* Left edge */}
          <div
            onMouseDown={e => startResize(e, 'left')}
            style={{
              position: 'absolute', top: 16, left: -4, bottom: 16,
              width: 8, cursor: 'ew-resize', zIndex: 20,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}
          >
            <div style={{
              width: 2, height: 32, borderRadius: 2,
              background: isResizing && resizingRef.current?.edge === 'left' ? planet.color : `${planet.color}30`,
              transition: 'background 0.2s ease',
            }} />
          </div>
          {/* Bottom edge */}
          <div
            onMouseDown={e => startResize(e, 'bottom')}
            style={{
              position: 'absolute', bottom: -4, left: 16, right: 16,
              height: 8, cursor: 'ns-resize', zIndex: 20,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}
          >
            <div style={{
              height: 2, width: 32, borderRadius: 2,
              background: isResizing && resizingRef.current?.edge === 'bottom' ? planet.color : `${planet.color}30`,
              transition: 'background 0.2s ease',
            }} />
          </div>
          {/* Bottom-right corner */}
          <div
            onMouseDown={e => startResize(e, 'br')}
            style={{
              position: 'absolute', bottom: -4, right: -4,
              width: 16, height: 16, cursor: 'se-resize', zIndex: 21,
            }}
          >
            <div style={{
              position: 'absolute', bottom: 4, right: 4,
              width: 8, height: 8,
              borderRight: `2px solid ${planet.color}55`,
              borderBottom: `2px solid ${planet.color}55`,
              borderRadius: '0 0 2px 0',
            }} />
          </div>
          {/* Bottom-left corner */}
          <div
            onMouseDown={e => startResize(e, 'bl')}
            style={{
              position: 'absolute', bottom: -4, left: -4,
              width: 16, height: 16, cursor: 'sw-resize', zIndex: 21,
            }}
          >
            <div style={{
              position: 'absolute', bottom: 4, left: 4,
              width: 8, height: 8,
              borderLeft: `2px solid ${planet.color}55`,
              borderBottom: `2px solid ${planet.color}55`,
              borderRadius: '0 0 0 2px',
            }} />
          </div>

          {/* Size indicator — shows while resizing, double-click to reset */}
          {isResizing && (
            <div
              onDoubleClick={() => {
                setPanelW(520); setPanelH(null);
                try { localStorage.removeItem(STORAGE_KEY); } catch (_e) { /* ignore */ }
              }}
              style={{
                position: 'absolute', top: -28, left: '50%',
                transform: 'translateX(-50%)',
                padding: '3px 12px',
                background: 'rgba(3,5,12,0.92)',
                border: `1px solid ${planet.color}35`,
                borderRadius: 12,
                fontSize: 9,
                color: planet.color,
                letterSpacing: '0.12em',
                whiteSpace: 'nowrap',
                cursor: 'pointer',
                display: 'flex', alignItems: 'center', gap: 6,
              }}
            >
              <i className="ri-save-line" style={{ fontSize: 9, opacity: 0.6 }} />
              {panelW} × {panelH ?? (panelRef.current?.offsetHeight ?? '—')}
              <span style={{ opacity: 0.4, fontSize: 8 }}>双击重置</span>
            </div>
          )}
        </div>
      </div>
    </>
  );
}
