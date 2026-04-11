import ReactDOM from "react-dom/client";
import { AppProviders } from "@/features/shared/AppProviders";
import { ShellBallPinnedBubbleWindow } from "@/features/shell-ball/ShellBallPinnedBubbleWindow";
import "@/features/shell-ball/shellBall.css";

const rootElement = document.getElementById("root")!;

document.documentElement.dataset.appWindow = "shell-ball-bubble-pinned";
document.body.dataset.appWindow = "shell-ball-bubble-pinned";
rootElement.dataset.appWindow = "shell-ball-bubble-pinned";
document.documentElement.setAttribute("data-app-window", "shell-ball-bubble-pinned");
document.body.setAttribute("data-app-window", "shell-ball-bubble-pinned");
rootElement.setAttribute("data-app-window", "shell-ball-bubble-pinned");

ReactDOM.createRoot(rootElement).render(
  <AppProviders>
    <ShellBallPinnedBubbleWindow />
  </AppProviders>,
);
