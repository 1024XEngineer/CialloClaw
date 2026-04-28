import { ArrowUpRight, FileSearch, PanelRightOpen } from "lucide-react";
import { useMemo } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { DashboardBackHomeLink } from "@/features/dashboard/shared/DashboardBackHomeLink";
import { navigateToDashboardTaskDetail } from "@/features/dashboard/shared/dashboardTaskDetailNavigation";
import { readDashboardResultPageLocation } from "@/features/dashboard/shared/dashboardResultPageNavigation";

function isLoopbackHost(hostname: string) {
  return hostname === "localhost" || hostname === "127.0.0.1" || hostname === "::1";
}

function isAllowedDashboardResultPageUrl(url: string) {
  try {
    const parsed = new URL(url);
    return parsed.protocol === "https:" || parsed.protocol === "http:";
  } catch {
    return false;
  }
}

function isEmbeddableDashboardResultPageUrl(url: string) {
  try {
    const parsed = new URL(url);
    return parsed.protocol === "https:" || (parsed.protocol === "http:" && isLoopbackHost(parsed.hostname));
  } catch {
    return false;
  }
}

/**
 * Renders the dashboard-local result-page shell so formal `result_page`
 * deliveries can stay inside the desktop dashboard instead of always jumping
 * straight to an external browser tab.
 *
 * @returns Dashboard result-page route content.
 */
export function DashboardResultPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const routeState = readDashboardResultPageLocation({
    search: location.search,
    state: location.state,
  });
  const resultUrl = routeState?.url ?? null;
  const canOpenExternally = resultUrl ? isAllowedDashboardResultPageUrl(resultUrl) : false;
  const canEmbed = resultUrl ? isEmbeddableDashboardResultPageUrl(resultUrl) : false;
  const hostLabel = useMemo(() => {
    if (!resultUrl || !canOpenExternally) {
      return null;
    }

    try {
      return new URL(resultUrl).host;
    } catch {
      return null;
    }
  }, [canOpenExternally, resultUrl]);

  if (!resultUrl || !canOpenExternally) {
    return (
      <main className="dashboard-page dashboard-result-page">
        <DashboardBackHomeLink />
        <section className="dashboard-page__hero dashboard-result-page__hero">
          <div className="dashboard-page__hero-copy">
            <p className="dashboard-page__eyebrow">result page</p>
            <div className="dashboard-page__title-row">
              <FileSearch className="dashboard-page__title-icon" />
              <h1>结果页不可用</h1>
            </div>
            <p className="dashboard-page__description">当前没有可嵌入的结果页地址，或者链接协议不受支持。</p>
          </div>
        </section>
      </main>
    );
  }

  const showBrowserOnlyFallback = !canEmbed;

  return (
    <main className="dashboard-page dashboard-result-page">
      <DashboardBackHomeLink />

      <section className="dashboard-page__hero dashboard-result-page__hero">
        <div className="dashboard-page__hero-copy">
          <p className="dashboard-page__eyebrow">result page</p>
          <div className="dashboard-page__title-row">
            <PanelRightOpen className="dashboard-page__title-icon" />
            <h1>{routeState?.title?.trim() || "结果页承接"}</h1>
          </div>
          <p className="dashboard-page__description">
            {showBrowserOnlyFallback
              ? "当前结果页地址不满足站内嵌入条件，已切换为浏览器承接模式；你仍然可以回到任务详情继续查看正式上下文。"
              : "当前交付使用正式 `result_page` 入口承接，优先留在 dashboard 内查看；需要时也可以切回任务详情或外部浏览器。"}
          </p>
        </div>

        <div className="dashboard-card dashboard-card--status">
          <p className="dashboard-card__kicker">当前来源</p>
          <div className="dashboard-card__status-row">
            <FileSearch className="h-4 w-4" />
            <span>{hostLabel ?? resultUrl}</span>
          </div>
          <div className="dashboard-result-page__actions">
            {routeState?.taskId ? (
              <button className="dashboard-result-page__action" onClick={() => navigateToDashboardTaskDetail(navigate, routeState.taskId!)} type="button">
                返回任务详情
              </button>
            ) : null}
            <button
              className="dashboard-result-page__action"
              onClick={() => window.open(resultUrl, "_blank", "noopener,noreferrer")}
              type="button"
            >
              <ArrowUpRight className="h-4 w-4" />
              浏览器打开
            </button>
          </div>
        </div>
      </section>

      {showBrowserOnlyFallback ? null : (
        <section className="dashboard-result-page__frame-shell">
          <iframe
            className="dashboard-result-page__frame"
            referrerPolicy="no-referrer"
            sandbox="allow-downloads allow-forms allow-popups allow-popups-to-escape-sandbox allow-scripts"
            src={resultUrl}
            title={routeState?.title?.trim() || "dashboard-result-page"}
          />
        </section>
      )}
    </main>
  );
}
