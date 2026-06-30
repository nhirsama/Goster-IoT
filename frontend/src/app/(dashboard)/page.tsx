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
import { DashboardPanel } from "@/components/dashboard/dashboard-panel";
import { Badge } from "@/components/ui/badge";
import {
  Activity,
  BellRing,
  Blocks,
  ChevronRight,
  RefreshCw,
  ShieldCheck,
  ShieldAlert,
  Wifi,
  Ban,
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
  const quickLinks = [
    {
      href: "/devices",
      title: "设备列表",
      description: "查看在线设备与实时状态",
      icon: Wifi,
      visible: true,
    },
    {
      href: "/pending",
      title: "审批队列",
      description: "快速处理新设备接入申请",
      icon: BellRing,
      visible: permission >= 2,
    },
    {
      href: "/blacklist",
      title: "黑名单管理",
      description: "恢复误拦设备并追踪风险来源",
      icon: Ban,
      visible: permission >= 1,
    },
    {
      href: "/tenants",
      title: "租户管理",
      description: "维护当前租户资料与成员",
      icon: ShieldCheck,
      visible: permission >= 3,
    },
  ].filter((item) => item.visible);

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

      <DashboardPanel title="快捷入口" description="按场景组织常用功能，减少跳转层级。" contentClassName="p-4 sm:p-5">
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {quickLinks.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className="group rounded-2xl border border-slate-200 bg-white/75 p-4 transition hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-md"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="rounded-xl bg-primary/10 p-2 text-primary">
                  <item.icon className="h-4 w-4" />
                </div>
                <ChevronRight className="h-4 w-4 text-slate-300 transition group-hover:translate-x-0.5 group-hover:text-primary" />
              </div>
              <p className="mt-4 text-sm font-semibold text-slate-900">{item.title}</p>
              <p className="mt-1 text-xs text-slate-500">{item.description}</p>
            </Link>
          ))}
        </div>
      </DashboardPanel>
    </div>
  );
}
