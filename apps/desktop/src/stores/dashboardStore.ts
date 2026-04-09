// 该文件维护仪表盘页面状态。 
import { create } from "zustand";
import type { DashboardModuleRoute } from "@/features/dashboard/shared/dashboardRoutes";

// DashboardState 描述当前模块状态。
type DashboardState = {
  hoveredModule: DashboardModuleRoute | null;
  setHoveredModule: (module: DashboardModuleRoute | null) => void;
};

// useDashboardStore 暴露当前模块的状态容器。
export const useDashboardStore = create<DashboardState>((set) => ({
  hoveredModule: null,
  setHoveredModule: (hoveredModule) => set({ hoveredModule }),
}));
