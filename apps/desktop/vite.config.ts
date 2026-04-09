// 桌面端 Vite 配置，负责多入口构建与路径别名。
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const currentDirectory = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": resolve(currentDirectory, "src"),
    },
  },
  build: {
    rollupOptions: {
      input: {
        "shell-ball": resolve(currentDirectory, "shell-ball.html"),
      dashboard: resolve(currentDirectory, "dashboard.html"),
      "control-panel": resolve(currentDirectory, "control-panel.html"),
      security: resolve(currentDirectory, "security.html"),
      mirror: resolve(currentDirectory, "mirror.html"),
      },
    },
  },
});
