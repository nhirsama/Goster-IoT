import type { ReactNode } from "react";
import { redirect } from "next/navigation";
import DashboardShell from "@/components/dashboard/dashboard-shell";
import { getServerAuthSession } from "@/lib/server-auth";

export default async function DashboardLayout({ children }: { children: ReactNode }) {
  const session = await getServerAuthSession();

  if (!session) {
    redirect("/login");
  }

  return <DashboardShell initialUser={session}>{children}</DashboardShell>;
}
