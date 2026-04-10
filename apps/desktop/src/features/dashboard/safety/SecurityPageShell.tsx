import { Theme } from "@radix-ui/themes";
import "@radix-ui/themes/styles.css";
import { SecurityApp } from "./SecurityApp";

export function SecurityPageShell() {
  return (
    <Theme appearance="light" panelBackground="solid" accentColor="orange" grayColor="sand" radius="large">
      <SecurityApp />
    </Theme>
  );
}
