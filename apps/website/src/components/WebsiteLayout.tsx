import type { PropsWithChildren } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { PillNav } from "@/components/PillNav";
import { siteNav } from "@/content/site";

type WebsiteLayoutProps = PropsWithChildren<{
  fullViewport?: boolean;
  mainClassName?: string;
}>;

export function WebsiteLayout({ children, fullViewport = false, mainClassName }: WebsiteLayoutProps) {
  const location = useLocation();
  const isHome = location.pathname === "/";
  const isDocs = location.pathname.startsWith("/docs");
  const activeHref =
    location.pathname.startsWith("/docs")
      ? "/docs/overview"
      : location.pathname.startsWith("/blog")
        ? "/blog"
        : location.pathname.startsWith("/about")
          ? "/about"
          : "/";
  const resolvedMainClassName =
    mainClassName ??
    (isHome || fullViewport
      ? "relative z-10 h-full overflow-hidden pt-[72px]"
      : isDocs
        ? "relative z-10 mx-auto h-full max-w-[1600px] overflow-hidden px-5 pb-0 pt-24 sm:px-8 lg:px-12"
        : "relative z-10 mx-auto h-full max-w-6xl overflow-hidden px-5 pb-0 pt-24 sm:px-8 lg:px-12");

  return (
    <div className="relative h-screen w-screen overflow-hidden">
      <PillNav logoAlt="Desktop Agent" items={siteNav} activeHref={activeHref} isHome={isHome} />
      <main className={resolvedMainClassName}>{children ?? <Outlet />}</main>
    </div>
  );
}
