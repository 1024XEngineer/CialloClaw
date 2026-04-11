import ReactDOM from "react-dom/client";
import { AppProviders } from "@/features/shared/AppProviders";
import { ShellBallBubbleWindow } from "@/features/shell-ball/ShellBallBubbleWindow";
import "@/features/shell-ball/shellBall.css";

const rootElement = document.getElementById("root")!;

document.documentElement.dataset.appWindow = "shell-ball-bubble";
document.body.dataset.appWindow = "shell-ball-bubble";
rootElement.dataset.appWindow = "shell-ball-bubble";
document.documentElement.setAttribute("data-app-window", "shell-ball-bubble");
document.body.setAttribute("data-app-window", "shell-ball-bubble");
rootElement.setAttribute("data-app-window", "shell-ball-bubble");

ReactDOM.createRoot(rootElement).render(
  <AppProviders>
    <ShellBallBubbleWindow />
  </AppProviders>,
);
