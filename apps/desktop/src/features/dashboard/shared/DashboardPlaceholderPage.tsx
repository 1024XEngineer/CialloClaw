import { ArrowLeft, CircleDashed } from "lucide-react";
import { Link, NavLink } from "react-router-dom";
import { cn } from "@/utils/cn";
import { resolveDashboardRoutePath } from "./dashboardRouteTargets";
import { dashboardModuleMap, dashboardModules, type DashboardModuleRoute } from "./dashboardRoutes";

type DashboardPlaceholderPageProps = {
  route: DashboardModuleRoute;
};

export function DashboardPlaceholderPage({ route }: DashboardPlaceholderPageProps) {
  const module = dashboardModuleMap[route];
  const Icon = module.icon;

  return (
    <main className="dashboard-page">
      <header className="dashboard-page__topbar">
        <Link className="dashboard-page__home-link" to={resolveDashboardRoutePath("home")}>
          <ArrowLeft className="h-4 w-4" />
          返回首页
        </Link>

        <nav className="dashboard-page__module-nav" aria-label="Dashboard modules">
          {dashboardModules.map((item) => (
            <NavLink
              key={item.route}
              className={({ isActive }) => cn("dashboard-page__module-link", isActive && "is-active")}
              to={item.path}
            >
              {item.title}
            </NavLink>
          ))}
        </nav>
      </header>

      <section className="dashboard-page__hero">
        <div className="dashboard-page__hero-copy">
          <p className="dashboard-page__eyebrow">{module.englishTitle}</p>
          <div className="dashboard-page__title-row">
            <Icon className="dashboard-page__title-icon" />
            <h1>{module.title}</h1>
          </div>
          <p className="dashboard-page__description">{module.description}</p>
        </div>

        <div className="dashboard-card dashboard-card--status">
          <p className="dashboard-card__kicker">当前状态</p>
          <div className="dashboard-card__status-row">
            <CircleDashed className="h-4 w-4" />
            <span>占位页已就绪，可继续在该路由下扩展子页面。</span>
          </div>
        </div>
      </section>

      <section className="dashboard-page__grid">
        <article className="dashboard-card">
          <p className="dashboard-card__kicker">下一层页面建议</p>
          <ul className="dashboard-card__list">
            {module.futurePages.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </article>

        <article className="dashboard-card">
          <p className="dashboard-card__kicker">路由策略</p>
          <p>
            当前模块已经接入路由。后续新增子页面时，可以直接在 `{module.path}` 下继续添加二级页面。
          </p>
        </article>

        <article className="dashboard-card">
          <p className="dashboard-card__kicker">设计约束</p>
          <p>保持浅色纸感背景、低噪声动效和自然留白，不把深色 glass 风格直接搬到这里。</p>
        </article>
      </section>
    </main>
  );
}
