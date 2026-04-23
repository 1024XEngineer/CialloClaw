import { DashboardBackHomeLink } from "@/features/dashboard/shared/DashboardBackHomeLink";
import { DashboardModuleFloatingNav } from "@/features/dashboard/shared/DashboardModuleFloatingNav";
import { MirrorApp } from "./MirrorApp";

export function MemoryPage() {
  return (
    <>
      <DashboardBackHomeLink />
      <DashboardModuleFloatingNav />
      <MirrorApp />
    </>
  );
}
