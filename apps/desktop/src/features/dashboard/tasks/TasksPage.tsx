import { Navigate, Route, Routes } from "react-router-dom";
import { resolveDashboardModuleRoutePath } from "@/features/dashboard/shared/dashboardRouteTargets";
import { TaskDeliveryPage } from "./TaskDeliveryPage";
import { TaskPage } from "./TaskPage";
import { dashboardTaskDeliveryRoutePattern } from "./taskDeliveryNavigation";

/**
 * Hosts the task workspace routes so the task list/detail view and the formal
 * delivery page stay inside the same dashboard module.
 */
export function TasksPage() {
  return (
    <Routes>
      <Route element={<TaskPage />} index />
      <Route element={<TaskDeliveryPage />} path={dashboardTaskDeliveryRoutePattern} />
      <Route element={<Navigate replace to={resolveDashboardModuleRoutePath("tasks")} />} path="*" />
    </Routes>
  );
}
