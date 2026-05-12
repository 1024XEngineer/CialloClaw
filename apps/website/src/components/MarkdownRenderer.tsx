import type { ReactNode } from "react";
import ReactMarkdown from "react-markdown";
import remarkBreaks from "remark-breaks";
import remarkGfm from "remark-gfm";
import { createHeadingId } from "@/lib/docs";

type MarkdownRendererProps = {
  content: string;
};

function preserveExtraBlankLines(content: string) {
  return content.replace(/\n{3,}/g, (match) => {
    const extraBlankParagraphs = Math.max(match.length - 2, 1);
    const fillers = Array.from({ length: extraBlankParagraphs }, () => "\u00A0").join("\n\n");
    return `\n\n${fillers}\n\n`;
  });
}

function toPlainText(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") {
    return String(node);
  }

  if (Array.isArray(node)) {
    return node.map((child) => toPlainText(child)).join("");
  }

  if (node && typeof node === "object" && "props" in node) {
    return toPlainText(node.props.children as ReactNode);
  }

  return "";
}

export function MarkdownRenderer({ content }: MarkdownRendererProps) {
  const normalizedContent = preserveExtraBlankLines(content);

  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm, remarkBreaks]}
      components={{
        h1: ({ children }) => <h1 id={createHeadingId(toPlainText(children))}>{children}</h1>,
        h2: ({ children }) => <h2 id={createHeadingId(toPlainText(children))}>{children}</h2>,
        h3: ({ children }) => <h3 id={createHeadingId(toPlainText(children))}>{children}</h3>,
        a: ({ href, children }) => (
          <a href={href} target={href?.startsWith("http") ? "_blank" : undefined} rel={href?.startsWith("http") ? "noreferrer" : undefined}>
            {children}
          </a>
        ),
        table: ({ children }) => (
          <div className="docs-page__table-shell">
            <table>{children}</table>
          </div>
        ),
      }}
    >
      {normalizedContent}
    </ReactMarkdown>
  );
}
