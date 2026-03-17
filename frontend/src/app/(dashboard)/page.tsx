"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/hooks/use-auth";
import { api } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { queryKeys } from "@/lib/query-keys";
import { EmptyState } from "@/components/dashboard/empty-state";
import { PageHeader } from "@/components/dashboard/page-header";
import { StatCard } from "@/components/dashboard/stat-card";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Activity,
  ArrowRight,
  BellRing,
  Blocks,
  RefreshCw,
  ShieldCheck,
  ShieldAlert,
  Wifi,
  Ban,
  Settings,
} from "lucide-react";

export default function DashboardHome() {
  const { user, isAuthenticated, isLoading } = useAuth();
  const permission = user?.permission || 0;

  const { data: activeData } = useQuery({
    queryKey: queryKeys.devicesByStatus("authenticated"),
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "authenticated" }),
    enabled: isAuthenticated,
    refetchInterval: 10000,
  });

  const { data: pendingData } = useQuery({
    queryKey: queryKeys.devicesByStatus("pending"),
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "pending" }),
    enabled: isAuthenticated && permission >= 2,
    refetchInterval: 10000,
  });

  const { data: revokedData } = useQuery({
    queryKey: queryKeys.devicesByStatus("revoked"),
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "revoked" }),
    enabled: isAuthenticated && permission >= 1,
    refetchInterval: 20000,
  });

  const { data: refusedData } = useQuery({
    queryKey: queryKeys.devicesByStatus("refused"),
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "refused" }),
    enabled: isAuthenticated && permission >= 1,
    refetchInterval: 20000,
  });

  if (isLoading) {
    return <EmptyState icon={RefreshCw} title="正在加载控制台" description="请稍候..." className="py-24" />;
  }

  if (!isAuthenticated) {
    return <EmptyState icon={ShieldAlert} title="需要登录" description="请先登录后再访问设备控制台。" className="py-24" />;
  }

  const onlineCount = activeData?.items?.length || 0;
  const pendingCount = pendingData?.items?.length || 0;
  const blacklistCount = (revokedData?.items?.length || 0) + (refusedData?.items?.length || 0);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Blocks}
        title="设备控制台"
        description="集中查看设备在线状态、安全审批和系统入口，所有数据自动刷新。"
        action={
          <Badge className="rounded-full bg-white/80 px-3 py-1 text-xs text-slate-600 ring-1 ring-slate-200">
            <Activity className="mr-1 h-3.5 w-3.5 text-emerald-500" />
            实时同步中
          </Badge>
        }
      />

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard title="在线设备" value={onlineCount} hint="每 10 秒自动刷新" icon={Wifi} tone="success" />
        <StatCard title="待审批设备" value={permission >= 2 ? pendingCount : "-"} hint="需要 ReadWrite 及以上权限" icon={BellRing} tone="warning" />
        <StatCard title="黑名单设备" value={permission >= 1 ? blacklistCount : "-"} hint="包含拒绝和撤销设备" icon={Ban} tone="neutral" />
        <StatCard title="当前权限" value={permission === 3 ? "管理员" : permission === 2 ? "读写" : "只读"} hint={user?.username || "当前用户"} icon={ShieldCheck} tone="primary" />
      </div>

      <div className="grid gap-4 xl:grid-cols-3">
        <Card className="xl:col-span-2">
          <CardHeader className="border-b border-slate-200/70">
            <CardTitle className="text-lg font-semibold">快捷入口</CardTitle>
            <CardDescription>按场景组织常用功能，减少跳转层级。</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3 pt-4 sm:grid-cols-2">
            <Link href="/devices" className="rounded-2xl border border-slate-200 bg-white/75 p-4 transition hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-md">
              <p className="text-sm font-semibold text-slate-900">设备列表</p>
              <p className="mt-1 text-xs text-slate-500">查看在线设备与实时状态</p>
            </Link>
            <Link href="/admin" className="rounded-2xl border border-slate-200 bg-white/75 p-4 transition hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-md">
              <p className="text-sm font-semibold text-slate-900">管理中心</p>
              <p className="mt-1 text-xs text-slate-500">审批、黑名单与权限管理入口</p>
            </Link>
            {permission >= 2 ? (
              <Link href="/pending" className="rounded-2xl border border-slate-200 bg-white/75 p-4 transition hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-md">
                <p className="text-sm font-semibold text-slate-900">审批队列</p>
                <p className="mt-1 text-xs text-slate-500">快速处理新设备接入申请</p>
              </Link>
            ) : null}
            {permission >= 1 ? (
              <Link href="/blacklist" className="rounded-2xl border border-slate-200 bg-white/75 p-4 transition hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-md">
                <p className="text-sm font-semibold text-slate-900">黑名单管理</p>
                <p className="mt-1 text-xs text-slate-500">恢复误拦设备并追踪风险来源</p>
              </Link>
            ) : null}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="border-b border-slate-200/70">
            <CardTitle className="text-lg font-semibold">建议操作</CardTitle>
            <CardDescription>根据当前状态推荐下一步。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 pt-4">
            {permission >= 2 && pendingCount > 0 ? (
              <Button asChild className="w-full justify-between">
                <Link href="/pending">
                  处理待审批设备
                  <ArrowRight />
                </Link>
              </Button>
            ) : (
              <Button asChild className="w-full justify-between" variant="outline">
                <Link href="/devices">
                  查看在线设备
                  <ArrowRight />
                </Link>
              </Button>
            )}

            {permission >= 3 ? (
              <Button asChild className="w-full justify-between" variant="secondary">
                <Link href="/users">
                  调整用户权限
                  <Settings />
                </Link>
              </Button>
            ) : null}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
