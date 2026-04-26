import { DashboardBackHomeLink } from "@/features/dashboard/shared/DashboardBackHomeLink";
import { DashboardModuleFloatingNav } from "@/features/dashboard/shared/DashboardModuleFloatingNav";
import { SecurityPageShell } from "./SecurityPageShell";

export function SafetyPage() {
  return (
    <>
      <DashboardBackHomeLink />
      <DashboardModuleFloatingNav />
      <SecurityPageShell />
    </>
  );
}
