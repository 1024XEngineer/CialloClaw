/**
 * Maps Chinese heading text to its canonical English anchor ID so that
 * anchors are always ASCII (no encoding issues in GitHub Pages project-page
 * URLs) and match the English heading slugs regardless of the active locale.
 *
 * Add new entries here when a new Chinese heading is introduced.
 */
const CANONICAL_HEADING_ID: Record<string, string> = {
  /* quick-start.md */
  "下载与启动": "download-and-launch",
  "下载": "download",
  "下载入口：": "download-options",
  "最新版安装": "install-the-latest-version",
  "先行版安装": "install-the-preview-version",
  "第一步：获取源码": "step-1-get-the-source-code",
  "方式一：使用 Git 克隆仓库": "option-1-clone-with-git",
  "方式二：下载 ZIP": "option-2-download-zip",
  "第二步：安装依赖": "step-2-install-dependencies",
  "第三步：启动本地服务": "step-3-start-the-local-service",
  "第四步：启动桌面端": "step-4-launch-the-desktop-app",
  "先行版启动流程总结": "preview-version-startup-summary",
  "配置模型": "configure-a-model",
  "打开控制面板": "open-the-control-panel",
  "填写模型信息": "fill-in-the-model-information",
  "尝试使用": "try-it-out",
  "输入一句话": "send-a-message",
  "长按语音": "long-press-the-floating-ball",
  "双击打开工作台": "double-click-to-open-the-workspace",
  "常用动作速览": "common-actions-overview",
  /* scenarios.md */
  "总结网页": "summarise-a-webpage",
  "翻译或解释文本": "translate-or-explain-text",
  "分析文件": "analyse-a-file",
  "解释报错": "explain-an-error",
  "生成草稿": "draft-content",
  /* faq.md */
  "工作台白屏了怎么办？": "the-workspace-is-blank-what-can-i-do",
  "悬浮球不见了怎么办？": "the-floating-ball-is-missing",
  "输入后没有回复怎么办？": "no-response-after-typing",
  "语音不能用怎么办？": "voice-input-is-not-working",
  "先行版启动失败怎么办？": "the-preview-version-fails-to-start",
  "最新版和先行版有什么区别？": "what-is-the-difference-between-the-latest-and-preview-versions",
  "CialloClaw 是免费的吗？": "is-cialloclaw-free",
};

export type DocsOutlineItem = {
  id: string;
  title: string;
};

export type DocsSearchSection = {
  id: string;
  title: string;
  text: string;
};

export type ParsedDocsPage = {
  path: string;
  title: string;
  summary: string;
  markdown: string;
  outline: DocsOutlineItem[];
  searchableSections: DocsSearchSection[];
};

const DOCS_PATH_MAP: Record<string, string> = {
  "CialloClaw 是什么？": "/docs/what-is",
  "What is CialloClaw?": "/docs/what-is",
  "快速上手": "/docs/quick-start",
  "Quick Start": "/docs/quick-start",
  "初认工作台": "/docs/workspace",
  "Meet the Workspace": "/docs/workspace",
  "使用场景": "/docs/scenarios",
  "Usage Scenarios": "/docs/scenarios",
  "安全与隐私": "/docs/security",
  "Security & Privacy": "/docs/security",
  "常见问题": "/docs/faq",
  "Frequently Asked Questions": "/docs/faq",
};

/**
 * Derives a URL-safe anchor ID from a heading title.
 *
 * Chinese characters are stripped so that anchors remain ASCII-only, which
 * avoids encoding problems in GitHub Pages project-page URLs.
 *
 * When the result would be empty (purely Chinese heading) a deterministic
 * short hash of the original title is used as a fallback.
 */
export function createHeadingId(title: string) {
  const trimmed = title.trim();
  const canonical = CANONICAL_HEADING_ID[trimmed];
  if (canonical) return canonical;

  const cleaned = trimmed
    .toLowerCase()
    .replace(/[“”"'`]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");

  if (cleaned) return cleaned;

  // Deterministic hash so the same heading always produces the same anchor.
  let hash = 0;
  for (let i = 0; i < trimmed.length; i++) {
    hash = ((hash << 5) - hash) + trimmed.charCodeAt(i);
    hash |= 0; // enforce 32-bit integer
  }
  return `h-${Math.abs(hash).toString(36).slice(0, 6)}`;
}

function extractSummary(lines: string[]) {
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    if (/^#{1,6}\s/.test(trimmed)) continue;
    if (/^(```|~~~)/.test(trimmed)) continue;
    if (/^[-*+]\s/.test(trimmed)) continue;
    if (/^>\s?/.test(trimmed)) continue;
    if (/^\|/.test(trimmed)) continue;
    return trimmed;
  }
  return "";
}

function markdownToPlainText(markdown: string) {
  return markdown
    .replace(/```[\s\S]*?```/g, (block) => block.replace(/```\w*\n?|```/g, " "))
    .replace(/`([^`]+)`/g, "$1")
    .replace(/!\[[^\]]*\]\(([^)]+)\)/g, "")
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, "$1")
    .replace(/^#{1,6}\s+/gm, "")
    .replace(/^>\s?/gm, "")
    .replace(/^[-*+]\s+/gm, "")
    .replace(/^\d+\.\s+/gm, "")
    .replace(/^\|\s*[:-]+[-|\s:]*\|?\s*$/gm, "")
    .replace(/\|/g, " ")
    .replace(/[*_~]/g, "")
    .replace(/\r\n/g, "\n")
    .replace(/\n{2,}/g, "\n")
    .trim();
}

function extractSearchableSections(pageTitle: string, lines: string[]) {
  const sections: DocsSearchSection[] = [];
  let currentTitle = pageTitle;
  let currentLines: string[] = [];
  let currentId = createHeadingId(pageTitle);

  const pushSection = () => {
    const text = markdownToPlainText(currentLines.join("\n"));
    if (!text) return;
    sections.push({ id: currentId, title: currentTitle, text });
  };

  for (const line of lines) {
    const h2 = line.match(/^##\s+(.+)$/);
    if (h2) {
      pushSection();
      currentTitle = h2[1].trim();
      currentId = createHeadingId(currentTitle);
      currentLines = [line];
      continue;
    }
    currentLines.push(line);
  }

  pushSection();
  return sections;
}

export function parseDocsMarkdown(document: string): ParsedDocsPage[] {
  const lines = document.replace(/\r\n/g, "\n").trim().split("\n");
  const pages: ParsedDocsPage[] = [];

  let currentTitle = "";
  let currentLines: string[] = [];

  const pushPage = () => {
    if (!currentTitle) return;
    const markdown = currentLines.join("\n").trim();
    const outline = currentLines
      .map((line) => line.match(/^##\s+(.+)$/)?.[1] ?? null)
      .filter((value): value is string => Boolean(value))
      .map((title) => ({ id: createHeadingId(title), title }));
    const searchableSections = extractSearchableSections(currentTitle, currentLines);

    pages.push({
      path: DOCS_PATH_MAP[currentTitle] ?? `/docs/${createHeadingId(currentTitle)}`,
      title: currentTitle,
      summary: extractSummary(currentLines),
      markdown,
      outline,
      searchableSections,
    });
  };

  for (const line of lines) {
    const heading = line.match(/^#\s+(.+)$/);
    if (heading) {
      pushPage();
      currentTitle = heading[1].trim();
      currentLines = [];
      continue;
    }
    currentLines.push(line);
  }

  pushPage();
  return pages;
}

export function parseDocsPage(document: string): ParsedDocsPage {
  const [page] = parseDocsMarkdown(document);
  if (!page) {
    throw new Error("Markdown document must contain a top-level H1 heading.");
  }
  return page;
}
