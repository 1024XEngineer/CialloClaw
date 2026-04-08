// 该入口负责挂载仪表盘窗口。
import ReactDOM from "react-dom/client";
import { DashboardApp } from "@/features/dashboard/DashboardApp";
import { AppProviders } from "@/features/shared/AppProviders";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <AppProviders>
    <DashboardApp />
  </AppProviders>,
);
