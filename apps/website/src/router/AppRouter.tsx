import { ConfigProvider, theme } from "antd";
import { Navigate, Route, Routes } from "react-router-dom";
import { WebsiteLayout } from "@/components/WebsiteLayout";
import { useWebsiteTheme } from "@/lib/theme.tsx";
import { CharacterPage } from "@/routes/CharacterPage";
import { DocsPage } from "@/routes/DocsPage";
import { HomePage } from "@/routes/HomePage";

/** Wraps the docs sub-tree so antd components respect the active dark mode. */
function DocsPageWrapper() {
  const { isDark } = useWebsiteTheme();

  return (
    <ConfigProvider
      theme={{
        algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
      }}
    >
      <DocsPage />
    </ConfigProvider>
  );
}

export function AppRouter() {
  return (
    <Routes>
      <Route element={<WebsiteLayout />}>
        <Route path="/" element={<HomePage />} />
        <Route path="/characters" element={<CharacterPage />} />
        <Route path="/docs" element={<Navigate replace to="/docs/what-is" />} />
        <Route path="/docs/*" element={<DocsPageWrapper />} />
      </Route>
    </Routes>
  );
}
