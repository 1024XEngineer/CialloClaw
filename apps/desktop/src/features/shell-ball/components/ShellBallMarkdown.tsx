import { Fragment, type ReactNode } from "react";

type ShellBallMarkdownProps = {
  text: string;
};

type MarkdownBlock =
  | { kind: "heading"; level: 1 | 2 | 3 | 4 | 5 | 6; text: string }
  | { kind: "paragraph"; lines: string[] }
  | { kind: "unordered-list"; items: string[] }
  | { kind: "ordered-list"; items: string[] }
  | { kind: "blockquote"; lines: string[] }
  | { kind: "code"; language: string; content: string };

type InlineMatch =
  | { kind: "link"; index: number; length: number; label: string; href: string }
  | { kind: "code"; index: number; length: number; text: string }
  | { kind: "strong"; index: number; length: number; text: string }
  | { kind: "emphasis"; index: number; length: number; text: string };

export function ShellBallMarkdown({ text }: ShellBallMarkdownProps) {
  const blocks = parseMarkdownBlocks(text);

  if (blocks.length === 0) {
    return <p className="shell-ball-bubble-message__text">{text}</p>;
  }

  return (
    <div className="shell-ball-bubble-message__markdown">
      {blocks.map((block) => renderMarkdownBlock(block, createBlockKey(block)))}
    </div>
  );
}

function renderMarkdownBlock(block: MarkdownBlock, key: string) {
  switch (block.kind) {
    case "heading": {
      const HeadingTag = `h${block.level}` as const;
      return <HeadingTag key={key}>{renderInline(block.text, key)}</HeadingTag>;
    }
    case "paragraph":
      return <p key={key}>{renderMultilineInline(block.lines, key)}</p>;
    case "unordered-list":
      return (
        <ul key={key}>
          {block.items.map((item) => {
            const itemKey = `${key}-item-${createStableKeyFragment(item)}`;
            return <li key={itemKey}>{renderInline(item, itemKey)}</li>;
          })}
        </ul>
      );
    case "ordered-list":
      return (
        <ol key={key}>
          {block.items.map((item) => {
            const itemKey = `${key}-item-${createStableKeyFragment(item)}`;
            return <li key={itemKey}>{renderInline(item, itemKey)}</li>;
          })}
        </ol>
      );
    case "blockquote":
      return <blockquote key={key}>{renderMultilineInline(block.lines, key)}</blockquote>;
    case "code":
      return (
        <pre key={key}>
          <code data-language={block.language || undefined}>{block.content}</code>
        </pre>
      );
  }
}

function renderMultilineInline(lines: string[], keyPrefix: string) {
  return lines.map((line, index) => {
    const lineKey = `${keyPrefix}-line-${createStableKeyFragment(lines.slice(0, index + 1).join("\n"))}`;
    return (
      <Fragment key={lineKey}>
        {index > 0 ? <br /> : null}
        {renderInline(line, lineKey)}
      </Fragment>
    );
  });
}

function renderInline(text: string, keyPrefix: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  let remainder = text;
  let tokenIndex = 0;

  while (remainder.length > 0) {
    const match = findFirstInlineMatch(remainder);
    if (match === null) {
      nodes.push(remainder);
      break;
    }

    if (match.index > 0) {
      nodes.push(remainder.slice(0, match.index));
    }

    const matchKey = `${keyPrefix}-token-${tokenIndex}`;
    switch (match.kind) {
      case "link":
        nodes.push(
          <a key={matchKey} href={match.href} target="_blank" rel="noreferrer">
            {renderInline(match.label, `${matchKey}-label`)}
          </a>,
        );
        break;
      case "code":
        nodes.push(<code key={matchKey}>{match.text}</code>);
        break;
      case "strong":
        nodes.push(<strong key={matchKey}>{renderInline(match.text, `${matchKey}-strong`)}</strong>);
        break;
      case "emphasis":
        nodes.push(<em key={matchKey}>{renderInline(match.text, `${matchKey}-emphasis`)}</em>);
        break;
    }

    remainder = remainder.slice(match.index + match.length);
    tokenIndex += 1;
  }

  return nodes;
}

function findFirstInlineMatch(text: string): InlineMatch | null {
  const matches = [matchLink(text), matchInlineCode(text), matchStrong(text), matchEmphasis(text)].filter(
    (match): match is InlineMatch => match !== null,
  );

  if (matches.length === 0) {
    return null;
  }

  matches.sort((left, right) => {
    if (left.index !== right.index) {
      return left.index - right.index;
    }

    return right.length - left.length;
  });

  return matches[0] ?? null;
}

function matchLink(text: string): InlineMatch | null {
  const match = /\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/.exec(text);
  if (match === null || match.index === undefined) {
    return null;
  }

  return {
    kind: "link",
    index: match.index,
    length: match[0].length,
    label: match[1],
    href: match[2],
  };
}

function matchInlineCode(text: string): InlineMatch | null {
  const match = /`([^`]+)`/.exec(text);
  if (match === null || match.index === undefined) {
    return null;
  }

  return {
    kind: "code",
    index: match.index,
    length: match[0].length,
    text: match[1],
  };
}

function matchStrong(text: string): InlineMatch | null {
  const match = /\*\*([^*]+)\*\*/.exec(text);
  if (match === null || match.index === undefined) {
    return null;
  }

  return {
    kind: "strong",
    index: match.index,
    length: match[0].length,
    text: match[1],
  };
}

function matchEmphasis(text: string): InlineMatch | null {
  const match = /(^|[^*])\*([^*]+)\*(?!\*)/.exec(text);
  if (match === null || match.index === undefined) {
    return null;
  }

  const prefixLength = match[1].length;
  return {
    kind: "emphasis",
    index: match.index + prefixLength,
    length: match[0].length - prefixLength,
    text: match[2],
  };
}

function parseMarkdownBlocks(text: string): MarkdownBlock[] {
  const normalized = text.replace(/\r\n?/g, "\n").trim();
  if (normalized === "") {
    return [];
  }

  const lines = normalized.split("\n");
  const blocks: MarkdownBlock[] = [];
  let index = 0;

  while (index < lines.length) {
    const line = lines[index]?.trimEnd() ?? "";

    if (line.trim() === "") {
      index += 1;
      continue;
    }

    const headingMatch = /^(#{1,6})\s+(.+)$/.exec(line);
    if (headingMatch !== null) {
      blocks.push({
        kind: "heading",
        level: headingMatch[1].length as 1 | 2 | 3 | 4 | 5 | 6,
        text: headingMatch[2].trim(),
      });
      index += 1;
      continue;
    }

    if (/^```/.test(line)) {
      const language = line.slice(3).trim();
      index += 1;
      const content: string[] = [];
      while (index < lines.length && !/^```/.test(lines[index] ?? "")) {
        content.push(lines[index] ?? "");
        index += 1;
      }
      if (index < lines.length) {
        index += 1;
      }
      blocks.push({
        kind: "code",
        language,
        content: content.join("\n"),
      });
      continue;
    }

    if (/^>\s?/.test(line)) {
      const quoteLines: string[] = [];
      while (index < lines.length) {
        const currentLine = lines[index] ?? "";
        if (!/^>\s?/.test(currentLine)) {
          break;
        }
        quoteLines.push(currentLine.replace(/^>\s?/, ""));
        index += 1;
      }
      blocks.push({ kind: "blockquote", lines: quoteLines });
      continue;
    }

    if (/^[-*]\s+/.test(line)) {
      const items: string[] = [];
      while (index < lines.length) {
        const currentLine = lines[index] ?? "";
        if (!/^[-*]\s+/.test(currentLine)) {
          break;
        }
        items.push(currentLine.replace(/^[-*]\s+/, "").trim());
        index += 1;
      }
      blocks.push({ kind: "unordered-list", items });
      continue;
    }

    if (/^\d+\.\s+/.test(line)) {
      const items: string[] = [];
      while (index < lines.length) {
        const currentLine = lines[index] ?? "";
        if (!/^\d+\.\s+/.test(currentLine)) {
          break;
        }
        items.push(currentLine.replace(/^\d+\.\s+/, "").trim());
        index += 1;
      }
      blocks.push({ kind: "ordered-list", items });
      continue;
    }

    const paragraphLines: string[] = [];
    while (index < lines.length) {
      const currentLine = lines[index] ?? "";
      if (currentLine.trim() === "" || /^(#{1,6})\s+/.test(currentLine) || /^```/.test(currentLine) || /^>\s?/.test(currentLine) || /^[-*]\s+/.test(currentLine) || /^\d+\.\s+/.test(currentLine)) {
        break;
      }
      paragraphLines.push(currentLine);
      index += 1;
    }
    blocks.push({ kind: "paragraph", lines: paragraphLines });
  }

  return blocks;
}

function createBlockKey(block: MarkdownBlock) {
  switch (block.kind) {
    case "heading":
      return `heading-${block.level}-${createStableKeyFragment(block.text)}`;
    case "paragraph":
      return `paragraph-${createStableKeyFragment(block.lines.join("\n"))}`;
    case "unordered-list":
      return `unordered-${createStableKeyFragment(block.items.join("\n"))}`;
    case "ordered-list":
      return `ordered-${createStableKeyFragment(block.items.join("\n"))}`;
    case "blockquote":
      return `blockquote-${createStableKeyFragment(block.lines.join("\n"))}`;
    case "code":
      return `code-${createStableKeyFragment(`${block.language}:${block.content}`)}`;
  }
}

function createStableKeyFragment(value: string) {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 48) || "empty";
}
