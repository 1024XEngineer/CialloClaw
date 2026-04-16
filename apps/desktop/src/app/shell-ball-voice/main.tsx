import ReactDOM from "react-dom/client";
import { AppProviders } from "@/features/shared/AppProviders";
import { ShellBallVoiceWindow } from "@/features/shell-ball/ShellBallVoiceWindow";
import "@/features/shell-ball/shellBall.css";

const rootElement = document.getElementById("root")!;

document.documentElement.dataset.appWindow = "shell-ball-voice";
document.body.dataset.appWindow = "shell-ball-voice";
rootElement.dataset.appWindow = "shell-ball-voice";
document.documentElement.setAttribute("data-app-window", "shell-ball-voice");
document.body.setAttribute("data-app-window", "shell-ball-voice");
rootElement.setAttribute("data-app-window", "shell-ball-voice");

ReactDOM.createRoot(rootElement).render(
  <AppProviders>
    <ShellBallVoiceWindow />
  </AppProviders>,
);
