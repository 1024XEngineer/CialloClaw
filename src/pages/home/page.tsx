import { useState, useEffect, useRef, useCallback } from 'react';
import { agentStates, moduleGroups } from '@/mocks/agentStates';
import type { AgentStateKey, ModuleKey } from '@/mocks/agentStates';
import PlanetNode from './components/PlanetNode';
import type { PlanetConfig } from './components/PlanetNode';
import CenterOrb from './components/CenterOrb';
import JarvisPanel from './components/JarvisPanel';
import DetachedWindow from './components/DetachedWindow';
import DashboardRings from './components/DashboardRings';
import VoiceInterface from './components/VoiceInterface';
import TrustBoundary from './components/TrustBoundary';
import ShortcutPanel from './components/ShortcutPanel';
import SummonedPlanet from './components/SummonedPlanet';
import type { SummonEvent } from './components/SummonedPlanet';

// ─────────────────────────────────────────────
// Three-tier consciousness field:
//   Tier 1 — FOCUS PLANET: the single most important event, pulled close, brightest
//   Tier 2 — CANDIDATE PLANETS: 2-3 weak presences, farther ring, low brightness
//   Tier 3 — BACKGROUND PULSE: barely visible, just signals "something running"
// ─────────────────────────────────────────────

// Focus planet — the one Agent is currently talking about
const FOCUS_PLANET: PlanetConfig = {
  key: 'task',
  label: '任务状态',
  icon: 'ri-robot-line',
  color: '#34d399',
  glow: 'rgba(52,211,153,0.4)',
  orbitRadius: 148,   // pulled closer to center
  orbitSpeed: 2.8,
  orbitOffset: 0,
  size: 62,
  description: 'TASK · FOCUS',
};

// Candidate planets — secondary attention, farther ring
const CANDIDATE_PLANETS: PlanetConfig[] = [
  {
    key: 'notepad',
    label: '便签协作',
    icon: 'ri-sticky-note-line',
    color: '#a78bfa',
    glow: 'rgba(167,139,250,0.3)',
    orbitRadius: 210,
    orbitSpeed: 2.2,
    orbitOffset: 110,
    size: 46,
    description: 'NOTEPAD · ASYNC',
  },
  {
    key: 'mirror',
    label: '镜子',
    icon: 'ri-eye-2-line',
    color: '#c4b5fd',
    glow: 'rgba(196,181,253,0.25)',
    orbitRadius: 215,
    orbitSpeed: 1.6,
    orbitOffset: 230,
    size: 42,
    description: 'MIRROR · INSIGHT',
  },
];

// Background pulse planets — barely there, just signals background activity
const BACKGROUND_PLANETS: PlanetConfig[] = [
  {
    key: 'sense',
    label: '硬件感知',
    icon: 'ri-pulse-line',
    color: '#38bdf8',
    glow: 'rgba(56,189,248,0.2)',
    orbitRadius: 248,
    orbitSpeed: 1.2,
    orbitOffset: 340,
    size: 34,
    description: 'HARDWARE · SENSE',
  },
];

const TRUST_PLANET: PlanetConfig = {
  key: 'trust' as ModuleKey,
  label: '信任与边界',
  icon: 'ri-shield-check-line',
  color: '#64748b',
  glow: 'rgba(100,116,139,0.2)',
  orbitRadius: 252,
  orbitSpeed: 0.9,
  orbitOffset: 175,
  size: 32,
  description: 'TRUST · BOUNDARY',
};

// All planets for orbit animation
const ALL_PLANETS = [FOCUS_PLANET, ...CANDIDATE_PLANETS, ...BACKGROUND_PLANETS, TRUST_PLANET];

// Planets shown in the main panel (excludes trust)
const PANEL_PLANETS = [FOCUS_PLANET, ...CANDIDATE_PLANETS, ...BACKGROUND_PLANETS];

// ── Summon event templates — Agent proactively surfaces these ──
const SUMMON_TEMPLATES: Omit<SummonEvent, 'id'>[] = [
  {
    planet: { ...CANDIDATE_PLANETS[0] }, // notepad
    message: '你有一个 2 小时后到期的任务',
    reason: '周报整理 · 今日 11:00 截止',
    nextStep: '点击查看并开始处理',
    priority: 'urgent',
    duration: 6000,
  },
  {
    planet: { ...CANDIDATE_PLANETS[1] }, // mirror
    message: '我总结了你这周反复出现的 3 类工作',
    reason: '产品策略、竞品分析、周报整理',
    nextStep: '查看洞察报告',
    priority: 'normal',
    duration: 5500,
  },
  {
    planet: { ...BACKGROUND_PLANETS[0] }, // sense
    message: '当前网络波动，上传已延后重试',
    reason: '自动处理中，无需操作',
    priority: 'low',
    duration: 4500,
  },
  {
    planet: { ...CANDIDATE_PLANETS[0] }, // notepad
    message: '刚保存了一份成果到工作区',
    reason: '竞品分析报告 · 已自动归档',
    nextStep: '查看工作区',
    priority: 'normal',
    duration: 5000,
  },
];

// Tier classification
const FOCUS_KEY = FOCUS_PLANET.key;
const CANDIDATE_KEYS = new Set(CANDIDATE_PLANETS.map(p => p.key));
const BACKGROUND_KEYS = new Set(BACKGROUND_PLANETS.map(p => p.key));

interface DetachedWindowState {
  id: string;
  moduleKey: ModuleKey;
  stateKey: AgentStateKey;
  x: number;
  y: number;
}

// ── Contextual trust boundary info — appears only in relevant scenarios ──
interface TrustInfo {
  riskLevel?: 'low' | 'medium' | 'high';
  needsAuth?: boolean;
  savePath?: string;
  recoverable?: boolean;
  costAnomaly?: string;
}

function getTrustInfo(stateKey: AgentStateKey): TrustInfo | null {
  switch (stateKey) {
    case 'error_permission':
      return { riskLevel: 'high', needsAuth: true, recoverable: true };
    case 'completing':
      return { savePath: '工作区 / 竞品分析', riskLevel: 'low' };
    case 'done':
      return { savePath: '工作区 / 竞品分析', recoverable: true };
    case 'error_blocked':
      return { riskLevel: 'medium', recoverable: true };
    case 'sense_alert':
      return { riskLevel: 'medium', costAnomaly: 'CPU 持续高负载，建议暂缓' };
    case 'highlight':
      return { riskLevel: 'low' };
    default:
      return null;
  }
}

// ── Main focus info — what's most important right now ──
function MainFocusBar({ stateKey, accentColor, visible }: {
  stateKey: AgentStateKey;
  accentColor: string;
  visible: boolean;
}) {
  const state = agentStates[stateKey];
  const trustInfo = getTrustInfo(stateKey);

  const riskColors = {
    low: '#34d399',
    medium: '#fbbf24',
    high: '#fb7185',
  };

  return (
    <div
      className="absolute"
      style={{
        bottom: 52,
        left: '50%',
        transform: 'translateX(-50%)',
        width: 520,
        opacity: visible ? 1 : 0,
        transition: 'opacity 0.8s ease 0.4s',
        pointerEvents: 'none',
        zIndex: 20,
      }}
    >
      {/* Main focus card */}
      <div
        className="rounded"
        style={{
          background: 'rgba(3,5,12,0.75)',
          border: `1px solid ${accentColor}18`,
          backdropFilter: 'blur(20px)',
          padding: '12px 16px',
          marginBottom: 8,
        }}
      >
        {/* Agent status line */}
        <div className="flex items-center gap-2 mb-2">
          <div className="rounded-full" style={{
            width: 5, height: 5,
            background: accentColor,
            boxShadow: `0 0 6px ${accentColor}`,
            animation: 'notifPulse 2s ease-in-out infinite',
          }} />
          <span style={{ fontSize: 9, color: `${accentColor}99`, letterSpacing: '0.2em' }}>
            AGENT · {state.tag.toUpperCase()}
          </span>
          <div className="flex-1" />
          <span style={{ fontSize: 8.5, color: 'rgba(71,85,105,0.5)', letterSpacing: '0.1em' }}>
            {new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}
          </span>
        </div>

        {/* Main headline */}
        <div style={{
          fontSize: 13,
          color: 'rgba(226,232,240,0.88)',
          lineHeight: 1.5,
          letterSpacing: '0.02em',
          marginBottom: 6,
          fontWeight: 500,
        }}>
          {state.headline}
        </div>

        {/* Subline — why it's worth seeing now */}
        <div style={{
          fontSize: 11,
          color: 'rgba(148,163,184,0.55)',
          lineHeight: 1.5,
          letterSpacing: '0.02em',
          marginBottom: trustInfo ? 10 : 0,
        }}>
          {state.subline}
        </div>

        {/* Contextual trust boundary info — only when relevant */}
        {trustInfo && (
          <div
            className="flex items-center gap-3 pt-2"
            style={{ borderTop: `1px solid rgba(255,255,255,0.05)` }}
          >
            {trustInfo.riskLevel && (
              <div className="flex items-center gap-1.5">
                <i className="ri-shield-line" style={{ fontSize: 9, color: riskColors[trustInfo.riskLevel] }} />
                <span style={{ fontSize: 8.5, color: riskColors[trustInfo.riskLevel], letterSpacing: '0.1em' }}>
                  风险 {trustInfo.riskLevel === 'low' ? '低' : trustInfo.riskLevel === 'medium' ? '中' : '高'}
                </span>
              </div>
            )}
            {trustInfo.needsAuth && (
              <div className="flex items-center gap-1.5">
                <i className="ri-key-line" style={{ fontSize: 9, color: '#fb7185' }} />
                <span style={{ fontSize: 8.5, color: '#fb7185', letterSpacing: '0.1em' }}>需要授权</span>
              </div>
            )}
            {trustInfo.savePath && (
              <div className="flex items-center gap-1.5">
                <i className="ri-folder-line" style={{ fontSize: 9, color: 'rgba(148,163,184,0.5)' }} />
                <span style={{ fontSize: 8.5, color: 'rgba(148,163,184,0.5)', letterSpacing: '0.06em' }}>
                  保存至 {trustInfo.savePath}
                </span>
              </div>
            )}
            {trustInfo.recoverable && (
              <div className="flex items-center gap-1.5">
                <i className="ri-history-line" style={{ fontSize: 9, color: 'rgba(148,163,184,0.4)' }} />
                <span style={{ fontSize: 8.5, color: 'rgba(148,163,184,0.4)', letterSpacing: '0.06em' }}>可恢复</span>
              </div>
            )}
            {trustInfo.costAnomaly && (
              <div className="flex items-center gap-1.5">
                <i className="ri-alert-line" style={{ fontSize: 9, color: '#fbbf24' }} />
                <span style={{ fontSize: 8.5, color: '#fbbf24', letterSpacing: '0.06em' }}>{trustInfo.costAnomaly}</span>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Next action hint — what user can do right now */}
      <div className="flex items-center justify-center gap-2">
        <div style={{ flex: 1, height: 1, background: `linear-gradient(to right, transparent, ${accentColor}20)` }} />
        <span style={{ fontSize: 9, color: 'rgba(71,85,105,0.4)', letterSpacing: '0.16em' }}>
          点击焦点球查看详情 · 长按中心球语音对话
        </span>
        <div style={{ flex: 1, height: 1, background: `linear-gradient(to left, transparent, ${accentColor}20)` }} />
      </div>
    </div>
  );
}

export default function Home() {
  const [angles, setAngles] = useState<Record<string, number>>(() => {
    const init: Record<string, number> = {};
    ALL_PLANETS.forEach(p => { init[p.key] = p.orbitOffset; });
    return init;
  });

  const draggingPlanets = useRef<Set<string>>(new Set());

  const [pulse, setPulse] = useState(0);
  const [activePlanet, setActivePlanet] = useState<ModuleKey | null>(null);
  const [currentState, setCurrentState] = useState<AgentStateKey>('working');
  const [transitioning, setTransitioning] = useState(false);
  const [orbDragOffset, setOrbDragOffset] = useState({ x: 0, y: 0 });
  const [trustOpen, setTrustOpen] = useState(false);
  const [voiceOpen, setVoiceOpen] = useState(false);
  const [shortcutOpen, setShortcutOpen] = useState(false);
  const [focusBarVisible, setFocusBarVisible] = useState(false);
  const [focusMode, setFocusMode] = useState(false);
  const [focusModeExiting, setFocusModeExiting] = useState(false);
  const [revealedPlanets, setRevealedPlanets] = useState<Set<string>>(new Set(ALL_PLANETS.map(p => p.key)));

  // Which fragment in DashboardRings is currently "focused"
  const [focusFragmentIndex] = useState<number | undefined>(1); // the urgent task fragment

  const [planetPositions, setPlanetPositions] = useState<Record<string, { x: number; y: number }>>(() => {
    const init: Record<string, { x: number; y: number }> = {};
    ALL_PLANETS.forEach(p => { init[p.key] = { x: 0, y: 0 }; });
    return init;
  });

  const [detachedWindows, setDetachedWindows] = useState<DetachedWindowState[]>([]);
  const [windowZOrder, setWindowZOrder] = useState<string[]>([]);
  const windowIdCounter = useRef(0);

  // ── Summon system ──
  const [activeSummons, setActiveSummons] = useState<SummonEvent[]>([]);
  const summonIdCounter = useRef(0);
  const summonIndexRef = useRef(0);
  const summonTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const animRef = useRef<number>(0);
  const lastTimeRef = useRef<number>(0);
  const containerRef = useRef<HTMLDivElement>(null);

  const stateData = agentStates[currentState];
  const activePlanetConfig = activePlanet
    ? ALL_PLANETS.find(p => p.key === activePlanet) ?? null
    : null;
  const activePlanetPos = activePlanet ? planetPositions[activePlanet] : null;

  // Determine tier of active planet
  const activeTier = activePlanet
    ? activePlanet === FOCUS_KEY
      ? 'focus'
      : CANDIDATE_KEYS.has(activePlanet)
      ? 'candidate'
      : BACKGROUND_KEYS.has(activePlanet)
      ? 'background'
      : 'trust'
    : null;

  // Focus bar appears after a short delay
  useEffect(() => {
    const t = setTimeout(() => setFocusBarVisible(true), 1200);
    return () => clearTimeout(t);
  }, []);

  // ── Summon scheduler — fires events sequentially with gaps ──
  const scheduleSummon = useCallback(() => {
    const idx = summonIndexRef.current % SUMMON_TEMPLATES.length;
    const template = SUMMON_TEMPLATES[idx];
    summonIndexRef.current += 1;

    const id = `summon-${++summonIdCounter.current}`;
    const event: SummonEvent = { ...template, id };

    setActiveSummons(prev => {
      // Max 1 summon at a time
      if (prev.length >= 1) return prev;
      return [...prev, event];
    });

    // Schedule next summon: wait for this one's duration + gap
    const gap = (template.duration ?? 5000) + 7000;
    summonTimerRef.current = setTimeout(scheduleSummon, gap);
  }, []);

  useEffect(() => {
    // First summon fires after 2.5s (let the page settle)
    summonTimerRef.current = setTimeout(scheduleSummon, 2500);
    return () => {
      if (summonTimerRef.current) clearTimeout(summonTimerRef.current);
    };
  }, [scheduleSummon]);

  const handleSummonDismiss = useCallback((id: string) => {
    setActiveSummons(prev => prev.filter(s => s.id !== id));
  }, []);

  const handleSummonExpand = useCallback((planetKey: string) => {
    setActivePlanet(planetKey as ModuleKey);
    const group = moduleGroups.find(g => g.key === planetKey);
    if (group && group.states.length > 0) setCurrentState(group.states[0]);
  }, []);

  // Pulse + orbit rotation animation
  useEffect(() => {
    const animate = (timestamp: number) => {
      const elapsed = (Date.now() - Date.now()) / 1000;
      void elapsed;
      const t = Date.now() / 1000;
      setPulse(Math.sin(t * 1.2) * 0.5 + 0.5);

      const dt = lastTimeRef.current ? (timestamp - lastTimeRef.current) / 1000 : 0;
      lastTimeRef.current = timestamp;

      if (dt > 0 && dt < 0.1) {
        setAngles(prev => {
          const next = { ...prev };
          ALL_PLANETS.forEach(p => {
            if (!draggingPlanets.current.has(p.key) && p.orbitSpeed > 0) {
              next[p.key] = (prev[p.key] + p.orbitSpeed * dt) % 360;
            }
          });
          return next;
        });
      }

      animRef.current = requestAnimationFrame(animate);
    };
    animRef.current = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(animRef.current);
  }, []);

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement).tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA') return;
      if (e.key === 'f' || e.key === 'F') {
        e.preventDefault();
        setFocusMode(prev => {
          if (prev) {
            // Exiting focus mode — stagger planet reveal
            setFocusModeExiting(true);
            setRevealedPlanets(new Set());
            ALL_PLANETS.forEach((p, i) => {
              setTimeout(() => {
                setRevealedPlanets(cur => new Set([...cur, p.key]));
              }, 120 + i * 110);
            });
            setTimeout(() => setFocusModeExiting(false), 120 + ALL_PLANETS.length * 110 + 400);
          }
          return !prev;
        });
        return;
      }
      if (e.key === '?' || (e.key === '/' && e.shiftKey)) { e.preventDefault(); setShortcutOpen(prev => !prev); return; }
      if (e.key === '1') { handlePlanetClick('task'); return; }
      if (e.key === '2') { handlePlanetClick('notepad'); return; }
      if (e.key === '3') { handlePlanetClick('mirror'); return; }
      if (e.key === '4') { handlePlanetClick('sense'); return; }
      if (e.key === '5') { setTrustOpen(true); return; }
      if (e.key === 'Escape') {
        if (shortcutOpen) { setShortcutOpen(false); return; }
        if (voiceOpen) { setVoiceOpen(false); return; }
        if (trustOpen) { setTrustOpen(false); return; }
        if (activePlanet) { handlePanelClose(); return; }
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [activePlanet, shortcutOpen, voiceOpen, trustOpen]);

  const handlePlanetClick = useCallback((key: ModuleKey) => {
    if (activePlanet === key) { setActivePlanet(null); return; }
    setActivePlanet(key);
    const group = moduleGroups.find(g => g.key === key);
    if (group && group.states.length > 0) setCurrentState(group.states[0]);
  }, [activePlanet]);

  const handleStateChange = useCallback((key: AgentStateKey) => {
    if (key === currentState) return;
    setTransitioning(true);
    setTimeout(() => { setCurrentState(key); setTransitioning(false); }, 200);
  }, [currentState]);

  const handleModuleChange = useCallback((key: ModuleKey) => {
    if (key === activePlanet) return;
    setActivePlanet(key);
    const group = moduleGroups.find(g => g.key === key);
    if (group && group.states.length > 0) {
      setTransitioning(true);
      setTimeout(() => { setCurrentState(group.states[0]); setTransitioning(false); }, 200);
    }
  }, [activePlanet]);

  const handlePanelClose = useCallback(() => { setActivePlanet(null); }, []);

  const handlePositionUpdate = useCallback((key: string, x: number, y: number) => {
    setPlanetPositions(prev => {
      if (prev[key]?.x === x && prev[key]?.y === y) return prev;
      return { ...prev, [key]: { x, y } };
    });
  }, []);

  const handleOrbitAngleChange = useCallback((key: string, newAngle: number) => {
    setAngles(prev => ({ ...prev, [key]: newAngle }));
    draggingPlanets.current.delete(key);
  }, []);

  const handlePlanetDragStart = useCallback((key: string) => {
    draggingPlanets.current.add(key);
  }, []);

  const handlePlanetDragEnd = useCallback((key: string) => {
    draggingPlanets.current.delete(key);
  }, []);

  const handleDetach = useCallback((moduleKey: ModuleKey, stateKey: AgentStateKey, x: number, y: number) => {
    const id = `dw-${++windowIdCounter.current}`;
    setDetachedWindows(prev => [...prev, { id, moduleKey, stateKey, x, y }]);
    setWindowZOrder(prev => [...prev, id]);
  }, []);

  const handleWindowClose = useCallback((id: string) => {
    setDetachedWindows(prev => prev.filter(w => w.id !== id));
    setWindowZOrder(prev => prev.filter(z => z !== id));
  }, []);

  const handleWindowFocus = useCallback((id: string) => {
    setWindowZOrder(prev => [...prev.filter(z => z !== id), id]);
  }, []);

  const getWindowZIndex = useCallback((id: string) => 150 + windowZOrder.indexOf(id), [windowZOrder]);

  const handleVoiceCommand = useCallback((_command: string, module?: ModuleKey) => {
    if (module) handlePlanetClick(module);
    setVoiceOpen(false);
  }, [handlePlanetClick]);

  const anyPanelOpen = activePlanet !== null || trustOpen;

  // Compute per-planet visual weight based on tier and active state
  const getPlanetVisualWeight = (key: string): 'focus' | 'candidate' | 'background' | 'dimmed' => {
    if (focusMode && !anyPanelOpen) return 'dimmed';
    // During focus mode exit — stagger reveal
    if (focusModeExiting && !anyPanelOpen && !revealedPlanets.has(key)) return 'dimmed';
    if (anyPanelOpen) {
      if (activePlanet === key || (trustOpen && key === 'trust')) return 'focus';
      return 'dimmed';
    }
    if (key === FOCUS_KEY) return 'focus';
    if (CANDIDATE_KEYS.has(key as ModuleKey)) return 'candidate';
    return 'background';
  };

  return (
    <div
      className="w-screen h-screen relative overflow-hidden"
      style={{
        background: '#03050c',
        fontFamily: "'Inter', -apple-system, sans-serif",
        minWidth: 1024,
        minHeight: 768,
      }}
    >
      {/* ── Background ── */}
      <div className="absolute inset-0" style={{
        background: 'radial-gradient(ellipse 90% 80% at 50% 50%, rgba(8,12,28,1) 0%, #020408 70%)',
        pointerEvents: 'none',
      }} />

      {/* Star field */}
      <div className="absolute inset-0" style={{
        backgroundImage: `
          radial-gradient(rgba(255,255,255,0.05) 1px, transparent 1px),
          radial-gradient(rgba(255,255,255,0.022) 1px, transparent 1px)
        `,
        backgroundSize: '130px 130px, 65px 65px',
        backgroundPosition: '0px 0px, 32px 32px',
        maskImage: 'radial-gradient(ellipse 88% 88% at 50% 50%, black 0%, transparent 100%)',
        pointerEvents: 'none',
      }} />

      {/* Central consciousness glow — breathes with pulse */}
      <div className="absolute inset-0" style={{
        background: `radial-gradient(ellipse 42% 38% at 50% 50%, rgba(30,40,80,${0.32 + pulse * 0.1}) 0%, transparent 65%)`,
        transition: 'background 0.4s ease',
        pointerEvents: 'none',
      }} />

      {/* Focus planet color wash — stronger than before */}
      <div className="absolute inset-0" style={{
        background: `radial-gradient(ellipse 38% 35% at 50% 50%, rgba(52,211,153,${0.06 + pulse * 0.03}) 0%, transparent 55%)`,
        pointerEvents: 'none',
      }} />

      {/* Active planet color wash */}
      {activePlanetConfig && (
        <div className="absolute inset-0" style={{
          background: `radial-gradient(ellipse 55% 50% at 50% 50%, ${activePlanetConfig.glow} 0%, transparent 60%)`,
          opacity: 0.2,
          transition: 'background 0.8s ease, opacity 0.8s ease',
          pointerEvents: 'none',
        }} />
      )}

      {/* ── Top HUD ── */}
      <div className="absolute top-0 left-0 right-0 flex items-center justify-between px-6 py-4" style={{ zIndex: 50, pointerEvents: 'none' }}>
        <div className="flex items-center gap-3" style={{ pointerEvents: 'auto' }}>
          <div className="flex items-center gap-2 px-3 py-1.5 rounded" style={{
            background: 'rgba(255,255,255,0.03)',
            border: '1px solid rgba(255,255,255,0.06)',
          }}>
            <div className="rounded-full" style={{ width: 5, height: 5, background: '#34d399', boxShadow: '0 0 6px #34d399', animation: 'notifPulse 2s ease-in-out infinite' }} />
            <span style={{ fontSize: 10, color: 'rgba(203,213,225,0.55)', letterSpacing: '0.2em' }}>JARVIS · ONLINE</span>
          </div>

          {/* Tier legend — subtle */}
          <div className="flex items-center gap-3" style={{ opacity: 0.35 }}>
            <div className="flex items-center gap-1.5">
              <div className="rounded-full" style={{ width: 6, height: 6, background: '#34d399', boxShadow: '0 0 5px #34d399' }} />
              <span style={{ fontSize: 8.5, color: 'rgba(148,163,184,0.6)', letterSpacing: '0.1em' }}>焦点</span>
            </div>
            <div className="flex items-center gap-1.5">
              <div className="rounded-full" style={{ width: 5, height: 5, background: '#a78bfa' }} />
              <span style={{ fontSize: 8.5, color: 'rgba(148,163,184,0.6)', letterSpacing: '0.1em' }}>候选</span>
            </div>
            <div className="flex items-center gap-1.5">
              <div className="rounded-full" style={{ width: 4, height: 4, background: 'rgba(148,163,184,0.4)' }} />
              <span style={{ fontSize: 8.5, color: 'rgba(148,163,184,0.6)', letterSpacing: '0.1em' }}>后台</span>
            </div>
          </div>
        </div>

        {/* Focus mode badge */}
        {focusMode && (
          <button
            onClick={() => {
              setFocusMode(false);
              setFocusModeExiting(true);
              setRevealedPlanets(new Set());
              ALL_PLANETS.forEach((p, i) => {
                setTimeout(() => {
                  setRevealedPlanets(cur => new Set([...cur, p.key]));
                }, 120 + i * 110);
              });
              setTimeout(() => setFocusModeExiting(false), 120 + ALL_PLANETS.length * 110 + 400);
            }}
            className="flex items-center gap-2 px-3 py-1.5 rounded cursor-pointer transition-all duration-200 whitespace-nowrap"
            style={{
              background: 'rgba(52,211,153,0.08)',
              border: '1px solid rgba(52,211,153,0.25)',
              color: '#34d399',
              fontSize: 9.5,
              letterSpacing: '0.16em',
              animation: 'focusModeIn 0.4s cubic-bezier(0.16,1,0.3,1)',
              pointerEvents: 'auto',
            }}
            onMouseEnter={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(52,211,153,0.14)'; }}
            onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(52,211,153,0.08)'; }}
          >
            <div className="rounded-full" style={{ width: 5, height: 5, background: '#34d399', boxShadow: '0 0 6px #34d399' }} />
            <span>专注模式</span>
            <span style={{ opacity: 0.5, fontFamily: 'monospace' }}>F</span>
          </button>
        )}

        {detachedWindows.length > 0 && (
          <div className="flex items-center gap-2 px-3 py-1.5 rounded" style={{
            background: 'rgba(255,255,255,0.04)',
            border: '1px solid rgba(255,255,255,0.07)',
            pointerEvents: 'auto',
          }}>
            <div className="rounded-full" style={{ width: 5, height: 5, background: 'rgba(148,163,184,0.5)', animation: 'notifPulse 2s ease-in-out infinite' }} />
            <span style={{ fontSize: 9.5, color: 'rgba(148,163,184,0.55)', letterSpacing: '0.12em' }}>
              {detachedWindows.length} 个独立窗口
            </span>
          </div>
        )}

        <div className="flex items-center gap-2" style={{ pointerEvents: 'auto' }}>
          <button
            onClick={() => setShortcutOpen(true)}
            className="flex items-center gap-2 px-3 py-1.5 rounded cursor-pointer transition-all duration-200 whitespace-nowrap"
            style={{
              background: 'rgba(255,255,255,0.03)',
              border: '1px solid rgba(255,255,255,0.06)',
              color: 'rgba(148,163,184,0.45)',
              fontSize: 9.5,
              letterSpacing: '0.12em',
            }}
            onMouseEnter={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(100,116,139,0.1)'; }}
            onMouseLeave={e => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.03)'; }}
          >
            <i className="ri-keyboard-line" style={{ fontSize: 11 }} />
            <span>快捷键</span>
            <span style={{ fontFamily: 'monospace', opacity: 0.5 }}>?</span>
          </button>
        </div>
      </div>

      {/* ── Dashboard Rings — decorative, no pointer events ── */}
      <div
        className="absolute inset-0 flex items-center justify-center"
        style={{
          pointerEvents: focusMode ? 'none' : undefined,
          opacity: focusMode ? 0 : 1,
          transition: 'opacity 0.6s cubic-bezier(0.16,1,0.3,1)',
        }}
      >
        <DashboardRings
          pulse={pulse}
          offset={orbDragOffset}
          focusFragmentIndex={focusFragmentIndex}
        />
      </div>

      {/* ── Orbit system ── */}
      <div
        className="absolute inset-0 flex items-center justify-center"
        style={{ pointerEvents: 'none' }}
      >
        <div
          ref={containerRef}
          className="relative flex items-center justify-center"
          style={{ width: 600, height: 600, pointerEvents: 'none' }}
        >
          {/* Focus orbit ring — slightly visible */}
          <div className="absolute rounded-full" style={{
            width: FOCUS_PLANET.orbitRadius * 2,
            height: FOCUS_PLANET.orbitRadius * 2,
            left: '50%', top: '50%',
            transform: 'translate(-50%, -50%)',
            border: '1px solid rgba(52,211,153,0.07)',
            pointerEvents: 'none',
          }} />

          {/* Candidate orbit ring — barely visible */}
          <div className="absolute rounded-full" style={{
            width: 430,
            height: 430,
            left: '50%', top: '50%',
            transform: 'translate(-50%, -50%)',
            border: '1px solid rgba(255,255,255,0.025)',
            pointerEvents: 'none',
          }} />

          {/* Background orbit ring — almost invisible */}
          <div className="absolute rounded-full" style={{
            width: 510,
            height: 510,
            left: '50%', top: '50%',
            transform: 'translate(-50%, -50%)',
            border: '1px solid rgba(255,255,255,0.012)',
            pointerEvents: 'none',
          }} />

          {/* ── Focus Planet — the main character ── */}
          <PlanetNode
            key={FOCUS_PLANET.key}
            config={FOCUS_PLANET}
            angle={angles[FOCUS_PLANET.key]}
            isActive={activePlanet === FOCUS_PLANET.key}
            isAnyActive={anyPanelOpen}
            visualWeight={getPlanetVisualWeight(FOCUS_PLANET.key)}
            onClick={() => handlePlanetClick(FOCUS_PLANET.key)}
            pulse={pulse}
            orbDragOffset={orbDragOffset}
            onPositionUpdate={handlePositionUpdate}
            containerRef={containerRef}
            onOrbitAngleChange={handleOrbitAngleChange}
            onDragStart={handlePlanetDragStart}
            onDragEnd={handlePlanetDragEnd}
          />

          {/* ── Candidate Planets ── */}
          {CANDIDATE_PLANETS.map(planet => (
            <PlanetNode
              key={planet.key}
              config={planet}
              angle={angles[planet.key]}
              isActive={activePlanet === planet.key}
              isAnyActive={anyPanelOpen}
              visualWeight={getPlanetVisualWeight(planet.key)}
              onClick={() => handlePlanetClick(planet.key)}
              pulse={pulse}
              orbDragOffset={orbDragOffset}
              onPositionUpdate={handlePositionUpdate}
              containerRef={containerRef}
              onOrbitAngleChange={handleOrbitAngleChange}
              onDragStart={handlePlanetDragStart}
              onDragEnd={handlePlanetDragEnd}
            />
          ))}

          {/* ── Background Pulse Planets ── */}
          {BACKGROUND_PLANETS.map(planet => (
            <PlanetNode
              key={planet.key}
              config={planet}
              angle={angles[planet.key]}
              isActive={activePlanet === planet.key}
              isAnyActive={anyPanelOpen}
              visualWeight={getPlanetVisualWeight(planet.key)}
              onClick={() => handlePlanetClick(planet.key)}
              pulse={pulse}
              orbDragOffset={orbDragOffset}
              onPositionUpdate={handlePositionUpdate}
              containerRef={containerRef}
              onOrbitAngleChange={handleOrbitAngleChange}
              onDragStart={handlePlanetDragStart}
              onDragEnd={handlePlanetDragEnd}
            />
          ))}

          {/* ── Trust Planet — background pulse tier ── */}
          <PlanetNode
            key="trust"
            config={TRUST_PLANET}
            angle={angles['trust']}
            isActive={trustOpen}
            isAnyActive={activePlanet !== null}
            visualWeight={getPlanetVisualWeight('trust')}
            onClick={() => setTrustOpen(true)}
            pulse={pulse}
            orbDragOffset={orbDragOffset}
            onPositionUpdate={handlePositionUpdate}
            containerRef={containerRef}
            onOrbitAngleChange={handleOrbitAngleChange}
            onDragStart={handlePlanetDragStart}
            onDragEnd={handlePlanetDragEnd}
          />

          {/* ── Center Orb — consciousness core ── */}
          <div style={{ position: 'absolute', left: '50%', top: '50%', transform: 'translate(-50%, -50%)', zIndex: 30, pointerEvents: 'auto' }}>
            <CenterOrb
              stateData={stateData}
              activePlanetColor={activePlanetConfig?.color ?? null}
              pulse={pulse}
              onDragOffset={(dx, dy) => setOrbDragOffset({ x: dx, y: dy })}
              onLongPress={() => setVoiceOpen(true)}
              focusMode={focusMode}
            />
          </div>

          {/* ── Summoned Planets — appear when no panel is open and not in focus mode ── */}
          {!anyPanelOpen && !focusMode && activeSummons.map(event => (
            <SummonedPlanet
              key={event.id}
              event={event}
              onDismiss={handleSummonDismiss}
              onExpand={handleSummonExpand}
            />
          ))}
        </div>
      </div>

      {/* ── Main Focus Bar — bottom, always visible when no panel open ── */}
      {!activePlanet && !trustOpen && (
        <MainFocusBar
          stateKey={currentState}
          accentColor={stateData.accentColor}
          visible={focusBarVisible}
        />
      )}

      {/* ── Tier label hints — very subtle, bottom corners ── */}
      {!activePlanet && !trustOpen && (
        <div
          className="absolute"
          style={{
            bottom: 20,
            right: 28,
            opacity: focusBarVisible ? 0.22 : 0,
            transition: 'opacity 0.8s ease 1.5s',
            pointerEvents: 'none',
          }}
        >
          <div className="flex flex-col items-end gap-1">
            {activeTier === null && (
              <>
                <div className="flex items-center gap-1.5">
                  <span style={{ fontSize: 8, color: 'rgba(148,163,184,0.5)', letterSpacing: '0.12em' }}>任务状态</span>
                  <div className="rounded-full" style={{ width: 5, height: 5, background: '#34d399', boxShadow: '0 0 4px #34d399' }} />
                </div>
                <div className="flex items-center gap-1.5">
                  <span style={{ fontSize: 8, color: 'rgba(148,163,184,0.4)', letterSpacing: '0.12em' }}>便签 · 镜子</span>
                  <div className="rounded-full" style={{ width: 4, height: 4, background: '#a78bfa' }} />
                </div>
                <div className="flex items-center gap-1.5">
                  <span style={{ fontSize: 8, color: 'rgba(148,163,184,0.3)', letterSpacing: '0.12em' }}>感知 · 边界</span>
                  <div className="rounded-full" style={{ width: 3, height: 3, background: 'rgba(148,163,184,0.4)' }} />
                </div>
              </>
            )}
          </div>
        </div>
      )}

      {/* ── Jarvis Panel ── */}
      {activePlanet && activePlanetConfig && activePlanetPos && (
        <>
          <div
            className="absolute inset-0"
            style={{
              background: 'rgba(3,5,12,0.55)',
              backdropFilter: 'blur(3px)',
              animation: 'backdropIn 0.3s ease',
              zIndex: 49,
            }}
            onClick={handlePanelClose}
          />
          <JarvisPanel
            planets={PANEL_PLANETS}
            activePlanetKey={activePlanet}
            planetScreenX={activePlanetPos.x}
            planetScreenY={activePlanetPos.y}
            stateData={stateData}
            currentStateKey={currentState}
            onClose={handlePanelClose}
            onStateChange={handleStateChange}
            onModuleChange={handleModuleChange}
            transitioning={transitioning}
            onDetach={handleDetach}
          />
        </>
      )}

      {/* ── Detached Windows ── */}
      {detachedWindows.map(win => {
        const planet = PANEL_PLANETS.find(p => p.key === win.moduleKey) ?? FOCUS_PLANET;
        const winStateData = agentStates[win.stateKey];
        return (
          <DetachedWindow
            key={win.id}
            id={win.id}
            planet={planet}
            stateData={winStateData}
            currentStateKey={win.stateKey}
            moduleKey={win.moduleKey}
            initialX={win.x}
            initialY={win.y}
            onClose={handleWindowClose}
            onFocus={handleWindowFocus}
            zIndex={getWindowZIndex(win.id)}
          />
        );
      })}

      {/* ── Voice Interface ── */}
      <VoiceInterface
        isOpen={voiceOpen}
        onClose={() => setVoiceOpen(false)}
        onCommand={handleVoiceCommand}
        orbColor={activePlanetConfig?.color ?? stateData.accentColor}
        orbGlow={activePlanetConfig?.glow ?? stateData.orbGlow}
      />

      {/* ── Trust Boundary ── */}
      <TrustBoundary
        visible={trustOpen}
        onClose={() => setTrustOpen(false)}
      />

      {/* ── Shortcut Panel ── */}
      <ShortcutPanel
        visible={shortcutOpen}
        onClose={() => setShortcutOpen(false)}
      />

      <style>{`
        @import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600&display=swap');
        @keyframes notifPulse { 0%,100%{opacity:1;transform:scale(1)} 50%{opacity:0.3;transform:scale(0.65)} }
        @keyframes focusModeIn { from{opacity:0;transform:scale(0.88)} to{opacity:1;transform:scale(1)} }
        @keyframes activeRingPulse { 0%,100%{opacity:0.6;transform:scale(1)} 50%{opacity:0.2;transform:scale(1.08)} }
        @keyframes anomalyPulse { 0%,100%{box-shadow:0 0 30px rgba(251,113,133,0.5),0 0 12px rgba(251,113,133,0.3) inset} 50%{box-shadow:0 0 55px rgba(251,113,133,0.8),0 0 20px rgba(251,113,133,0.5) inset} }
        @keyframes labelFadeIn { from{opacity:0;transform:translateX(-50%) translateY(4px)} to{opacity:1;transform:translateX(-50%) translateY(0)} }
        @keyframes backdropIn { from{opacity:0} to{opacity:1} }
        @keyframes snapRing { 0%{opacity:0.8;transform:scale(1)} 100%{opacity:0;transform:scale(1.6)} }
        * { box-sizing: border-box; }
        ::-webkit-scrollbar { display: none; }
      `}</style>
    </div>
  );
}
