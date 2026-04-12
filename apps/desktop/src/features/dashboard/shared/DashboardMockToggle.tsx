import "./dashboardMockToggle.css";
import { isDashboardMockModeEnabled } from "./dashboardDataMode";

type DashboardMockToggleProps = {
  enabled: boolean;
  onToggle: () => void;
};

export function DashboardMockToggle({ enabled, onToggle }: DashboardMockToggleProps) {
  if (!isDashboardMockModeEnabled()) {
    return null;
  }

  return (
    <button
      aria-label={enabled ? "切换为实时数据" : "切换为 Mock 数据"}
      aria-pressed={enabled}
      className={`dashboard-mock-toggle${enabled ? " is-active" : ""}`}
      onClick={onToggle}
      type="button"
    >
      <span className="dashboard-mock-toggle__label">Mock</span>
      <span className="dashboard-mock-toggle__state">{enabled ? "ON" : "OFF"}</span>
    </button>
  );
}
