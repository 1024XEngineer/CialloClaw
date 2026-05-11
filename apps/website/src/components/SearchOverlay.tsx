import { FileText } from "lucide-react";
import { Fragment, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Command,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList,
  CommandShortcut,
} from "@/components/ui/command";
import { docsPages } from "@/content/site";
import { Dialog, DialogContent } from "@/components/ui/dialog";

type SearchOverlayProps = {
  open: boolean;
  onClose: () => void;
};

type SearchResult = {
  key: string;
  pageTitle: string;
  pagePath: string;
  anchorId: string;
  sectionTitle: string;
  snippet: string;
  matchIndex: number;
  score: number;
};

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function createSnippet(text: string, query: string) {
  const normalizedText = text.toLowerCase();
  const normalizedQuery = query.toLowerCase();
  const matchIndex = normalizedText.indexOf(normalizedQuery);
  if (matchIndex === -1) {
    return { snippet: text.slice(0, 80), matchIndex: -1 };
  }

  const snippetStart = Math.max(0, matchIndex - 18);
  const snippetEnd = Math.min(text.length, matchIndex + query.length + 30);
  const prefix = snippetStart > 0 ? "..." : "";
  const suffix = snippetEnd < text.length ? "..." : "";
  return {
    snippet: `${prefix}${text.slice(snippetStart, snippetEnd)}${suffix}`,
    matchIndex,
  };
}

function renderHighlightedSnippet(snippet: string, query: string) {
  if (!query) return snippet;
  const pattern = new RegExp(`(${escapeRegExp(query)})`, "giu");
  return snippet.split(pattern).map((part, index) =>
    pattern.test(part) ? (
      <mark key={`${part}-${index}`} className="rounded bg-[color:rgba(109,184,255,0.24)] px-0.5 text-[color:var(--cc-ink)]">
        {part}
      </mark>
    ) : (
      <Fragment key={`${part}-${index}`}>{part}</Fragment>
    ),
  );
}

export function SearchOverlay({ open, onClose }: SearchOverlayProps) {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");

  const normalizedQuery = query.trim().toLowerCase();
  const results = useMemo<SearchResult[]>(() => {
    if (!normalizedQuery) {
      return [];
    }

    return docsPages
      .flatMap((page) => {
        return page.searchableSections
          .map((section) => {
            const haystack = [page.title, page.summary, section.title, section.text].join("\n");
            const normalizedHaystack = haystack.toLowerCase();
            if (!normalizedHaystack.includes(normalizedQuery)) {
              return null;
            }

            const { snippet, matchIndex } = createSnippet(section.text, normalizedQuery);
            const score =
              (page.title.toLowerCase().includes(normalizedQuery) ? 6 : 0) +
              (section.title.toLowerCase().includes(normalizedQuery) ? 4 : 0) +
              (page.summary.toLowerCase().includes(normalizedQuery) ? 2 : 0) +
              (matchIndex >= 0 ? Math.max(0, 1000 - matchIndex) / 1000 : 0);

            return {
              key: `${page.path}#${section.id}`,
              pageTitle: page.title,
              pagePath: page.path,
              anchorId: section.id,
              sectionTitle: section.title,
              snippet,
              matchIndex,
              score,
            } satisfies SearchResult;
          })
          .filter((entry): entry is SearchResult => entry !== null);
      })
      .sort((left, right) => right.score - left.score || left.pageTitle.localeCompare(right.pageTitle))
      .slice(0, 12);
  }, [normalizedQuery]);

  useEffect(() => {
    if (!open) {
      setQuery("");
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose, open]);

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) {
          onClose();
        }
      }}
    >
      <DialogContent className="top-[12%] max-w-[700px] translate-y-0 p-0 sm:rounded-[24px]">
        <Command className="overflow-hidden rounded-[24px] bg-transparent">
          <CommandInput
            value={query}
            onValueChange={setQuery}
            placeholder="搜索整个文档..."
            className="text-base"
          />

          {normalizedQuery ? (
            <CommandList className="px-2 pb-2 [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
              {results.length > 0 ? (
                results.map((result) => (
                  <CommandItem
                    key={result.key}
                    value={`${result.pageTitle} ${result.sectionTitle} ${result.snippet}`}
                    onSelect={() => {
                      navigate(`${result.pagePath}#${result.anchorId}`);
                      onClose();
                    }}
                    className="mx-1 my-1 rounded-xl border border-transparent px-3 py-3 transition-colors hover:border-[color:var(--cc-line-strong)] hover:bg-[color:var(--cc-surface-strong)] hover:shadow-[0_10px_24px_rgba(20,32,60,0.12)] data-[selected=true]:border-[color:var(--cc-line-strong)] data-[selected=true]:bg-[color:var(--cc-surface-strong)] data-[selected=true]:text-[color:var(--cc-ink)]"
                  >
                    <FileText />
                    <div className="flex min-w-0 flex-1 flex-col">
                      <span className="truncate text-sm font-semibold text-[color:var(--cc-ink)]">{result.pageTitle}</span>
                      <span className="truncate text-xs text-[color:var(--cc-ink-muted)]">{result.sectionTitle}</span>
                      <span className="line-clamp-2 text-xs text-[color:var(--cc-ink)]/78">{renderHighlightedSnippet(result.snippet, query)}</span>
                    </div>
                  </CommandItem>
                ))
              ) : (
                <CommandEmpty>没有找到匹配的文档。</CommandEmpty>
              )}
            </CommandList>
          ) : null}

          <div className="flex items-center justify-center gap-6 border-t px-4 py-3 text-xs text-[color:var(--cc-ink-muted)]" style={{ borderColor: "var(--cc-line)" }}>
            <span>↑ ↓ to navigate</span>
            <CommandShortcut className="m-0 text-[color:var(--cc-ink-muted)]">enter to select</CommandShortcut>
            <span>esc to close</span>
          </div>
        </Command>
      </DialogContent>
    </Dialog>
  );
}
