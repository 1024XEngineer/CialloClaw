import { DashboardBackHomeLink } from "@/features/dashboard/shared/DashboardBackHomeLink";
import { DashboardModuleFloatingNav } from "@/features/dashboard/shared/DashboardModuleFloatingNav";
import { MirrorApp } from "./MirrorApp";

/**
 * Preserves the previous protocol-backed mirror page while the new mock memory
 * surface takes over the default dashboard route.
 */
export function LegacyMemoryPage() {
  return (
    <>
      <DashboardBackHomeLink />
      <DashboardModuleFloatingNav />
      <MirrorApp />
    </>
  );
}
