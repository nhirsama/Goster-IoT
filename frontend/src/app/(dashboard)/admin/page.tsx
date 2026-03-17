import { redirect } from "next/navigation";
import { getServerAuthSession } from "@/lib/server-auth";

function getManagementLanding(permission: number): string {
  if (permission >= 3) return "/users";
  if (permission >= 2) return "/pending";
  if (permission >= 1) return "/blacklist";
  return "/";
}

export default async function AdminPage() {
  const session = await getServerAuthSession();

  if (!session) {
    redirect("/login");
  }

  redirect(getManagementLanding(session.permission || 0));
}
