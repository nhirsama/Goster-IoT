"use client";

import { useRouter } from "next/navigation";
import { useAuth } from "@/hooks/use-auth";
import { getPermissionRoleLabel } from "@/lib/dashboard-meta";
import { PageHeader } from "@/components/dashboard/page-header";
import { DashboardPanel } from "@/components/dashboard/dashboard-panel";
import { StatCard } from "@/components/dashboard/stat-card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Building2, ChevronRight, Settings, Shield, User, Users } from "lucide-react";

export default function SettingsPage() {
  const router = useRouter();
  const { user } = useAuth();
  const permission = user?.permission || 0;
  const tenantCount = Object.keys(user?.tenant_roles || {}).length;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Settings}
        title="设置"
        description="管理当前账号信息和常用管理入口。"
      />

      <div className="grid gap-4 md:grid-cols-3">
        <StatCard title="当前账号" value={user?.username || "-"} hint="已登录用户" icon={User} tone="primary" />
        <StatCard title="当前权限" value={getPermissionRoleLabel(permission)} hint="由当前租户角色派生" icon={Shield} tone="success" />
        <StatCard title="可访问租户" value={tenantCount || "-"} hint="可在侧边栏底部切换" icon={Building2} tone="neutral" />
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.6fr)]">
        <DashboardPanel
          title="账户信息"
          description="这些信息由登录会话和当前租户上下文提供。"
          contentClassName="p-5"
        >
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
              <p className="text-xs font-semibold text-slate-500">用户名</p>
              <p className="mt-1 text-sm font-semibold text-slate-900">{user?.username || "-"}</p>
            </div>
            <div className="rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
              <p className="text-xs font-semibold text-slate-500">权限级别</p>
              <p className="mt-1 text-sm font-semibold text-slate-900">
                {permission === 3
                  ? "管理员"
                  : permission === 2
                    ? "读写权限"
                    : permission === 1
                      ? "只读权限"
                      : "无权限"}
              </p>
            </div>
            <div className="rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3 sm:col-span-2">
              <p className="text-xs font-semibold text-slate-500">租户角色</p>
              <div className="mt-2 flex flex-wrap gap-2">
                {Object.entries(user?.tenant_roles || {}).length ? (
                  Object.entries(user?.tenant_roles || {}).map(([tenantId, role]) => (
                    <Badge key={tenantId} variant="outline" className="h-6 bg-white text-slate-600">
                      <span className="font-mono">{tenantId}</span>
                      <span className="text-slate-400">·</span>
                      {role}
                    </Badge>
                  ))
                ) : (
                  <span className="text-sm text-slate-500">暂无租户角色</span>
                )}
              </div>
            </div>
          </div>
        </DashboardPanel>

        <DashboardPanel
          title="管理入口"
          description="根据当前权限展示可用入口。"
          contentClassName="p-5"
        >
          <div className="space-y-3">
            <Button
              variant="outline"
              className="h-auto w-full justify-between bg-white px-4 py-3"
              onClick={() => router.push("/tenants")}
            >
              <span className="flex items-center gap-2">
                <Building2 className="h-4 w-4" />
                租户管理
              </span>
              <ChevronRight className="h-4 w-4 text-slate-400" />
            </Button>
            {permission >= 3 ? (
              <Button
                variant="outline"
                className="h-auto w-full justify-between bg-white px-4 py-3"
                onClick={() => router.push("/users")}
              >
                <span className="flex items-center gap-2">
                  <Users className="h-4 w-4" />
                  用户管理
                </span>
                <ChevronRight className="h-4 w-4 text-slate-400" />
              </Button>
            ) : null}
            <Button
              variant="outline"
              className="h-auto w-full justify-between bg-white px-4 py-3"
              onClick={() => router.push(permission >= 2 ? "/pending" : "/blacklist")}
            >
              <span className="flex items-center gap-2">
                <Shield className="h-4 w-4" />
                安全队列
              </span>
              <ChevronRight className="h-4 w-4 text-slate-400" />
            </Button>
          </div>
        </DashboardPanel>
      </div>
    </div>
  );
}
