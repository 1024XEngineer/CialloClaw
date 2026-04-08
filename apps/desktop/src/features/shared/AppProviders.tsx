// 该文件承载跨入口共享相关的界面逻辑。
import type { PropsWithChildren } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { queryClient } from "@/queries/queryClient";
import "@/styles/globals.css";

// AppProviders 处理当前模块的相关逻辑。
export function AppProviders({ children }: PropsWithChildren) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}
