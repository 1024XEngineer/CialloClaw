import type { HTMLAttributes } from "react";
import { cn } from "@/lib/utils";

export function Badge({ className, ...props }: HTMLAttributes<HTMLSpanElement>) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border border-[color:var(--cc-line)] bg-white/72 px-3 py-1 text-xs font-semibold tracking-[0.12em] text-[color:var(--cc-ink-soft)] uppercase",
        className,
      )}
      {...props}
    />
  );
}
