// 该文件维护仪表盘页面状态。 
import { create } from "zustand";

// DashboardState 描述当前模块状态。
type DashboardState = {
  selectedPanel: "tasks" | "memory" | "safety" | "plugins";
  setSelectedPanel: (panel: DashboardState["selectedPanel"]) => void;
};

// useDashboardStore 暴露当前模块的状态容器。
export const useDashboardStore = create<DashboardState>((set) => ({
  selectedPanel: "tasks",
  setSelectedPanel: (selectedPanel) => set({ selectedPanel }),
}));
