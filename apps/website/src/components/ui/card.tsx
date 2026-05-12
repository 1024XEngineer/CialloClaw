import type { HTMLAttributes } from "react";
import { cn } from "@/lib/utils";

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "rounded-[2rem] border border-[color:var(--cc-line)] bg-[color:var(--cc-paper)] p-6 shadow-card backdrop-blur",
        className,
      )}
      {...props}
    />
  );
}
