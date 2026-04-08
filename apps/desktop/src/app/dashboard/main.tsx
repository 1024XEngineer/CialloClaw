import ReactDOM from "react-dom/client";
import { DashboardApp } from "@/features/dashboard/DashboardApp";
import { AppProviders } from "@/features/shared/AppProviders";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <AppProviders>
    <DashboardApp />
  </AppProviders>,
);
