import { useEffect, useMemo, useRef, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { HashRouter, Link, Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { DashboardVoiceField } from "@/features/dashboard/home/components/DashboardVoiceField";
import {
  loadDashboardHomeData,
  submitDashboardHomeRecommendationFeedback,
} from "@/features/dashboard/home/dashboardHome.service";
import { MemoryPage } from "@/features/dashboard/memory/MemoryPage";
import { NotesPage } from "@/features/dashboard/notes/NotesPage";
import { SafetyPage } from "@/features/dashboard/safety/SafetyPage";
import { DashboardModuleFloatingNav } from "@/features/dashboard/shared/DashboardModuleFloatingNav";
import {
  dashboardTaskDetailNavigationEvent,
  navigateToDashboardTaskDetail,
  type DashboardTaskDetailOpenRequest,
} from "@/features/dashboard/shared/dashboardTaskDetailNavigation";
import {
  DashboardEscapeCoordinatorProvider,
  useDashboardEscapeCoordinator,
  useDashboardEscapeHandler,
} from "@/features/dashboard/shared/dashboardEscapeCoordinator";
import { dashboardModules } from "@/features/dashboard/shared/dashboardRoutes";
import { resolveDashboardModuleRoutePath, resolveDashboardRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import {
  dashboardTaskDeliveryNavigationEvent,
  navigateToDashboardTaskDelivery,
  type DashboardTaskDeliveryOpenRequest,
} from "@/features/dashboard/tasks/taskDeliveryNavigation";
import { TasksPage } from "@/features/dashboard/tasks/TasksPage";
import { subscribeApprovalPending, subscribeDeliveryReady, subscribeTaskUpdated } from "@/rpc/subscriptions";
import { rememberConversationSessionFromTaskUpdated } from "@/services/conversationSessionService";
import { cn } from "@/utils/cn";
import { DashboardHome } from "./DashboardHome";
import { suppressDashboardEscapeClose } from "./dashboardEscapeCloseGuard";
import { createDashboardOpeningTransitionController } from "./dashboardOpeningTransition";
import "./dashboard.css";

const DASHBOARD_TASK_DETAIL_REQUEST_MEMORY_MS = 5_000;

/**
 * Replays the dashboard opening mask after a hidden desktop window becomes
 * visible again so long-idle sessions do not stay visually clipped.
 */
function useDashboardOpeningTransitionState() {
  const [isOpening, setIsOpening] = useState(true);

  useEffect(() => {
    let disposed = false;
    let clearWindowFocusListener: (() => void) | null = null;
    // Keep the visibility/focus recovery state machine outside the hook so the
    // long-idle window path stays contract-testable without mounting Tauri.
    const openingTransitionController = createDashboardOpeningTransitionController({
      cancelAnimationFrame: (handle) => window.cancelAnimationFrame(handle),
      clearTimeout: (handle) => window.clearTimeout(handle),
      hasFocus: () => document.hasFocus(),
      getVisibilityState: () => document.visibilityState,
      requestAnimationFrame: (callback) => window.requestAnimationFrame(callback),
      setIsOpening,
      setTimeout: (callback, timeoutMs) => window.setTimeout(callback, timeoutMs),
    });

    const handleVisibilityChange = () => {
      openingTransitionController.handleVisibilityChange();
    };

    openingTransitionController.trigger();
    document.addEventListener("visibilitychange", handleVisibilityChange);
    void getCurrentWindow()
      .onFocusChanged(({ payload: focused }) => {
        openingTransitionController.handleWindowFocusChanged(focused);
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        clearWindowFocusListener = unlisten;
      })
      .catch((error) => {
        console.warn("dashboard focus listener failed", error);
      });

    return () => {
      disposed = true;
      openingTransitionController.dispose();
      clearWindowFocusListener?.();
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, []);

  return isOpening;
}

function DashboardRoutes() {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const escapeCoordinator = useDashboardEscapeCoordinator();
  const isOpening = useDashboardOpeningTransitionState();
  const [voiceOpen, setVoiceOpen] = useState(false);
  const handledTaskDetailRequestIdsRef = useRef<Map<string, number>>(new Map());
  const dashboardHomeQuery = useQuery({
    queryKey: ["dashboard", "home"],
    queryFn: loadDashboardHomeData,
    placeholderData: (previousData) => previousData,
    refetchOnMount: false,
    refetchOnReconnect: false,
    refetchOnWindowFocus: false,
    retry: false,
  });
  const dashboardHomeData = dashboardHomeQuery.data ?? null;
  const isHomeRoute = location.pathname === resolveDashboardRoutePath("home");
  const recommendationFeedbackMutation = useMutation({
    mutationFn: ({ feedback, recommendationId }: { feedback: "positive" | "negative"; recommendationId: string }) =>
      submitDashboardHomeRecommendationFeedback(recommendationId, feedback),
    retry: false,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["dashboard", "home"] });
    },
    onError: (error) => {
      console.warn("dashboard recommendation feedback failed", error);
    },
  });

  /**
   * Retries for task-detail open requests intentionally reuse the same
   * `request_id`, so the dashboard must remember more than the latest value.
   * Otherwise an older delayed retry can arrive after a newer request and
   * incorrectly navigate the window back to the stale task detail.
   */
  function rememberHandledTaskDetailRequest(requestId: string) {
    const now = Date.now();
    const handledRequestIds = handledTaskDetailRequestIdsRef.current;

    for (const [handledRequestId, handledAt] of handledRequestIds) {
      if (now - handledAt > DASHBOARD_TASK_DETAIL_REQUEST_MEMORY_MS) {
        handledRequestIds.delete(handledRequestId);
      }
    }

    if (handledRequestIds.has(requestId)) {
      return false;
    }

    handledRequestIds.set(requestId, now);
    return true;
  }

  useEffect(() => {
    let disposed = false;
    let clearTaskDetailNavigation: (() => void) | null = null;
    let clearTaskDeliveryNavigation: (() => void) | null = null;

    void getCurrentWindow()
      .listen<DashboardTaskDetailOpenRequest>(dashboardTaskDetailNavigationEvent, ({ payload }) => {
        if (!rememberHandledTaskDetailRequest(payload.request_id)) {
          return;
        }

        setVoiceOpen(false);
        navigateToDashboardTaskDetail(navigate, payload.task_id);
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        clearTaskDetailNavigation = unlisten;
      })
      .catch((error) => {
        console.warn("dashboard task-detail navigation listener failed", error);
      });

    void getCurrentWindow()
      .listen<DashboardTaskDeliveryOpenRequest>(dashboardTaskDeliveryNavigationEvent, ({ payload }) => {
        if (!rememberHandledTaskDetailRequest(payload.request_id)) {
          return;
        }

        setVoiceOpen(false);
        navigateToDashboardTaskDelivery(navigate, payload.task_id);
      })
      .then((unlisten) => {
        if (disposed) {
          unlisten();
          return;
        }

        clearTaskDeliveryNavigation = unlisten;
      })
      .catch((error) => {
        console.warn("dashboard task-delivery navigation listener failed", error);
      });

    return () => {
      disposed = true;
      clearTaskDetailNavigation?.();
      clearTaskDeliveryNavigation?.();
    };
  }, [navigate]);

  useEffect(() => {
    const clearTaskSubscription = subscribeTaskUpdated((payload) => {
      rememberConversationSessionFromTaskUpdated(payload);
      void queryClient.invalidateQueries({ queryKey: ["dashboard", "home"] });
    });

    const clearApprovalSubscription = subscribeApprovalPending(() => {
      void queryClient.invalidateQueries({ queryKey: ["dashboard", "home"] });
    });

    const clearDeliverySubscription = subscribeDeliveryReady(() => {
      void queryClient.invalidateQueries({ queryKey: ["dashboard", "home"] });
    });

    return () => {
      clearTaskSubscription();
      clearApprovalSubscription();
      clearDeliverySubscription();
    };
  }, [queryClient]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      const tag = target?.tagName;
      if (event.defaultPrevented) {
        return;
      }

      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || target?.isContentEditable) {
        return;
      }

      if (event.key === "Escape") {
        if (escapeCoordinator.consumeEscape()) {
          suppressDashboardEscapeClose();
          event.preventDefault();
          return;
        }

        if (!isHomeRoute) {
          suppressDashboardEscapeClose();
          event.preventDefault();
          navigate(resolveDashboardRoutePath("home"));
        }

        return;
      }

      if (!event.ctrlKey && !event.metaKey) {
        return;
      }

      if (event.key === "1") {
        event.preventDefault();
        setVoiceOpen(false);
        navigate(resolveDashboardModuleRoutePath("tasks"));
      }

      if (event.key === "2") {
        event.preventDefault();
        setVoiceOpen(false);
        navigate(resolveDashboardModuleRoutePath("notes"));
      }

      if (event.key === "3") {
        event.preventDefault();
        setVoiceOpen(false);
        navigate(resolveDashboardModuleRoutePath("memory"));
      }

      if (event.key === "4") {
        event.preventDefault();
        setVoiceOpen(false);
        navigate(resolveDashboardModuleRoutePath("safety"));
      }

      if (event.key === "5") {
        event.preventDefault();
        setVoiceOpen(true);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [escapeCoordinator, isHomeRoute, navigate]);

  useDashboardEscapeHandler({
    enabled: voiceOpen,
    handleEscape: () => setVoiceOpen(false),
    priority: 300,
  });

  const handleRecommendationFeedback = (recommendationId: string, feedback: "positive" | "negative") => {
    recommendationFeedbackMutation.mutate({ feedback, recommendationId });
  };

  const dashboardHomeRoute = dashboardHomeData
    ? (
        <DashboardHome
          data={dashboardHomeData}
          onRecommendationFeedback={handleRecommendationFeedback}
          onVoiceOpen={() => setVoiceOpen(true)}
          voiceOpen={voiceOpen}
        />
      )
    : (
        <DashboardHomeStatusShell
          title={dashboardHomeQuery.isError ? "首页同步失败" : "正在同步首页轨道"}
          message={dashboardHomeQuery.isError
            ? (dashboardHomeQuery.error instanceof Error ? dashboardHomeQuery.error.message : "首页总览请求失败")
            : "正在连接任务、便签、镜子与安全模块的正式摘要。"}
          onRetry={dashboardHomeQuery.isError ? () => void dashboardHomeQuery.refetch() : null}
        />
      );

  const activeSharedModule = useMemo(
    () => dashboardModules.find((module) => module.path === location.pathname) ?? null,
    [location.pathname],
  );

  return (
    <div className={cn("dashboard-app", isOpening && "is-opening")}>
      {activeSharedModule ? (
        <DashboardModuleFloatingNav accentColor={activeSharedModule.accent} includeHomeLink />
      ) : null}
      <AnimatePresence mode="wait">
        <motion.div
          key={location.pathname}
          animate={{ opacity: 1, scale: 1, y: 0 }}
          className={cn("dashboard-route-layer", activeSharedModule && "dashboard-route-layer--with-shared-topbar")}
          exit={{ opacity: 0, scale: 0.988, y: -16 }}
          initial={{ opacity: 0, scale: 0.958, y: 30 }}
          style={{ transformOrigin: "50% 53.2%" }}
          transition={{ duration: 0.46, ease: [0.22, 1, 0.36, 1] }}
        >
          <Routes location={location}>
            <Route
              element={dashboardHomeRoute}
              path={resolveDashboardRoutePath("home")}
            />
            <Route element={<TasksPage />} path={`${resolveDashboardModuleRoutePath("tasks")}/*`} />
            <Route element={<NotesPage />} path={`${resolveDashboardModuleRoutePath("notes")}/*`} />
            <Route element={<MemoryPage />} path={`${resolveDashboardModuleRoutePath("memory")}/*`} />
            <Route element={<SafetyPage />} path={`${resolveDashboardModuleRoutePath("safety")}/*`} />
            <Route element={<Navigate replace to={resolveDashboardRoutePath("home")} />} path="*" />
          </Routes>
        </motion.div>
      </AnimatePresence>
      <DashboardVoiceField
        isOpen={voiceOpen}
        onClose={() => setVoiceOpen(false)}
        onRecommendationConfirm={(recommendationId) => {
          recommendationFeedbackMutation.mutate({ feedback: "positive", recommendationId });
        }}
        sequences={dashboardHomeData?.voiceSequences ?? []}
      />
    </div>
  );
}

type DashboardHomeStatusShellProps = {
  title: string;
  message: string;
  onRetry: (() => void) | null;
};

const dashboardHomeStatusShellModules = [
  { label: "任务", route: resolveDashboardModuleRoutePath("tasks") },
  { label: "便签", route: resolveDashboardModuleRoutePath("notes") },
  { label: "镜子", route: resolveDashboardModuleRoutePath("memory") },
  { label: "安全", route: resolveDashboardModuleRoutePath("safety") },
] as const;

function DashboardHomeStatusShell({ title, message, onRetry }: DashboardHomeStatusShellProps) {
  return (
    <main className="dashboard-home dashboard-home--status">
      <section className="dashboard-home__status-card">
        <p className="dashboard-page__eyebrow">dashboard</p>
        <h1 className="dashboard-home__status-title">{title}</h1>
        <p className="dashboard-home__status-copy">{message}</p>
        <div className="dashboard-home__status-links">
          {dashboardHomeStatusShellModules.map((module) => (
            <Link key={module.route} className="dashboard-home__status-link" to={module.route}>
              打开{module.label}
            </Link>
          ))}
        </div>
        {onRetry ? (
          <button className="dashboard-home__status-action" onClick={onRetry} type="button">
            重试
          </button>
        ) : null}
      </section>
    </main>
  );
}

/**
 * Mounts the dashboard router tree for the dedicated desktop window.
 */
export function DashboardRoot() {
  return (
    <HashRouter>
      <DashboardEscapeCoordinatorProvider>
        <DashboardRoutes />
      </DashboardEscapeCoordinatorProvider>
    </HashRouter>
  );
}
