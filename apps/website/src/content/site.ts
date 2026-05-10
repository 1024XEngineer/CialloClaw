import { BookOpen, CalendarDays, Rocket, ScanFace, Users } from "lucide-react";

export type SiteNavItem = {
  label: string;
  href: string;
};

export type HomeAction = {
  label: string;
  href: string;
  primary?: boolean;
};

export type DocsSectionItem = {
  title: string;
  href: string;
};

export type DocsSidebarSection = {
  title: string;
  icon: typeof Rocket;
  items: DocsSectionItem[];
};

export type DocsPageEntry = {
  path: string;
  title: string;
  summary: string;
  sections: Array<{
    id: string;
    title: string;
    body: string[];
  }>;
};

export const siteNav: SiteNavItem[] = [
  { label: "文档", href: "/docs/overview" },
  { label: "博客 / 开发日志", href: "/blog" },
  { label: "关于", href: "/about" },
];

export const homeActions: HomeAction[] = [
  { label: "网页版", href: "/", primary: true },
  { label: "下载", href: "/docs/overview/versions" },
  { label: "使用教程", href: "/docs/overview" },
];

export const docsSidebar: DocsSidebarSection[] = [
  {
    title: "概览",
    icon: Rocket,
    items: [
      { title: "这是什么项目？", href: "/docs/overview" },
      { title: "版本与下载", href: "/docs/overview/versions" },
      { title: "为什么不是聊天窗口", href: "/docs/overview/interaction" },
      { title: "其他类似项目", href: "/docs/overview/other-projects" },
    ],
  },
  {
    title: "用户手册",
    icon: BookOpen,
    items: [
      { title: "桌面版快速开始", href: "/docs/manual/desktop" },
      { title: "网页版快速开始", href: "/docs/manual/web" },
      { title: "配置指南", href: "/docs/manual/config" },
    ],
  },
  {
    title: "贡献指南",
    icon: Users,
    items: [
      { title: "环境配置与基础准备", href: "/docs/contributing" },
      { title: "官网前端", href: "/docs/contributing/website" },
      { title: "设计资源", href: "/docs/contributing/design-resources" },
    ],
  },
  {
    title: "编年史",
    icon: CalendarDays,
    items: [
      { title: "第一期官网框架", href: "/docs/chronicles/website-v0.1.0" },
      { title: "产品主链路草图", href: "/docs/chronicles/product-foundation" },
    ],
  },
  {
    title: "角色",
    icon: ScanFace,
    items: [{ title: "CialloClaw Mascot", href: "/characters" }],
  },
];

export const docsPages: DocsPageEntry[] = [
  {
    path: "/docs/overview",
    title: "这是什么项目？",
    summary: "把 CialloClaw 看成一个桌面现场协作 Agent，而不是另一个聊天窗口。",
    sections: [
      {
        id: "tldr",
        title: "太长不看",
        body: [
          "你可以把 CialloClaw 理解为一个更贴近桌面现场的协作入口：语音、文本选中、文件拖拽都能直接进入任务。",
          "它对外围绕 task 组织，对内通过 run、step、event、tool_call 编排，并把结果统一交付到 delivery_result 和 artifact。",
        ],
      },
      {
        id: "why",
        title: "为什么这样做",
        body: [
          "我们不想让用户为了一个协作动作，总是先切进大聊天页、再补上下文、再担心副作用。",
          "所以官网和产品叙事都围绕同一条主链路：近场输入承接、任务创建、治理执行、正式交付。",
        ],
      },
    ],
  },
  {
    path: "/docs/overview/versions",
    title: "版本与下载",
    summary: "第一期先预留下载和版本页框架，方便后续接桌面包与更新日志。",
    sections: [
      {
        id: "channels",
        title: "可用入口",
        body: [
          "官网会逐步覆盖网页版介绍、桌面端下载、更新日志和安装说明。",
          "当前这版先把页面结构和跳转关系搭起来，后面再接真实下载地址。",
        ],
      },
    ],
  },
  {
    path: "/docs/overview/interaction",
    title: "为什么不是聊天窗口",
    summary: "解释悬浮球、轻量输入、任务详情和正式交付之间的关系。",
    sections: [
      {
        id: "entry",
        title: "入口设计",
        body: [
          "悬浮球是桌面默认入口，强调低打扰、低摩擦和现场承接。",
          "双击打开仪表盘，悬停显示轻量输入，长按优先进入语音表达。",
        ],
      },
      {
        id: "delivery",
        title: "结果分层",
        body: [
          "短结果留在气泡，长结果进入文档、文件定位或任务详情，不把所有输出都塞进一段聊天记录。",
        ],
      },
    ],
  },
  {
    path: "/docs/overview/other-projects",
    title: "其他类似项目",
    summary: "官网后续可在这里补充同类项目对比与参考。",
    sections: [
      {
        id: "compare",
        title: "对比方向",
        body: [
          "可以补充与 AIRI、OpenCode、OpenClaw 这类项目的定位对比，但这一版先保留结构。",
        ],
      },
    ],
  },
  {
    path: "/docs/manual/desktop",
    title: "桌面版快速开始",
    summary: "桌面版更适合近场协作、权限治理和正式结果承接。",
    sections: [
      {
        id: "desktop-start",
        title: "开始使用",
        body: [
          "第一期官网会把桌面端作为主推荐入口，后续补充安装包、平台说明和首次启动引导。",
        ],
      },
    ],
  },
  {
    path: "/docs/manual/web",
    title: "网页版快速开始",
    summary: "网页版适合轻体验和快速浏览，但完整协作能力会以桌面版为核心。",
    sections: [
      {
        id: "web-start",
        title: "网页入口",
        body: [
          "这一版先保留网页入口按钮和说明卡，等后续真实 Web 能力确定后再补全。",
        ],
      },
    ],
  },
  {
    path: "/docs/manual/config",
    title: "配置指南",
    summary: "后续可在这里承接模型配置、API Key、工作区和安全设置说明。",
    sections: [
      {
        id: "config",
        title: "配置范围",
        body: [
          "官网先提供文档壳子，真实设置项仍以桌面控制面板与正式 JSON-RPC 设置链路为准。",
        ],
      },
    ],
  },
  {
    path: "/docs/contributing",
    title: "环境配置与基础准备",
    summary: "告诉贡献者怎么运行、怎么找目录、怎么修改官网。",
    sections: [
      {
        id: "repo",
        title: "仓库结构",
        body: [
          "官网源码位于 apps/website，当前使用 React、Vite、Tailwind 和 React Router。",
          "这一版框架借鉴 AIRI 文档站的导航方式，但保留了 CialloClaw 自己的视觉和内容语义。",
        ],
      },
    ],
  },
  {
    path: "/docs/contributing/website",
    title: "官网前端",
    summary: "说明网站前端的页面分层、数据入口和后续改法。",
    sections: [
      {
        id: "frontend",
        title: "页面结构",
        body: [
          "首页和 Docs 页面共用一套网站布局；导航、按钮和侧边栏数据都集中在 site.ts。",
        ],
      },
    ],
  },
  {
    path: "/docs/contributing/design-resources",
    title: "设计资源",
    summary: "后续可在这里放海报、悬浮球实体、品牌色和插画资源。",
    sections: [
      {
        id: "assets",
        title: "资源说明",
        body: [
          "当前首页主视觉直接复用了桌面端宠物资源，后续可以替换成更完整的宣传插画和海报资产。",
        ],
      },
    ],
  },
  {
    path: "/docs/chronicles/website-v0.1.0",
    title: "第一期官网框架",
    summary: "记录官网第一期把 AIRI 风格框架迁移到 React 站点的结果。",
    sections: [
      {
        id: "milestone",
        title: "里程碑",
        body: [
          "已完成首页 Hero、三按钮入口、Docs 导航、左侧分组栏和页内目录。",
        ],
      },
    ],
  },
  {
    path: "/docs/chronicles/product-foundation",
    title: "产品主链路草图",
    summary: "把官网叙事和主产品链路保持一致。",
    sections: [
      {
        id: "task-flow",
        title: "主链路",
        body: [
          "输入承接 -> 意图确认 -> task 创建/更新 -> 风险治理 -> 正式交付 -> 仪表盘展示。",
        ],
      },
    ],
  },
];

export function getDocsPage(pathname: string) {
  return docsPages.find((entry) => entry.path === pathname) ?? docsPages[0];
}
