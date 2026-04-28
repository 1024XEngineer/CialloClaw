import { useEffect, useRef, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { HashRouter, Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { DashboardVoiceField } from "@/features/dashboard/home/components/DashboardVoiceField";
import {
  getDashboardHomeFallbackData,
  loadDashboardHomeData,
  submitDashboardHomeRecommendationFeedback,
} from "@/features/dashboard/home/dashboardHome.service";
import { MemoryPage } from "@/features/dashboard/memory/MemoryPage";
import { NotesPage } from "@/features/dashboard/notes/NotesPage";
import { SafetyPage } from "@/features/dashboard/safety/SafetyPage";
import {
  dashboardTaskDetailNavigationEvent,
  navigateToDashboardTaskDetail,
  type DashboardTaskDetailOpenRequest,
} from "@/features/dashboard/shared/dashboardTaskDetailNavigation";
import { resolveDashboardModuleRoutePath, resolveDashboardRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { TasksPage } from "@/features/dashboard/tasks/TasksPage";
import { subscribeApprovalPending, subscribeDeliveryReady, subscribeTaskUpdated } from "@/rpc/subscriptions";
import { rememberConversationSessionFromTaskUpdated } from "@/services/conversationSessionService";
import { cn } from "@/utils/cn";
import { DashboardHome } from "./DashboardHome";
import { createDashboardOpeningTransitionController } from "./dashboardOpeningTransition";
import "./dashboard.css";

const DASHBOARD_TASK_DETAIL_REQUEST_MEMORY_MS = 5_000;

function useDashboardDomainExpansion() {
  const [isOpening, setIsOpening] = useState(true);

  useEffect(() => {
    let disposed = false;
    let clearWindowFocusListener: (() => void) | null = null;
    const controller = createDashboardOpeningTransitionController({
      cancelAnimationFrame: (handle) => window.cancelAnimationFrame(handle),
      clearTimeout: (handle) => window.clearTimeout(handle),
      getVisibilityState: () => document.visibilityState,
      requestAnimationFrame: (callback) => window.requestAnimationFrame(callback),
      setIsOpening,
      setTimeout: (callback, timeoutMs) => window.setTimeout(callback, timeoutMs),
    });

    const handleVisibilityChange = () => {
      controller.handleVisibilityChange();
    };

    controller.trigger();
    document.addEventListener("visibilitychange", handleVisibilityChange);
    void getCurrentWindow()
      .onFocusChanged(({ payload: focused }) => {
        controller.handleWindowFocusChanged(focused);
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
      controller.dispose();
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
  const isOpening = useDashboardDomainExpansion();
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
  const dashboardHomeData = dashboardHomeQuery.data ?? getDashboardHomeFallbackData();
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
    let cleanup: (() => void) | null = null;

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

        cleanup = unlisten;
      })
      .catch((error) => {
        console.warn("dashboard task-detail navigation listener failed", error);
      });

    return () => {
      disposed = true;
      cleanup?.();
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
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || target?.isContentEditable) {
        return;
      }

      if (event.key === "Escape") {
        if (voiceOpen) {
          event.preventDefault();
          setVoiceOpen(false);
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
  }, [navigate, voiceOpen]);

  const handleRecommendationFeedback = (recommendationId: string, feedback: "positive" | "negative") => {
    recommendationFeedbackMutation.mutate({ feedback, recommendationId });
  };

  return (
    <div className={cn("dashboard-app", isOpening && "is-opening")}>
      <AnimatePresence mode="wait">
        <motion.div
          key={location.pathname}
          animate={{ opacity: 1, scale: 1, y: 0 }}
          className="dashboard-route-layer"
          exit={{ opacity: 0, scale: 0.988, y: -16 }}
          initial={{ opacity: 0, scale: 0.958, y: 30 }}
          style={{ transformOrigin: "50% 53.2%" }}
          transition={{ duration: 0.46, ease: [0.22, 1, 0.36, 1] }}
        >
          <Routes location={location}>
            <Route
              element={
                <DashboardHome
                  data={dashboardHomeData}
                  onRecommendationFeedback={handleRecommendationFeedback}
                  onVoiceOpen={() => setVoiceOpen(true)}
                  voiceOpen={voiceOpen}
                />
              }
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
        sequences={dashboardHomeData.voiceSequences}
      />
    </div>
  );
}

export function DashboardRoot() {
  return (
    <HashRouter>
      <DashboardRoutes />
    </HashRouter>
  );
}
