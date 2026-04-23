import { Theme } from "@radix-ui/themes";
import "@radix-ui/themes/styles.css";
import ReactDOM from "react-dom/client";
import { OnboardingWindow } from "@/features/onboarding/OnboardingWindow";
import { AppProviders } from "@/features/shared/AppProviders";

const rootElement = document.getElementById("root")!;

document.documentElement.dataset.appWindow = "onboarding";
document.body.dataset.appWindow = "onboarding";
rootElement.dataset.appWindow = "onboarding";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <Theme appearance="light" panelBackground="translucent" accentColor="orange" grayColor="sand" radius="large">
    <AppProviders>
      <OnboardingWindow />
    </AppProviders>
  </Theme>,
);
