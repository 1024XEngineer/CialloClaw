// 该入口负责挂载控制面板窗口。
import { Theme } from "@radix-ui/themes";
import "@radix-ui/themes/styles.css";
import ReactDOM from "react-dom/client";
import { ControlPanelApp } from "@/features/control-panel/ControlPanelApp";
import { AppProviders } from "@/features/shared/AppProviders";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <Theme appearance="light" panelBackground="solid" accentColor="orange" grayColor="sand" radius="large">
    <AppProviders>
      <ControlPanelApp />
    </AppProviders>
  </Theme>,
);
