/**
 * AppProviders wires cross-window desktop concerns that are shared by every
 * frontend entrypoint.
 */
import type { PropsWithChildren } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { TooltipProvider } from "@/components/ui/tooltip";
import { queryClient } from "@/queries/queryClient";
import "@/platform/namedPipeBridge";
import "@/styles/globals.css";

/**
 * Provides shared query, tooltip, and near-field selection infrastructure for
 * desktop entry windows.
 *
 * @param props Child route tree to render.
 * @returns The wrapped application tree.
 */
export function AppProviders({ children }: PropsWithChildren) {
  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>{children}</TooltipProvider>
    </QueryClientProvider>
  );
}
