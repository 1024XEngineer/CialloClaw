// 桌面端 Vite 配置，负责多入口构建与路径别名。
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const currentDirectory = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/rpc": {
        changeOrigin: true,
        target: "http://127.0.0.1:4317",
      },
    },
  },
  resolve: {
    alias: {
      "@": resolve(currentDirectory, "src"),
    },
  },
  build: {
    rollupOptions: {
      input: {
        "shell-ball": resolve(currentDirectory, "shell-ball.html"),
        "shell-ball-bubble-pinned": resolve(currentDirectory, "shell-ball-bubble-pinned.html"),
        dashboard: resolve(currentDirectory, "dashboard.html"),
        "control-panel": resolve(currentDirectory, "control-panel.html"),
      },
    },
  },
});
