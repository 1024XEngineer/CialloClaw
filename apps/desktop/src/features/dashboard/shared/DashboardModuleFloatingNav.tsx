import type { CSSProperties } from "react";
import { Link, NavLink } from "react-router-dom";
import { cn } from "@/utils/cn";
import { resolveDashboardRoutePath } from "./dashboardRouteTargets";
import { dashboardModules } from "./dashboardRoutes";

type DashboardModuleFloatingNavProps = {
  accentColor?: string;
  className?: string;
  includeHomeLink?: boolean;
};

/**
 * Renders the shared dashboard module navigation bar.
 */
export function DashboardModuleFloatingNav({ accentColor, className, includeHomeLink = false }: DashboardModuleFloatingNavProps) {
  const style = accentColor
    ? ({ "--dashboard-nav-accent": accentColor } as CSSProperties)
    : undefined;

  return (
    <header className={cn("dashboard-page__topbar dashboard-page__topbar--shared", className)} style={style}>
      {includeHomeLink ? (
        <Link className="dashboard-page__home-link" to={resolveDashboardRoutePath("home")}>
          返回首页
        </Link>
      ) : <span />}

      <nav aria-label="Dashboard modules" className="dashboard-page__module-nav">
        {dashboardModules.map((item) => (
          <NavLink key={item.route} className={({ isActive }) => cn("dashboard-page__module-link", isActive && "is-active")} to={item.path}>
            {item.title}
          </NavLink>
        ))}
      </nav>
    </header>
  );
}
