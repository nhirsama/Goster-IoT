"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { EmptyState } from "@/components/dashboard/empty-state";
import { useAuth } from "@/hooks/use-auth";
import { RefreshCw, ShieldAlert } from "lucide-react";

function getManagementLanding(permission: number): string {
  if (permission >= 3) return "/users";
  if (permission >= 2) return "/pending";
  if (permission >= 1) return "/blacklist";
  return "/";
}

export default function AdminPage() {
  const router = useRouter();
  const { user, isAuthenticated, isLoading } = useAuth();

  useEffect(() => {
    if (isLoading) {
      return;
    }
    if (!isAuthenticated) {
      router.replace("/login");
      return;
    }
    router.replace(getManagementLanding(user?.permission || 0));
  }, [isAuthenticated, isLoading, router, user?.permission]);

  if (!isLoading && !isAuthenticated) {
    return <EmptyState icon={ShieldAlert} title="需要登录" description="请先登录后再访问管理模块。" className="py-24" />;
  }

  return <EmptyState icon={RefreshCw} title="正在进入管理模块" description="请稍候..." className="py-24" />;
}
