// 桌面端 Tailwind 配置，集中定义扫描范围与主题扩展。
import type { Config } from "tailwindcss";

export default {
  content: ["./*.html", "./src/**/*.{ts,tsx}", "../../packages/ui/src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ink: "#09111f",
        accent: "#fb923c",
        aqua: "#22d3ee",
        status: {
          "confirming-intent": "#38bdf8",
          "confirming-intent-foreground": "#e0f2fe",
          processing: "#22d3ee",
          "processing-foreground": "#cffafe",
          "waiting-auth": "#fbbf24",
          "waiting-auth-foreground": "#fef3c7",
          "waiting-input": "#a78bfa",
          "waiting-input-foreground": "#ede9fe",
          paused: "#64748b",
          "paused-foreground": "#f1f5f9",
          blocked: "#fb923c",
          "blocked-foreground": "#ffedd5",
          completed: "#34d399",
          "completed-foreground": "#d1fae5",
          failed: "#fb7185",
          "failed-foreground": "#ffe4e6",
          cancelled: "#334155",
          "cancelled-foreground": "#e2e8f0",
          "ended-unfinished": "#71717a",
          "ended-unfinished-foreground": "#f4f4f5",
        },
      },
      boxShadow: {
        glow: "0 24px 80px -32px rgba(34, 211, 238, 0.5)",
      },
    },
  },
  plugins: [],
} satisfies Config;
