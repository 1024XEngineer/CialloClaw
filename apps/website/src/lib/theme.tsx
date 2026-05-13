/**
 * Website-level theme context for dark/light mode.
 *
 * Keeps the active theme in a single place outside any component lifecycle so
 * that page transitions never lose the user's preference.
 */
import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

export type Theme = "light" | "dark";

const STORAGE_KEY = "cialloclaw-website-theme";

function readStoredTheme(): Theme {
  if (typeof window === "undefined") {
    return "light";
  }
  const stored = window.localStorage.getItem(STORAGE_KEY);
  return stored === "dark" ? "dark" : "light";
}

function applyThemeToDocument(theme: Theme): void {
  document.documentElement.dataset.theme = theme;
}

interface ThemeContextValue {
  isDark: boolean;
  toggleTheme: () => void;
  theme: Theme;
}

const ThemeContext = createContext<ThemeContextValue>({
  isDark: false,
  toggleTheme: () => {},
  theme: "light",
});

export function useWebsiteTheme(): ThemeContextValue {
  return useContext(ThemeContext);
}

/**
 * Wraps children so the active theme is synchronised across route changes and
 * persists to localStorage on every toggle.
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => {
    const t = readStoredTheme();
    // Apply before the first paint so there is never a flash of the wrong theme.
    if (typeof document !== "undefined") {
      applyThemeToDocument(t);
    }
    return t;
  });

  const toggleTheme = () => {
    setTheme((prev) => {
      const next: Theme = prev === "dark" ? "light" : "dark";
      applyThemeToDocument(next);
      window.localStorage.setItem(STORAGE_KEY, next);
      return next;
    });
  };

  // Sync when the theme value changes (covers the initial mount path where
  // useState may have already applied the value synchronously).
  useEffect(() => {
    applyThemeToDocument(theme);
  }, [theme]);

  return (
    <ThemeContext.Provider value={{ isDark: theme === "dark", toggleTheme, theme }}>
      {children}
    </ThemeContext.Provider>
  );
}
