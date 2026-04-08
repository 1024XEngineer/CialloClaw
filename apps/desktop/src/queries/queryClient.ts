// 该文件负责前端查询缓存客户端的初始化。
import { QueryClient } from "@tanstack/react-query";

// queryClient 表示当前模块的客户端实例。
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5_000,
      retry: 1,
    },
  },
});
