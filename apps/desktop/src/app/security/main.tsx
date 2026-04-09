import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Theme } from "@radix-ui/themes";
import "@radix-ui/themes/styles.css";
import { AppProviders } from "@/features/shared/AppProviders";
import { SecurityApp } from "@/features/security/SecurityApp";

const container = document.getElementById("root");

if (!container) {
  throw new Error("Failed to find root container for security app.");
}

createRoot(container).render(
  <StrictMode>
    <Theme appearance="light" panelBackground="solid" accentColor="orange" grayColor="sand" radius="large">
      <AppProviders>
        <SecurityApp />
      </AppProviders>
    </Theme>
  </StrictMode>,
);
