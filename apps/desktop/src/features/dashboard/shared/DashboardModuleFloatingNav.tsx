import { NavLink } from "react-router-dom";
import { cn } from "@/utils/cn";
import { dashboardModules } from "./dashboardRoutes";

/**
 * Renders the fixed module navigation used by dashboard detail surfaces that do
 * not own their own top bar, such as memory and safety workspaces.
 */
export function DashboardModuleFloatingNav() {
  return (
    <nav aria-label="Dashboard modules" className="dashboard-page__module-nav dashboard-page__module-nav--floating">
      {dashboardModules.map((item) => (
        <NavLink key={item.route} className={({ isActive }) => cn("dashboard-page__module-link", isActive && "is-active")} to={item.path}>
          {item.title}
        </NavLink>
      ))}
    </nav>
  );
}
