import ReactDOM from "react-dom/client";
import { AppProviders } from "@/features/shared/AppProviders";
import { ShellBallInputWindow } from "@/features/shell-ball/ShellBallInputWindow";
import "@/features/shell-ball/shellBall.css";

const rootElement = document.getElementById("root")!;

document.documentElement.dataset.appWindow = "shell-ball-input";
document.body.dataset.appWindow = "shell-ball-input";
rootElement.dataset.appWindow = "shell-ball-input";
document.documentElement.setAttribute("data-app-window", "shell-ball-input");
document.body.setAttribute("data-app-window", "shell-ball-input");
rootElement.setAttribute("data-app-window", "shell-ball-input");

ReactDOM.createRoot(rootElement).render(
  <AppProviders>
    <ShellBallInputWindow />
  </AppProviders>,
);
