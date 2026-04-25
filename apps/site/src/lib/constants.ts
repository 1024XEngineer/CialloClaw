export const SITE_URL = "https://cialloclaw.vercel.app";

export const PRIMARY_NAV_ITEMS = [
  { href: "#product", label: "产品" },
  { href: "#workflow", label: "工作方式" },
  { href: "#safety", label: "安全" },
  { href: "#download", label: "下载" },
  { href: "/docs", label: "文档" },
  { href: "https://github.com/1024XEngineer/CialloClaw", label: "GitHub" },
] as const;

export const DOC_LINKS = [
  {
    href: "/docs#architecture-overview",
    label: "架构总览",
    description: "了解 task、run、delivery_result、artifact 与治理链路如何组成同一条正式主线。",
  },
  {
    href: "/docs#product-interaction-design",
    label: "产品交互设计",
    description: "查看悬浮球、轻量输入和仪表盘如何组成低打扰的桌面协作体验。",
  },
  {
    href: "/docs#dashboard-design",
    label: "仪表盘设计",
    description: "查看 task-centric 工作台如何承接任务进度、信任摘要和正式结果。",
  },
  {
    href: "/docs#control-panel-settings",
    label: "控制面板设置",
    description: "在同一处查看 provider、记忆、自动化和安全边界设置。",
  },
  {
    href: "/docs#protocol-design",
    label: "协议设计",
    description: "查看让桌面 UI 与本地 harness 保持一致的 JSON-RPC 正式边界。",
  },
  {
    href: "https://github.com/1024XEngineer/CialloClaw/issues/332#issue-4321666828",
    label: "Issue #332",
    description: "查看官网动态体验与 Release 同步需求的原始任务说明。",
  },
] as const;

export const FAQ_ITEMS = [
  {
    question: "CialloClaw 是聊天机器人吗？",
    answer:
      "不是。CialloClaw 是一个 Windows first 的桌面协作 Agent。它以近场入口、task-centric 编排和正式的 delivery_result / artifact 输出为核心，而不是把聊天窗口当成整个产品。",
  },
  {
    question: "目前主要支持哪些平台？",
    answer:
      "当前产品方向和工程基线都是 Windows first。桌面 runtime、本地 harness 和任务流都围绕 Windows 桌面工作现场设计。",
  },
  {
    question: "为什么悬浮球会被反复强调？",
    answer:
      "因为悬浮球是低摩擦入口。它允许用户从文本选中、文件拖拽、当前错误、悬停输入或语音承接开始，而不是先切进一个笨重的聊天窗口。",
  },
  {
    question: "高风险动作会怎么处理？",
    answer:
      "文件写入、命令执行、工作区边界变化等高风险动作，都会先进入 approval_request、authorization_record、audit_record 和 recovery_point 保护链路，再进入正式交付。",
  },
  {
    question: "Stable 和 Tip Preview 有什么区别？",
    answer:
      "Stable 是推荐给大多数用户的稳定通道。Tip Preview 跟踪更快更新的 tag，更适合开发者和愿意抢先体验新变化的早期测试者。",
  },
  {
    question: "官网的下载区会直接连接本地桌面 runtime 吗？",
    answer:
      "不会。官网刻意与 apps/desktop、local-service、workers、Named Pipe 和本地存储解耦，它只负责展示产品信息和同步后的 release 元数据。",
  },
] as const;
