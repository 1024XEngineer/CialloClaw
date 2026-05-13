import { useCallback, useEffect, useRef, useState } from "react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import type { CSSProperties } from "react";
import { useNavigate } from "react-router-dom";
import ClickSpark from "@/components/ClickSpark";
import { dashboardDecorOrbs, dashboardEntranceOrbs, dashboardModuleColors } from "@/features/dashboard/home/dashboardHome.config";
import type { DashboardHomeData } from "@/features/dashboard/home/dashboardHome.service";
import type { DashboardHomeEventStateKey, DashboardHomeModuleKey, DashboardHomeSummonEvent } from "@/features/dashboard/home/dashboardHome.types";
import { DashboardCenterOrb } from "@/features/dashboard/home/components/DashboardCenterOrb";
import { DashboardDecorOrb } from "@/features/dashboard/home/components/DashboardDecorOrb";
import { DashboardEntranceOrb } from "@/features/dashboard/home/components/DashboardEntranceOrb";
import { DashboardEventOrb } from "@/features/dashboard/home/components/DashboardEventOrb";
import { DashboardEventPanel } from "@/features/dashboard/home/components/DashboardEventPanel";
import { DashboardOrbitRings } from "@/features/dashboard/home/components/DashboardOrbitRings";
import { useDashboardEscapeHandler } from "@/features/dashboard/shared/dashboardEscapeCoordinator";
import { resolveDashboardModuleRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { buildDesktopOnboardingPresentation } from "@/features/onboarding/onboardingGeometry";
import { setDesktopOnboardingPresentation } from "@/features/onboarding/onboardingService";
import { useDesktopOnboardingActions } from "@/features/onboarding/useDesktopOnboardingActions";
import { useDesktopOnboardingLoading } from "@/features/onboarding/useDesktopOnboardingLoading";
import { useDesktopOnboardingSession } from "@/features/onboarding/useDesktopOnboardingSession";
import { openControlPanelFromTray } from "@/platform/trayController";
import { openOrFocusDesktopWindow } from "@/platform/windowController";
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

function pickNextSummonIndex(
  templates: Array<Omit<DashboardHomeSummonEvent, "id">>,
  previousIndex: number,
  previousModule: DashboardHomeModuleKey | null,
) {
  const total = templates.length;
  if (total <= 1) {
    return 0;
  }

  // Keep the very first visible orb aligned with the service-side priority
  // ordering so urgent overview signals are not randomized behind softer copy.
  if (previousIndex < 0 || previousModule === null) {
    return 0;
  }

  const candidateIndexes = templates
    .map((template, index) => ({ index, module: template.module }))
    .filter((candidate) => candidate.index !== previousIndex && candidate.module !== previousModule)
    .map((candidate) => candidate.index);

  const fallbackIndexes = templates
    .map((_, index) => index)
    .filter((index) => index !== previousIndex);

  const pool = candidateIndexes.length > 0 ? candidateIndexes : fallbackIndexes;
  const nextIndex = pool[Math.floor(Math.random() * pool.length)];
  if (nextIndex !== previousIndex) {
    return nextIndex;
  }

  return (nextIndex + 1) % total;
}

function buildSummonTemplateSignature(templates: Array<Omit<DashboardHomeSummonEvent, "id">>) {
  const buildNavigationTargetSignature = (
    target: NonNullable<DashboardHomeSummonEvent["expandedState"]>["navigationTarget"] | undefined,
  ) => {
    if (!target) {
      return "";
    }

    if (target.kind === "task_detail") {
      return [
        target.kind,
        target.module,
        target.label,
        target.taskId,
      ].join("::");
    }

    if (target.kind === "mirror_detail") {
      return [
        target.kind,
        target.module,
        target.label,
        target.activeDetailKey,
        target.focusMemoryId ?? "",
      ].join("::");
    }

    return [
      target.kind,
      target.module,
      target.label,
    ].join("::");
  };

  return templates
    .map((template) => [
      template.stateKey,
      template.module,
      template.message,
      template.reason,
      template.nextStep ?? "",
      template.priority,
      template.recommendationId ?? "",
      template.expandedState?.headline ?? "",
      template.expandedState?.subline ?? "",
      buildNavigationTargetSignature(template.expandedState?.navigationTarget),
    ].join("::"))
    .join("||");
}

type DashboardHomeProps = {
  data: DashboardHomeData;
  onVoiceOpen: () => void;
  onRecommendationFeedback?: (recommendationId: string, feedback: "positive" | "negative") => void;
  voiceOpen: boolean;
};

export function DashboardHome({
  data,
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
  const [activeExpandedState, setActiveExpandedState] = useState<DashboardHomeSummonEvent["expandedState"] | null>(null);
  const [summons, setSummons] = useState<DashboardHomeSummonEvent[]>([]);
  const summonIdRef = useRef(0);
  const lastSummonIndexRef = useRef(-1);
  const lastSummonModuleRef = useRef<DashboardHomeModuleKey | null>(null);
  const summonTimerRef = useRef<number | null>(null);
  const summonTemplatesRef = useRef(data.summonTemplates);
  const summonTemplateSignature = buildSummonTemplateSignature(data.summonTemplates);

  const activeState = activeExpandedState ?? (activeStateKey ? data.stateMap[activeStateKey] : null);
  const activeModule = hoveredEntranceKey
    ? dashboardEntranceOrbs.find((config) => config.key === hoveredEntranceKey)?.module ?? activeState?.module ?? null
    : activeState?.module ?? null;
  const activeModuleColor = activeModule ? dashboardModuleColors[activeModule].color : null;
  const hasLoadWarnings = data.loadWarnings.length > 0;
  const isOverlayOpen = Boolean(activeState || voiceOpen);

  const closeActiveOverlay = useCallback(() => {
    setActiveExpandedState(null);
    setActiveStateKey(null);
  }, []);

  const scheduleSummon = useCallback(() => {
    const templates = summonTemplatesRef.current;
    if (templates.length === 0) {
      return;
    }

    // Summons are local presentation state, so balancing the module order here
    // does not change the formal dashboard data contract or backend ranking.
    const nextIndex = pickNextSummonIndex(
      templates,
      lastSummonIndexRef.current,
      lastSummonModuleRef.current,
    );
    lastSummonIndexRef.current = nextIndex;
    const template = templates[nextIndex];
    lastSummonModuleRef.current = template.module;

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
  }, []);

  useEffect(() => {
    summonTemplatesRef.current = data.summonTemplates;
  }, [data.summonTemplates]);

  useEffect(() => {
    summonIdRef.current = 0;
    lastSummonIndexRef.current = -1;
    lastSummonModuleRef.current = null;
    setSummons([]);

    if (data.summonTemplates.length === 0) {
      return;
    }

    summonTimerRef.current = window.setTimeout(scheduleSummon, 2_500);

    return () => {
      if (summonTimerRef.current) {
        window.clearTimeout(summonTimerRef.current);
      }
    };
  }, [data.summonTemplates.length, scheduleSummon, summonTemplateSignature]);

  useDashboardEscapeHandler({
    enabled: activeStateKey !== null || Boolean(activeExpandedState),
    handleEscape: closeActiveOverlay,
    priority: 200,
  });
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      const tag = target?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA") {
        return;
      }

      if (event.key === "Escape" && (activeStateKey || activeExpandedState)) {
        event.preventDefault();
        closeActiveOverlay();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activeExpandedState, activeStateKey, closeActiveOverlay]);

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
        {hasLoadWarnings ? (
          <div className="dashboard-orbit-home__shortcut-pill dashboard-orbit-home__shortcut-pill--warn" title={data.loadWarnings.join(" | ")}>
            部分模块未同步
          </div>
        ) : null}
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
                onExpand={(expandedEvent) => {
                  setActiveStateKey(expandedEvent.stateKey);
                  setActiveExpandedState(expandedEvent.expandedState ?? null);
                  if (event.recommendationId) {
                    onRecommendationFeedback?.(event.recommendationId, "positive");
                  }
                }}
              />
            ))
          : null}

        <DashboardCenterOrb activeColor={activeModuleColor} onDragOffset={handleOrbDragOffset} onLongPress={onVoiceOpen} visualState={centerVisualState} />
      </div>

      <DashboardEventPanel
        activeState={activeState}
        onClose={closeActiveOverlay}
        onStateChange={(stateKey) => {
          setActiveExpandedState(null);
          setActiveStateKey(stateKey);
        }}
        stateGroups={data.stateGroups}
        stateMap={data.stateMap}
      />
    </ClickSpark>
  );
}
