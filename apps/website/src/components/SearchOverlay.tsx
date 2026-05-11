import { FileText } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
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

export function SearchOverlay({ open, onClose }: SearchOverlayProps) {
  const navigate = useNavigate();
  const [query, setQuery] = useState("");

  const normalizedQuery = query.trim().toLowerCase();
  const results = useMemo(() => {
    if (!normalizedQuery) {
      return [];
    }

    return docsPages
      .map((page) => {
        const haystack = [page.title, page.summary, ...page.outline.map((section) => section.title), page.markdown]
          .join("\n")
          .toLowerCase();

        const score =
          (page.title.toLowerCase().includes(normalizedQuery) ? 4 : 0) +
          (page.summary.toLowerCase().includes(normalizedQuery) ? 2 : 0) +
          (haystack.includes(normalizedQuery) ? 1 : 0);

        return {
          page,
          score,
        };
      })
      .filter((entry) => entry.score > 0)
      .sort((left, right) => right.score - left.score || left.page.title.localeCompare(right.page.title))
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
            <CommandList className="overflow-hidden px-2 pb-2">
              {results.length > 0 ? (
                results.map(({ page }) => (
                  <CommandItem
                    key={page.path}
                    value={`${page.title} ${page.summary}`}
                    onSelect={() => {
                      navigate(page.path);
                      onClose();
                    }}
                    className="mx-1 my-1 rounded-xl px-3 py-3 data-[selected=true]:bg-[color:var(--cc-surface)] data-[selected=true]:text-[color:var(--cc-ink)]"
                  >
                    <FileText />
                    <div className="flex min-w-0 flex-1 flex-col">
                      <span className="truncate text-sm font-semibold text-[color:var(--cc-ink)]">{page.title}</span>
                      <span className="truncate text-xs text-[color:var(--cc-ink-soft)]">{page.summary}</span>
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
