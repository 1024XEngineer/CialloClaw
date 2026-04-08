// 桌面端 Tailwind 配置，集中定义扫描范围与主题扩展。
import type { Config } from "tailwindcss";

export default {
  content: ["./*.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        ink: "#09111f",
        accent: "#fb923c",
        aqua: "#22d3ee",
      },
      boxShadow: {
        glow: "0 24px 80px -32px rgba(34, 211, 238, 0.5)",
      },
    },
  },
  plugins: [],
} satisfies Config;
