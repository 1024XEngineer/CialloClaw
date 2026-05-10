import type { PropsWithChildren } from "react";
import { useLocation } from "react-router-dom";
import { PillNav } from "@/components/PillNav";
import { siteNav } from "@/content/site";

type WebsiteLayoutProps = PropsWithChildren<{
  fullViewport?: boolean;
  mainClassName?: string;
}>;

export function WebsiteLayout({ children, fullViewport = false, mainClassName }: WebsiteLayoutProps) {
  const location = useLocation();
  const isHome = location.pathname === "/";
  const activeHref =
    location.pathname.startsWith("/docs")
      ? "/docs/overview"
      : location.pathname.startsWith("/blog")
        ? "/blog"
        : location.pathname.startsWith("/about")
          ? "/about"
          : "/";

  return (
    <div className="relative h-screen overflow-hidden">
      <PillNav logoAlt="Desktop Agent" items={siteNav} activeHref={activeHref} isHome={isHome} />
      <main
        className={
          mainClassName ??
          (fullViewport
            ? "relative z-10 mx-auto h-full overflow-hidden px-5 pt-[72px] sm:px-8 lg:px-12"
            : "relative z-10 mx-auto h-full max-w-6xl overflow-hidden px-5 pb-0 pt-24 sm:px-8 lg:px-12")
        }
      >
        {children}
      </main>
    </div>
  );
}
