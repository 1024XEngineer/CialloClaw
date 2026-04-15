// This entrypoint mounts the standalone control panel window.
import { Theme } from "@radix-ui/themes";
import "@radix-ui/themes/styles.css";
import ReactDOM from "react-dom/client";
import { ControlPanelApp } from "@/features/control-panel/ControlPanelApp";
import { AppProviders } from "@/features/shared/AppProviders";
import { installDesktopEscapeClose } from "@/platform/desktopWindowFrame";
import { installHideOnCloseRequest } from "@/platform/hideOnCloseRequest";

// Keep close requests consistent with the other frameless desktop windows.
void installHideOnCloseRequest();
// Escape should dismiss the frameless window unless an editable field owns the key.
installDesktopEscapeClose();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <Theme appearance="light" panelBackground="solid" accentColor="orange" grayColor="sand" radius="large">
    <AppProviders>
      <ControlPanelApp />
    </AppProviders>
  </Theme>,
);
