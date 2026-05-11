import { Navigate, Route, Routes } from "react-router-dom";
import { WebsiteLayout } from "@/components/WebsiteLayout";
import { AboutPage } from "@/routes/AboutPage";
import { BlogPage } from "@/routes/BlogPage";
import { CharacterPage } from "@/routes/CharacterPage";
import { DocsPage } from "@/routes/DocsPage";
import { HomePage } from "@/routes/HomePage";

export function AppRouter() {
  return (
    <Routes>
      <Route element={<WebsiteLayout />}>
        <Route path="/" element={<HomePage />} />
        <Route path="/blog" element={<BlogPage />} />
        <Route path="/about" element={<AboutPage />} />
        <Route path="/characters" element={<CharacterPage />} />
        <Route path="/docs" element={<Navigate replace to="/docs/what-is" />} />
        <Route path="/docs/*" element={<DocsPage />} />
      </Route>
    </Routes>
  );
}
