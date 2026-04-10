import { DashboardBackHomeLink } from "@/features/dashboard/shared/DashboardBackHomeLink";
import { SecurityPageShell } from "./SecurityPageShell";

export function SafetyPage() {
  return (
    <>
      <DashboardBackHomeLink />
      <SecurityPageShell />
    </>
  );
}
