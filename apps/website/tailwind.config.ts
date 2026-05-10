import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      boxShadow: {
        glow: "0 28px 90px -40px rgba(91, 66, 42, 0.28)",
        card: "var(--cc-card-shadow)",
      },
      borderRadius: {
        hero: "2.25rem",
      },
      fontFamily: {
        display: ["var(--cc-font-display)"],
        sans: ["var(--cc-font-ui)"],
      },
    },
  },
  plugins: [],
} satisfies Config;
