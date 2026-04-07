import ReactDOM from "react-dom/client";
import { AppProviders } from "@/features/shared/AppProviders";
import { ShellBallApp } from "@/features/shell-ball/ShellBallApp";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <AppProviders>
    <ShellBallApp />
  </AppProviders>,
);
