import ReactDOM from "react-dom/client";
import { ControlPanelApp } from "@/features/control-panel/ControlPanelApp";
import { AppProviders } from "@/features/shared/AppProviders";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <AppProviders>
    <ControlPanelApp />
  </AppProviders>,
);
