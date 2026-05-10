import { Navigate, Route, Routes } from "react-router-dom";
import { AboutPage } from "@/routes/AboutPage";
import { BlogPage } from "@/routes/BlogPage";
import { CharacterPage } from "@/routes/CharacterPage";
import { DocsPage } from "@/routes/DocsPage";
import { HomePage } from "@/routes/HomePage";

export function AppRouter() {
  return (
    <Routes>
      <Route path="/" element={<HomePage />} />
      <Route path="/blog" element={<BlogPage />} />
      <Route path="/about" element={<AboutPage />} />
      <Route path="/characters" element={<CharacterPage />} />
      <Route path="/docs" element={<Navigate replace to="/docs/overview" />} />
      <Route path="/docs/*" element={<DocsPage />} />
    </Routes>
  );
}
