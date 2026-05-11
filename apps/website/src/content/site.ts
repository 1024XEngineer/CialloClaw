import { Rocket } from "lucide-react";
import faqMarkdown from "@/content/docs/faq.md?raw";
import quickStartMarkdown from "@/content/docs/quick-start.md?raw";
import scenariosMarkdown from "@/content/docs/scenarios.md?raw";
import securityMarkdown from "@/content/docs/security.md?raw";
import whatIsMarkdown from "@/content/docs/what-is.md?raw";
import workspaceMarkdown from "@/content/docs/workspace.md?raw";
import enFaqMarkdown from "@/content/docs/en/faq.md?raw";
import enQuickStartMarkdown from "@/content/docs/en/quick-start.md?raw";
import enScenariosMarkdown from "@/content/docs/en/scenarios.md?raw";
import enSecurityMarkdown from "@/content/docs/en/security.md?raw";
import enWhatIsMarkdown from "@/content/docs/en/what-is.md?raw";
import enWorkspaceMarkdown from "@/content/docs/en/workspace.md?raw";
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
  searchableSections: Array<{
    id: string;
    title: string;
    text: string;
  }>;
  outline: Array<{
    id: string;
    title: string;
  }>;
};

export const siteNav: SiteNavItem[] = [
  { label: "文档", href: "/docs/what-is" },
];

export const homeActions: HomeAction[] = [
  { label: "网页版", href: "/", primary: true },
  { label: "下载", href: "/docs/quick-start#下载与启动" },
  { label: "使用教程", href: "/docs/quick-start#配置模型" },
];

const zhDocsPages: DocsPageEntry[] = [
  parseDocsPage(whatIsMarkdown),
  parseDocsPage(quickStartMarkdown),
  parseDocsPage(workspaceMarkdown),
  parseDocsPage(scenariosMarkdown),
  parseDocsPage(securityMarkdown),
  parseDocsPage(faqMarkdown),
];

const enDocsPages: DocsPageEntry[] = [
  parseDocsPage(enWhatIsMarkdown),
  parseDocsPage(enQuickStartMarkdown),
  parseDocsPage(enWorkspaceMarkdown),
  parseDocsPage(enScenariosMarkdown),
  parseDocsPage(enSecurityMarkdown),
  parseDocsPage(enFaqMarkdown),
];

export function getDocsPages(locale: string): DocsPageEntry[] {
  return locale === "en" ? enDocsPages : zhDocsPages;
}

export function getDocsSidebar(locale: string): DocsSidebarSection[] {
  const pages = getDocsPages(locale);
  return [
    {
      title: locale === "en" ? "Documentation" : "文档",
      icon: Rocket,
      items: pages.map((page) => ({ title: page.title, href: page.path })),
    },
  ];
}

export function getDocsPage(pathname: string, locale: string): DocsPageEntry {
  const pages = getDocsPages(locale);
  return pages.find((entry) => entry.path === pathname) ?? pages[0];
}
