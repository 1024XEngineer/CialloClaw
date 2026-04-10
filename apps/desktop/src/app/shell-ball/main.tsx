// 该入口负责挂载悬浮球窗口。
import ReactDOM from "react-dom/client";
import { AppProviders } from "@/features/shared/AppProviders";
import { ShellBallApp } from "@/features/shell-ball/ShellBallApp";
import "@/features/shell-ball/shellBall.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <AppProviders>
    <ShellBallApp />
  </AppProviders>,
);
