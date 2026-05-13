import { BrainCircuit, ListTodo, NotebookPen, ShieldCheck } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { resolveDashboardModuleRoutePath } from "./dashboardRouteTargets";

export type DashboardView = "home" | DashboardModuleRoute;
export type DashboardModuleRoute = "tasks" | "notes" | "memory" | "safety";

type DashboardModule = {
  route: DashboardModuleRoute;
  path: `/${DashboardModuleRoute}`;
  title: string;
  englishTitle: string;
  description: string;
  accent: string;
  icon: LucideIcon;
  futurePages: string[];
};

export const dashboardModules: DashboardModule[] = [
  {
    route: "tasks",
    path: resolveDashboardModuleRoutePath("tasks"),
    title: "任务",
    englishTitle: "Task Flow",
    description: "查看任务链路、状态回显与正式交付入口。",
    accent: "#557247",
    icon: ListTodo,
    futurePages: ["任务列表", "任务详情", "交付结果"],
  },
  {
    route: "notes",
    path: resolveDashboardModuleRoutePath("notes"),
    title: "便签",
    englishTitle: "Notepad",
    description: "承接零散记录、草稿沉淀与转任务入口。",
    accent: "#70885f",
    icon: NotebookPen,
    futurePages: ["便签列表", "便签详情", "转任务"],
  },
  {
    route: "memory",
    path: resolveDashboardModuleRoutePath("memory"),
    title: "记忆",
    englishTitle: "Memory Mirror",
    description: "进入镜像概览、命中摘要与回填观察位。",
    accent: "#7d9270",
    icon: BrainCircuit,
    futurePages: ["镜像概览", "检索命中", "摘要回填"],
  },
  {
    route: "safety",
    path: resolveDashboardModuleRoutePath("safety"),
    title: "安全",
    englishTitle: "Safety",
    description: "查看授权挂起、审计摘要与恢复点占位。",
    accent: "#5d6f52",
    icon: ShieldCheck,
    futurePages: ["授权请求", "审计记录", "恢复点"],
  },
];

export const dashboardModuleMap = Object.fromEntries(
  dashboardModules.map((module) => [module.route, module]),
) as Record<DashboardModuleRoute, DashboardModule>;
