export type DocsOutlineItem = {
  id: string;
  title: string;
};

export type ParsedDocsPage = {
  path: string;
  title: string;
  summary: string;
  markdown: string;
  outline: DocsOutlineItem[];
};

const DOCS_PATH_MAP: Record<string, string> = {
  "CialloClaw 是什么？": "/docs/what-is",
  "快速上手": "/docs/quick-start",
  "初认工作台": "/docs/workspace",
  "使用场景": "/docs/scenarios",
  "安全与隐私": "/docs/security",
  "常见问题": "/docs/faq",
};

export function createHeadingId(title: string) {
  return title
    .trim()
    .toLowerCase()
    .replace(/[“”"'`]/g, "")
    .replace(/[^a-z0-9\u4e00-\u9fa5]+/g, "-")
    .replace(/^-+|-+$/g, "");
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

    pages.push({
      path: DOCS_PATH_MAP[currentTitle] ?? `/docs/${createHeadingId(currentTitle)}`,
      title: currentTitle,
      summary: extractSummary(currentLines),
      markdown,
      outline,
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
