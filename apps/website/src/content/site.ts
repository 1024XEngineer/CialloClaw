import { Rocket } from "lucide-react";
import faqMarkdown from "@/content/docs/faq.md?raw";
import quickStartMarkdown from "@/content/docs/quick-start.md?raw";
import scenariosMarkdown from "@/content/docs/scenarios.md?raw";
import securityMarkdown from "@/content/docs/security.md?raw";
import whatIsMarkdown from "@/content/docs/what-is.md?raw";
import workspaceMarkdown from "@/content/docs/workspace.md?raw";
import { parseDocsPage } from "@/lib/docs";

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
  markdown: string;
  outline: Array<{
    id: string;
    title: string;
  }>;
};

export const siteNav: SiteNavItem[] = [
  { label: "文档", href: "/docs/what-is" },
  { label: "博客 / 开发日志", href: "/blog" },
  { label: "关于", href: "/about" },
];

export const homeActions: HomeAction[] = [
  { label: "网页版", href: "/", primary: true },
  { label: "下载", href: "/docs/quick-start" },
  { label: "使用教程", href: "/docs/quick-start" },
];

export const docsPages: DocsPageEntry[] = [
  parseDocsPage(whatIsMarkdown),
  parseDocsPage(quickStartMarkdown),
  parseDocsPage(workspaceMarkdown),
  parseDocsPage(scenariosMarkdown),
  parseDocsPage(securityMarkdown),
  parseDocsPage(faqMarkdown),
];

export const docsSidebar: DocsSidebarSection[] = [
  {
    title: "文档",
    icon: Rocket,
    items: docsPages.map((page) => ({ title: page.title, href: page.path })),
  },
];

export function getDocsPage(pathname: string) {
  return docsPages.find((entry) => entry.path === pathname) ?? docsPages[0];
}
