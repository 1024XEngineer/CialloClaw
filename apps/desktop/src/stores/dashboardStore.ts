// 该文件维护仪表盘页面状态。 
import { create } from "zustand";

type DashboardState = {
  selectedPanel: "tasks" | "memory" | "safety" | "plugins";
  setSelectedPanel: (panel: DashboardState["selectedPanel"]) => void;
};

export const useDashboardStore = create<DashboardState>((set) => ({
  selectedPanel: "tasks",
  setSelectedPanel: (selectedPanel) => set({ selectedPanel }),
}));
