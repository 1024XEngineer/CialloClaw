import { Menu as MenuIcon } from "lucide-react";
import type { AnchorProps, MenuProps } from "antd";
import { Anchor, Drawer, Menu } from "antd";
import "antd/dist/reset.css";
import { useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { WebsiteLayout } from "@/components/WebsiteLayout";
import { docsSidebar, getDocsPage } from "@/content/site";
import "@/styles/docs-page.css";

export function DocsPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const entry = getDocsPage(location.pathname);
  const [menuOpen, setMenuOpen] = useState(false);

  const menuItems = useMemo<MenuProps["items"]>(() => {
    return docsSidebar.map((section) => ({
      key: section.title,
      label: section.title,
      type: "group",
      children: section.items.map((item) => ({
        key: item.href,
        label: item.title,
      })),
    }));
  }, []);

  const anchorItems = useMemo<AnchorProps["items"]>(() => {
    return entry.sections.map((section) => ({
      key: section.id,
      href: `#${section.id}`,
      title: section.title,
    }));
  }, [entry.sections]);

  return (
    <WebsiteLayout
      mainClassName="relative z-10 mx-auto h-full max-w-[1600px] overflow-hidden px-5 pb-0 pt-24 sm:px-8 lg:px-12"
    >
      <section className="docs-page grid h-full gap-8 lg:grid-cols-[260px_minmax(0,820px)_220px] lg:justify-center">
        <aside className="docs-page__sidebar hidden lg:block">
          <div className="docs-page__sticky docs-page__panel docs-page__menu-shell">
            <p className="docs-page__sidebar-title">文档导航</p>
            <Menu
              mode="inline"
              selectedKeys={[entry.path]}
              items={menuItems}
              onClick={({ key }) => navigate(String(key))}
              className="docs-page__menu"
            />
          </div>
        </aside>

        <main className="docs-page__content min-w-0">
          <header className="docs-page__hero">
            <button type="button" className="docs-page__menu-button lg:hidden" onClick={() => setMenuOpen(true)}>
              <MenuIcon className="h-4 w-4" />
              <span>文档导航</span>
            </button>
            <p className="docs-page__eyebrow">Docs</p>
            <h1 className="docs-page__title">{entry.title}</h1>
            <p className="docs-page__summary">{entry.summary}</p>
          </header>

          <article className="docs-page__article">
            {entry.sections.map((section) => (
              <section key={section.id} id={section.id} className="docs-page__section scroll-mt-28">
                <h2>{section.title}</h2>
                {section.body.map((paragraph) => (
                  <p key={paragraph}>{paragraph}</p>
                ))}
              </section>
            ))}
          </article>
        </main>

        <aside className="docs-page__outline hidden lg:block">
          <div className="docs-page__sticky docs-page__panel docs-page__anchor-shell">
            <p className="docs-page__sidebar-title docs-page__outline-title">本页大纲</p>
            <Anchor
              items={anchorItems}
              affix={false}
              targetOffset={104}
              className="docs-page__anchor"
            />
          </div>
        </aside>
      </section>

      <Drawer
        title="文档导航"
        placement="left"
        width={300}
        open={menuOpen}
        onClose={() => setMenuOpen(false)}
        className="docs-page__drawer lg:hidden"
        styles={{
          body: { padding: 12, background: "var(--cc-bg)" },
          header: { background: "var(--cc-bg)", borderBottom: `1px solid var(--cc-line)` },
          content: { background: "var(--cc-bg)" },
        }}
      >
        <Menu
          mode="inline"
          selectedKeys={[entry.path]}
          items={menuItems}
          onClick={({ key }) => {
            navigate(String(key));
            setMenuOpen(false);
          }}
          className="docs-page__menu"
        />
      </Drawer>
    </WebsiteLayout>
  );
}
