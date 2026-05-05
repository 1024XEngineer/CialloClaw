import { useCallback, useEffect, useRef, useState } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import type { CSSProperties } from "react";
import { Keyboard, Sparkles } from "lucide-react";
import { useNavigate } from "react-router-dom";
import ClickSpark from "@/components/ClickSpark";
import { dashboardDecorOrbs, dashboardEntranceOrbs, dashboardModuleColors } from "@/features/dashboard/home/dashboardHome.config";
import { getDashboardHomeFallbackData, type DashboardHomeData } from "@/features/dashboard/home/dashboardHome.service";
import type { DashboardHomeEventStateKey, DashboardHomeModuleKey, DashboardHomeSummonEvent } from "@/features/dashboard/home/dashboardHome.types";
import { DashboardCenterOrb } from "@/features/dashboard/home/components/DashboardCenterOrb";
import { DashboardDecorOrb } from "@/features/dashboard/home/components/DashboardDecorOrb";
import { DashboardEntranceOrb } from "@/features/dashboard/home/components/DashboardEntranceOrb";
import { DashboardEventOrb } from "@/features/dashboard/home/components/DashboardEventOrb";
import { DashboardEventPanel } from "@/features/dashboard/home/components/DashboardEventPanel";
import { DashboardOrbitRings } from "@/features/dashboard/home/components/DashboardOrbitRings";
import { resolveDashboardModuleRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { buildDesktopOnboardingPresentation } from "@/features/onboarding/onboardingGeometry";
import { setDesktopOnboardingPresentation } from "@/features/onboarding/onboardingService";
import { useDesktopOnboardingActions } from "@/features/onboarding/useDesktopOnboardingActions";
import { useDesktopOnboardingLoading } from "@/features/onboarding/useDesktopOnboardingLoading";
import { useDesktopOnboardingSession } from "@/features/onboarding/useDesktopOnboardingSession";
import { openControlPanelFromTray } from "@/platform/trayController";
import { openOrFocusDesktopWindow } from "@/platform/windowController";
import { cn } from "@/utils/cn";
import "@/features/shell-ball/shellBall.css";
import "@/features/dashboard/home/dashboardHome.css";

function getRouteForModule(module: DashboardHomeModuleKey) {
  return resolveDashboardModuleRoutePath(module);
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

type DashboardHomeProps = {
  data?: DashboardHomeData;
  onVoiceOpen: () => void;
  onRecommendationFeedback?: (recommendationId: string, feedback: "positive" | "negative") => void;
  voiceOpen: boolean;
};

export function DashboardHome({
  data = getDashboardHomeFallbackData(),
  onVoiceOpen,
  onRecommendationFeedback,
  voiceOpen,
}: DashboardHomeProps) {
  const onboardingSession = useDesktopOnboardingSession();
  const onboardingLoading = useDesktopOnboardingLoading("dashboard");
  const navigate = useNavigate();
  const [orbDragOffset, setOrbDragOffset] = useState({ x: 0, y: 0 });
  const [hoveredEntranceKey, setHoveredEntranceKey] = useState<string | null>(null);
  const [activeStateKey, setActiveStateKey] = useState<DashboardHomeEventStateKey | null>(null);
  const [summons, setSummons] = useState<DashboardHomeSummonEvent[]>([]);
  const summonIndexRef = useRef(0);
  const summonIdRef = useRef(0);
  const summonTimerRef = useRef<number | null>(null);

  const activeState = activeStateKey ? data.stateMap[activeStateKey] : null;
  const activeModule = hoveredEntranceKey
    ? dashboardEntranceOrbs.find((config) => config.key === hoveredEntranceKey)?.module ?? activeState?.module ?? null
    : activeState?.module ?? null;
  const activeModuleColor = activeModule ? dashboardModuleColors[activeModule].color : null;
  const currentFocusLine = activeState?.headline ?? summons[0]?.message ?? data.focusLine.headline;
  const currentReasonLine = activeState?.subline ?? summons[0]?.reason ?? data.focusLine.reason;
  const isOverlayOpen = Boolean(activeState || voiceOpen);

  const scheduleSummon = useCallback(() => {
    const template = data.summonTemplates[summonIndexRef.current % data.summonTemplates.length];
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

    const gap = (template.duration ?? 5_000) + 7_000;
    summonTimerRef.current = window.setTimeout(scheduleSummon, gap);
  }, [data.summonTemplates]);

  useEffect(() => {
    summonIndexRef.current = 0;
    summonIdRef.current = 0;
    setSummons([]);

    summonTimerRef.current = window.setTimeout(scheduleSummon, 2_500);

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

      if (event.key === "Escape" && activeStateKey) {
        event.preventDefault();
        setActiveStateKey(null);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activeStateKey]);

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

  const handleModuleNavigate = useCallback(
    (module: DashboardHomeModuleKey) => {
      const nextPath = getRouteForModule(module);
      navigate(nextPath);
    },
    [navigate],
  );

  useDesktopOnboardingActions(
    "dashboard",
    useCallback((action) => {
      if (action.type === "open_control_panel") {
        void openControlPanelFromTray();
      }

      if (action.type === "open_dashboard") {
        void openOrFocusDesktopWindow("dashboard");
      }

      if (action.type === "close_dashboard") {
        void getCurrentWindow().close();
      }
    }, []),
  );

  useEffect(() => {
    if (
      onboardingSession?.isOpen !== true ||
      (onboardingSession.step !== "dashboard_overview" && onboardingSession.step !== "tray_hint")
    ) {
      return;
    }

    void (async () => {
      const presentation = await buildDesktopOnboardingPresentation({
        anchors: [],
        placement: "top-right",
        step: onboardingSession.step,
        windowLabel: "dashboard",
      });

      if (presentation !== null) {
        await setDesktopOnboardingPresentation(presentation);
      }
    })();
  }, [onboardingSession]);

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
        {onboardingLoading ? <div className="dashboard-orbit-home__shortcut-pill">{onboardingLoading.message}</div> : null}
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
          ? summons.map((event) => (
              <DashboardEventOrb
                key={event.id}
                event={event}
                stateMap={data.stateMap}
                onDismiss={(id) => {
                  setSummons((current) => current.filter((item) => item.id !== id));
                  if (event.recommendationId) {
                    onRecommendationFeedback?.(event.recommendationId, "negative");
                  }
                }}
                onExpand={(stateKey) => {
                  setActiveStateKey(stateKey);
                  if (event.recommendationId) {
                    onRecommendationFeedback?.(event.recommendationId, "positive");
                  }
                }}
              />
            ))
          : null}

        <DashboardCenterOrb activeColor={activeModuleColor} onDragOffset={handleOrbDragOffset} onLongPress={onVoiceOpen} visualState={centerVisualState} />
      </div>

      <div className={cn("dashboard-orbit-home__focus-bar", isOverlayOpen && "is-muted")}>
        <div className="dashboard-orbit-home__focus-main">
          <p className="dashboard-orbit-home__focus-eyebrow">现在最值得注意的</p>
          <p className="dashboard-orbit-home__focus-title">{currentFocusLine}</p>
          <p className="dashboard-orbit-home__focus-copy">{currentReasonLine}</p>
        </div>
        <div className="dashboard-orbit-home__focus-hint">
          <Sparkles className="h-4 w-4" />
          入口球负责跳页，事件球负责展开首页实时信号。
        </div>
      </div>

      <DashboardEventPanel activeState={activeState} onClose={() => setActiveStateKey(null)} onStateChange={setActiveStateKey} stateGroups={data.stateGroups} stateMap={data.stateMap} />
    </ClickSpark>
  );
}
