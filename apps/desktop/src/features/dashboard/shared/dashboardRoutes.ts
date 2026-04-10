import { BrainCircuit, ListTodo, NotebookPen, ShieldCheck } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import flower1Image from "@/assets/lily-of-the-valley/flower1.png";
import flower2Image from "@/assets/lily-of-the-valley/flower2.png";
import flower3Image from "@/assets/lily-of-the-valley/flower3.png";
import flower4Image from "@/assets/lily-of-the-valley/flower4.png";

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
  flowerImage: string;
  futurePages: string[];
  flowerPosition: {
    left: string;
    top: string;
    stemHeight: string;
    stemRotate: string;
    imageWidth: string;
    swayDuration: string;
    swayDelay: string;
  };
};

// These coordinates pin each bloom to the visible arc of the branch PNG.
export const dashboardModules: DashboardModule[] = [
  {
    route: "tasks",
    path: "/tasks",
    title: "任务",
    englishTitle: "Task Flow",
    description: "查看任务链路、状态回显与正式交付入口。",
    accent: "#557247",
    icon: ListTodo,
    flowerImage: flower1Image,
    futurePages: ["任务列表", "任务详情", "交付结果"],
    flowerPosition: {
      left: "34.5%",
      top: "13.5%",
      stemHeight: "3.9rem",
      stemRotate: "16deg",
      imageWidth: "7.25rem",
      swayDuration: "5.4s",
      swayDelay: "-1.2s",
    },
  },
  {
    route: "notes",
    path: "/notes",
    title: "便签",
    englishTitle: "Notepad",
    description: "承接零散记录、草稿沉淀与转任务入口。",
    accent: "#70885f",
    icon: NotebookPen,
    flowerImage: flower2Image,
    futurePages: ["便签列表", "便签详情", "转任务"],
    flowerPosition: {
      left: "46%",
      top: "4.5%",
      stemHeight: "4.45rem",
      stemRotate: "7deg",
      imageWidth: "7.25rem",
      swayDuration: "5.8s",
      swayDelay: "-2.6s",
    },
  },
  {
    route: "memory",
    path: "/memory",
    title: "记忆",
    englishTitle: "Memory Mirror",
    description: "进入镜像概览、命中摘要与回填观察位。",
    accent: "#7d9270",
    icon: BrainCircuit,
    flowerImage: flower3Image,
    futurePages: ["镜像概览", "检索命中", "摘要回填"],
    flowerPosition: {
      left: "59.5%",
      top: "5%",
      stemHeight: "4.15rem",
      stemRotate: "-6deg",
      imageWidth: "7.25rem",
      swayDuration: "5.6s",
      swayDelay: "-3.4s",
    },
  },
  {
    route: "safety",
    path: "/safety",
    title: "安全",
    englishTitle: "Safety",
    description: "查看授权挂起、审计摘要与恢复点占位。",
    accent: "#5d6f52",
    icon: ShieldCheck,
    flowerImage: flower4Image,
    futurePages: ["授权请求", "审计记录", "恢复点"],
    flowerPosition: {
      left: "72.5%",
      top: "14.5%",
      stemHeight: "3.5rem",
      stemRotate: "-12deg",
      imageWidth: "7.25rem",
      swayDuration: "5.2s",
      swayDelay: "-0.8s",
    },
  },
];

export const dashboardModuleMap = Object.fromEntries(
  dashboardModules.map((module) => [module.route, module]),
) as Record<DashboardModuleRoute, DashboardModule>;
