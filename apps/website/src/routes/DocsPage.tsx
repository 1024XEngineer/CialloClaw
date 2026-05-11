import { Menu as MenuIcon } from "lucide-react";
import type { AnchorProps, MenuProps } from "antd";
import { Anchor, Drawer, Menu } from "antd";
import "antd/dist/reset.css";
import { useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import { MarkdownRenderer } from "@/components/MarkdownRenderer";
import { getDocsPage, getDocsSidebar } from "@/content/site";
import "@/styles/docs-page.css";

export function DocsPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const { t, locale } = useI18n();
  const entry = getDocsPage(location.pathname, locale);
  const [menuOpen, setMenuOpen] = useState(false);
  const docsScrollRef = useRef<HTMLElement | null>(null);

  const sidebar = useMemo(() => getDocsSidebar(locale), [locale]);

  const menuItems = useMemo<MenuProps["items"]>(() => {
    return sidebar.map((section) => ({
      key: section.title,
      label: section.title,
      type: "group",
      children: section.items.map((item) => ({
        key: item.href,
        label: item.title,
      })),
    }));
  }, [sidebar]);

  const anchorItems = useMemo<NonNullable<AnchorProps["items"]>>(() => {
    return entry.outline.map((section) => ({
      key: section.id,
      href: `#${section.id}`,
      title: section.title,
    }));
  }, [entry.outline]);

  const scrollToHeading = (href: string) => {
    const rawId = href.replace(/^#/, "");
    const id = decodeURIComponent(rawId);
    const target = docsScrollRef.current?.querySelector<HTMLElement>(`#${CSS.escape(id)}`);
    if (!target) return;
    target.scrollIntoView({ behavior: "smooth", block: "start" });
    window.history.replaceState(null, "", `${location.pathname}#${encodeURIComponent(id)}`);
  };

  useEffect(() => {
    if (!location.hash) return;
    const frame = window.requestAnimationFrame(() => {
      scrollToHeading(location.hash);
    });
    return () => window.cancelAnimationFrame(frame);
  }, [location.hash, location.pathname]);

  return (
    <>
      <section ref={docsScrollRef} className="docs-page grid h-full gap-10 lg:grid-cols-[260px_minmax(0,820px)_220px] lg:gap-12 lg:justify-center">
        <aside className="docs-page__sidebar hidden lg:block">
          <div className="docs-page__sticky docs-page__panel docs-page__menu-shell">
            <p className="docs-page__sidebar-title">{t("docs.page-title")}</p>
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
              <span>{t("docs.menu-button")}</span>
            </button>
            <p className="docs-page__eyebrow" lang="en">Docs</p>
            <h1 className="docs-page__title">{entry.title}</h1>
          </header>

          <article className="docs-page__article">
            <MarkdownRenderer content={entry.markdown} />
          </article>
        </main>

        {anchorItems.length > 0 ? (
          <aside className="docs-page__outline hidden lg:block">
            <div className="docs-page__sticky docs-page__panel docs-page__anchor-shell">
              <p className="docs-page__sidebar-title docs-page__outline-title">{t("docs.outline-title")}</p>
              <Anchor
                items={anchorItems}
                affix={false}
                targetOffset={104}
                getContainer={() => docsScrollRef.current ?? window}
                onClick={(event, link) => {
                  event.preventDefault();
                  if (link.href) {
                    scrollToHeading(link.href);
                  }
                }}
                className="docs-page__anchor"
              />
            </div>
          </aside>
        ) : null}
      </section>

      <Drawer
        title={t("docs.drawer-title")}
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
    </>
  );
}
