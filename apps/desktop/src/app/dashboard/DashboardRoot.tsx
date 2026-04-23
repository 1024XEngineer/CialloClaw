import { useEffect, useRef, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { HashRouter, Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import "./dashboard.css";
import { DashboardVoiceField } from "@/features/dashboard/home/components/DashboardVoiceField";
import { getDashboardHomeFallbackData, loadDashboardHomeData, submitDashboardHomeRecommendationFeedback } from "@/features/dashboard/home/dashboardHome.service";
import { MemoryPage } from "@/features/dashboard/memory/MemoryPage";
import { NotesPage } from "@/features/dashboard/notes/NotesPage";
import { SafetyPage } from "@/features/dashboard/safety/SafetyPage";
import { resolveDashboardModuleRoutePath, resolveDashboardRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { TasksPage } from "@/features/dashboard/tasks/TasksPage";
import { cn } from "@/utils/cn";
import { DashboardHome } from "./DashboardHome";
import { subscribeApprovalPending, subscribeDeliveryReady, subscribeTaskUpdated } from "@/rpc/subscriptions";

function useDashboardDomainExpansion() {
  const [isOpening, setIsOpening] = useState(true);
  const hiddenRef = useRef(false);

  useEffect(() => {
    let frame = 0;
    let timeout = 0;

    const trigger = () => {
      window.cancelAnimationFrame(frame);
      window.clearTimeout(timeout);
      setIsOpening(true);
      frame = window.requestAnimationFrame(() => {
        setIsOpening(false);
      });
      // Hidden/background Tauri windows can miss the RAF edge and stay clipped.
      timeout = window.setTimeout(() => {
        setIsOpening(false);
      }, 720);
    };

    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        hiddenRef.current = true;
        return;
      }

      if (!hiddenRef.current) {
        return;
      }

      hiddenRef.current = false;
      trigger();
    };

    trigger();
    document.addEventListener("visibilitychange", handleVisibilityChange);

    return () => {
      window.cancelAnimationFrame(frame);
      window.clearTimeout(timeout);
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

  useEffect(() => {
    const clearTaskSubscription = subscribeTaskUpdated(() => {
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
