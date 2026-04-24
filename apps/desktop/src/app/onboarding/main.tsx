import { Theme } from "@radix-ui/themes";
import "@radix-ui/themes/styles.css";
import ReactDOM from "react-dom/client";
import { OnboardingWindow } from "@/features/onboarding/OnboardingWindow";
import { AppProviders } from "@/features/shared/AppProviders";

const rootElement = document.getElementById("root")!;

document.documentElement.dataset.appWindow = "onboarding";
document.body.dataset.appWindow = "onboarding";
rootElement.dataset.appWindow = "onboarding";
document.documentElement.style.background = "transparent";
document.documentElement.style.backgroundColor = "transparent";
document.body.style.background = "transparent";
document.body.style.backgroundColor = "transparent";
rootElement.style.background = "transparent";
rootElement.style.backgroundColor = "transparent";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <Theme
    appearance="light"
    panelBackground="translucent"
    accentColor="orange"
    grayColor="sand"
    radius="large"
    style={{ background: "transparent", backgroundColor: "transparent" }}
  >
    <AppProviders>
      <OnboardingWindow />
    </AppProviders>
  </Theme>,
);
