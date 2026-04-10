import { useCallback, useEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { Keyboard, Sparkles } from "lucide-react";
import { useNavigate } from "react-router-dom";
import ClickSpark from "@/components/ClickSpark";
import { dashboardDecorOrbs, dashboardEntranceOrbs, dashboardModuleColors } from "@/features/dashboard/home/dashboardHome.config";
import { dashboardHomeStates, dashboardSummonTemplates, dashboardVoiceSequences } from "@/features/dashboard/home/dashboardHome.mocks";
import type { DashboardHomeEventStateKey, DashboardHomeModuleKey, DashboardHomeSummonEvent } from "@/features/dashboard/home/dashboardHome.types";
import { DashboardCenterOrb } from "@/features/dashboard/home/components/DashboardCenterOrb";
import { DashboardDecorOrb } from "@/features/dashboard/home/components/DashboardDecorOrb";
import { DashboardEntranceOrb } from "@/features/dashboard/home/components/DashboardEntranceOrb";
import { DashboardEventOrb } from "@/features/dashboard/home/components/DashboardEventOrb";
import { DashboardEventPanel } from "@/features/dashboard/home/components/DashboardEventPanel";
import { DashboardOrbitRings } from "@/features/dashboard/home/components/DashboardOrbitRings";
import { DashboardVoiceField } from "@/features/dashboard/home/components/DashboardVoiceField";
import { cn } from "@/utils/cn";
import "@/features/shell-ball/shellBall.css";
import "@/features/dashboard/home/dashboardHome.css";

function getRouteForModule(module: DashboardHomeModuleKey) {
  const routes = {
    tasks: "/tasks",
    notes: "/notes",
    memory: "/memory",
    safety: "/safety",
  } as const;

  return routes[module];
}

function getCenterState(activeStateKey: DashboardHomeEventStateKey | null) {
  if (!activeStateKey) {
    return "idle" as const;
  }

  if (activeStateKey.startsWith("task_error") || activeStateKey === "safety_alert") {
    return "waiting_auth" as const;
  }

  if (activeStateKey === "task_working" || activeStateKey === "notes_processing") {
    return "processing" as const;
  }

  if (activeStateKey === "task_completing") {
    return "confirming_intent" as const;
  }

  return "hover_input" as const;
}

export function DashboardHome() {
  const navigate = useNavigate();
  const [orbDragOffset, setOrbDragOffset] = useState({ x: 0, y: 0 });
  const [hoveredEntranceKey, setHoveredEntranceKey] = useState<string | null>(null);
  const [activeStateKey, setActiveStateKey] = useState<DashboardHomeEventStateKey | null>(null);
  const [voiceOpen, setVoiceOpen] = useState(false);
  const [summons, setSummons] = useState<DashboardHomeSummonEvent[]>([]);
  const summonIndexRef = useRef(0);
  const summonIdRef = useRef(0);
  const summonTimerRef = useRef<number | null>(null);

  const activeState = activeStateKey ? dashboardHomeStates[activeStateKey] : null;
  const activeModule = hoveredEntranceKey
    ? dashboardEntranceOrbs.find((config) => config.key === hoveredEntranceKey)?.module ?? activeState?.module ?? null
    : activeState?.module ?? null;
  const activeModuleColor = activeModule ? dashboardModuleColors[activeModule].color : null;
  const currentFocusLine = activeState?.headline ?? summons[0]?.message ?? "让中心球和 4 个入口球一起构成今天的任务轨道。";
  const currentReasonLine = activeState?.subline ?? summons[0]?.reason ?? "长按中心球可以直接进入语音模式，四个入口球会始终保持最显眼的位置。";
  const isOverlayOpen = Boolean(activeState || voiceOpen);

  const scheduleSummon = useCallback(() => {
    const template = dashboardSummonTemplates[summonIndexRef.current % dashboardSummonTemplates.length];
    summonIndexRef.current += 1;

    setSummons((current) => {
      if (current.length >= 1) {
        return current;
      }

      return [
        ...current,
        {
          ...template,
          id: `summon-${++summonIdRef.current}`,
        },
      ];
    });

    const gap = (template.duration ?? 5000) + 7000;
    summonTimerRef.current = window.setTimeout(scheduleSummon, gap);
  }, []);

  useEffect(() => {
    summonTimerRef.current = window.setTimeout(scheduleSummon, 2500);

    return () => {
      if (summonTimerRef.current) {
        window.clearTimeout(summonTimerRef.current);
      }
    };
  }, [scheduleSummon]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      const tag = target?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA") {
        return;
      }

      if (event.key === "Escape") {
        if (voiceOpen) {
          event.preventDefault();
          setVoiceOpen(false);
          return;
        }

        if (activeStateKey) {
          event.preventDefault();
          setActiveStateKey(null);
        }
        return;
      }

      if (!event.ctrlKey && !event.metaKey) {
        return;
      }

      if (event.key === "1") {
        event.preventDefault();
        navigate("/tasks");
      }

      if (event.key === "2") {
        event.preventDefault();
        navigate("/notes");
      }

      if (event.key === "3") {
        event.preventDefault();
        navigate("/memory");
      }

      if (event.key === "4") {
        event.preventDefault();
        navigate("/safety");
      }

      if (event.key === "5") {
        event.preventDefault();
        setVoiceOpen(true);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activeStateKey, navigate, voiceOpen]);

  const centerVisualState = voiceOpen ? "voice_locked" : getCenterState(activeStateKey);
  const pageStyle = {
    "--dashboard-active-color": activeModuleColor ?? "#9FB7D8",
  } as CSSProperties;

  const handleOrbDragOffset = useCallback((x: number, y: number) => {
    setOrbDragOffset((current) => {
      if (current.x === x && current.y === y) {
        return current;
      }

      return { x, y };
    });
  }, []);

  const handleModuleNavigate = useCallback((module: DashboardHomeModuleKey) => {
    const nextPath = getRouteForModule(module);
    navigate(nextPath);
  }, [navigate]);

  return (
    <ClickSpark className="dashboard-orbit-home" duration={360} extraScale={1.12} sparkColor="#d9b980" sparkCount={10} sparkRadius={18} sparkSize={11} style={pageStyle}>
      <header className="dashboard-orbit-home__hud">
        <div className="dashboard-orbit-home__badge-shell">
          <div className="dashboard-orbit-home__badge-dot" />
          <span>Dashboard Orbit</span>
        </div>

        <div className="dashboard-orbit-home__shortcut-pill">
          <Keyboard className="h-3.5 w-3.5" />
          Ctrl / Cmd + 1 2 3 4 5
        </div>
      </header>

      <div className="dashboard-orbit-home__canvas">
        <DashboardOrbitRings offset={orbDragOffset} />

        {dashboardDecorOrbs.map((config) => (
          <DashboardDecorOrb key={config.key} config={config} dimmed={isOverlayOpen} offset={orbDragOffset} />
        ))}

        {dashboardEntranceOrbs.map((config) => (
          <DashboardEntranceOrb
            key={config.key}
            config={config}
            dimmed={Boolean(activeState && activeState.module !== config.module) || voiceOpen}
            isHovered={hoveredEntranceKey === config.key}
            offset={orbDragOffset}
            onClick={() => handleModuleNavigate(config.module)}
            onHoverChange={(hovered) => setHoveredEntranceKey(hovered ? config.key : null)}
          />
        ))}

        {!isOverlayOpen
          ? summons.map((event) => <DashboardEventOrb key={event.id} event={event} onDismiss={(id) => setSummons((current) => current.filter((item) => item.id !== id))} onExpand={(stateKey) => setActiveStateKey(stateKey)} />)
          : null}

        <DashboardCenterOrb activeColor={activeModuleColor} onDragOffset={handleOrbDragOffset} onLongPress={() => setVoiceOpen(true)} visualState={centerVisualState} />
      </div>

      <div className={cn("dashboard-orbit-home__focus-bar", isOverlayOpen && "is-muted")}>
        <div className="dashboard-orbit-home__focus-main">
          <p className="dashboard-orbit-home__focus-eyebrow">现在最值得注意的</p>
          <p className="dashboard-orbit-home__focus-title">{currentFocusLine}</p>
          <p className="dashboard-orbit-home__focus-copy">{currentReasonLine}</p>
        </div>
        <div className="dashboard-orbit-home__focus-hint">
          <Sparkles className="h-4 w-4" />
          入口球负责跳页，事件球负责打开首页事件舱
        </div>
      </div>

      <DashboardEventPanel activeState={activeState} onClose={() => setActiveStateKey(null)} onStateChange={setActiveStateKey} />

      <DashboardVoiceField isOpen={voiceOpen} onClose={() => setVoiceOpen(false)} onCommand={handleModuleNavigate} sequences={dashboardVoiceSequences} />
    </ClickSpark>
  );
}
