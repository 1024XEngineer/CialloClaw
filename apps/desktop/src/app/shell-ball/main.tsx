// This entry mounts the shell-ball window.
import ReactDOM from "react-dom/client";
import { AppProviders } from "@/features/shared/AppProviders";
import { ShellBallApp } from "@/features/shell-ball/ShellBallApp";
import "@/features/shell-ball/shellBall.css";

const rootElement = document.getElementById("root")!;

document.documentElement.dataset.appWindow = "shell-ball";
document.body.dataset.appWindow = "shell-ball";
rootElement.dataset.appWindow = "shell-ball";
document.documentElement.setAttribute("data-app-window", "shell-ball");
document.body.setAttribute("data-app-window", "shell-ball");
rootElement.setAttribute("data-app-window", "shell-ball");

ReactDOM.createRoot(rootElement).render(
  <AppProviders>
    <ShellBallApp />
  </AppProviders>,
);
