import { AlertTriangle, RefreshCcw } from "lucide-react";
import { Component, type ErrorInfo, type ReactNode } from "react";
import { Button } from "@/components/ui/button";

type DashboardWindowErrorBoundaryProps = {
  children: ReactNode;
};

type DashboardWindowErrorBoundaryState = {
  hasError: boolean;
};

/**
 * React 18 still requires a class component to catch render-phase errors.
 * Keep that legacy API isolated behind a small wrapper so the rest of the
 * dashboard tree can stay on hooks and function components.
 */
export function DashboardWindowErrorBoundary(props: DashboardWindowErrorBoundaryProps) {
  return <DashboardWindowErrorBoundaryImpl {...props} />;
}

/**
 * Keeps the dashboard window recoverable when long-lived desktop sessions hit
 * an unexpected render-time exception. Without a window-level boundary the
 * whole React tree disappears and the user only sees the background shell.
 */
class DashboardWindowErrorBoundaryImpl extends Component<
  DashboardWindowErrorBoundaryProps,
  DashboardWindowErrorBoundaryState
> {
  state: DashboardWindowErrorBoundaryState = {
    hasError: false,
  };

  static getDerivedStateFromError() {
    return {
      hasError: true,
    } satisfies DashboardWindowErrorBoundaryState;
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("dashboard window render failed", error, errorInfo);
  }

  private handleReload = () => {
    window.location.reload();
  };

  render() {
    if (!this.state.hasError) {
      return this.props.children;
    }

    return (
      <main className="dashboard-page">
        <section className="dashboard-page__hero">
          <div className="dashboard-page__hero-copy">
            <p className="dashboard-page__eyebrow">dashboard recovery</p>
            <div className="dashboard-page__title-row">
              <AlertTriangle className="dashboard-page__title-icon" />
              <h1>仪表盘需要恢复</h1>
            </div>
            <p className="dashboard-page__description">
              这个窗口在驻留期间遇到了未处理的前端异常。重新加载后会重新挂载 dashboard
              路由树，并恢复到当前窗口会话。
            </p>
          </div>

          <div className="dashboard-card dashboard-card--status">
            <p className="dashboard-card__kicker">恢复操作</p>
            <div className="dashboard-card__list">
              <Button className="dashboard-orbit-panel__primary-button" onClick={this.handleReload} type="button">
                重新加载仪表盘
                <RefreshCcw className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </section>
      </main>
    );
  }
}
