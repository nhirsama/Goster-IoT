"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { queryKeys } from "@/lib/query-keys";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DashboardPanel, DashboardToolbar } from "@/components/dashboard/dashboard-panel";
import { EmptyState } from "@/components/dashboard/empty-state";
import { PageHeader } from "@/components/dashboard/page-header";
import { StatCard } from "@/components/dashboard/stat-card";
import { Fingerprint, RefreshCw, Search, Server, ShieldAlert, Wifi, WifiOff } from "lucide-react";

type DeviceRecord = components["schemas"]["DeviceRecord"];

const DEVICE_STATUS_META: Record<number, { label: string; className: string }> = {
  0: { label: "待认证", className: "bg-amber-500/10 border-amber-500/20 text-amber-600" },
  1: { label: "在线", className: "bg-emerald-500/10 border-emerald-500/20 text-emerald-600" },
  2: { label: "休眠", className: "bg-slate-500/10 border-slate-500/20 text-slate-600" },
};

export default function DevicesPage() {
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const [keyword, setKeyword] = useState("");
  const [groupId, setGroupId] = useState("");
  const groupFilter = groupId.trim();

  const { data: deviceData, isLoading, isFetching, refetch } = useQuery({
    queryKey: [...queryKeys.devicesByStatus("authenticated"), groupFilter || "__all__"],
    queryFn: () =>
      api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", {
        status: "authenticated",
        group_id: groupFilter || undefined,
      }),
    enabled: isAuthenticated,
    refetchInterval: 10000,
  });

  const filteredDevices = useMemo(() => {
    const devices = deviceData?.items || [];
    const normalized = keyword.trim().toLowerCase();
    if (!normalized) return devices;
    return devices.filter((device) => {
      const byName = device.meta.name.toLowerCase().includes(normalized);
      const byUuid = device.uuid.toLowerCase().includes(normalized);
      const bySn = device.meta.sn.toLowerCase().includes(normalized);
      return byName || byUuid || bySn;
    });
  }, [deviceData?.items, keyword]);
  const devices = deviceData?.items || [];
  const hasFilters = Boolean(keyword.trim() || groupFilter);

  if (authLoading) {
    return (
      <EmptyState
        icon={RefreshCw}
        title="正在校验会话状态"
        description="请稍候..."
        className="py-24"
      />
    );
  }

  if (!isAuthenticated) {
    return (
      <EmptyState
        icon={ShieldAlert}
        title="需要登录"
        description="请先登录后再访问设备列表。"
        className="py-24"
      />
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Server}
        title="在线设备"
        description="浏览当前在线设备并进入详情页查看实时指标。"
        action={
          <Button variant="outline" onClick={() => refetch()} disabled={isFetching}>
            <RefreshCw className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
            刷新
          </Button>
        }
      />

      <div className="grid gap-4 md:grid-cols-3">
        <StatCard title="在线设备" value={devices.length} hint="当前认证通过的设备" icon={Wifi} tone="success" />
        <StatCard
          title="匹配结果"
          value={filteredDevices.length}
          hint={hasFilters ? "已应用搜索或分组筛选" : "未应用筛选"}
          icon={Search}
          tone="primary"
        />
        <StatCard
          title="分组范围"
          value={groupFilter ? "指定分组" : "全部分组"}
          hint={groupFilter || "未指定 group_id"}
          icon={Server}
          tone="neutral"
        />
      </div>

      <DashboardPanel
        title="设备列表"
        description="先筛选再进入详情，列表保持自动同步。"
        action={
          <Badge variant="outline" className="w-fit rounded-full bg-white/70 text-slate-600">
            <Wifi className="mr-1 h-3.5 w-3.5 text-emerald-500" />
            共 {filteredDevices.length} 台
          </Badge>
        }
      >
        <div className="space-y-4 p-4 sm:p-5">
          <DashboardToolbar>
            <div className="grid gap-3 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)_auto] lg:items-center">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
                <Input
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  placeholder="搜索设备名称 / UUID / SN"
                  className="h-10 bg-white pl-9"
                />
              </div>
              <Input
                value={groupId}
                onChange={(event) => setGroupId(event.target.value)}
                placeholder="按分组过滤（group_id）"
                className="h-10 bg-white font-mono"
              />
              <Button
                variant="outline"
                className="h-10 bg-white"
                onClick={() => {
                  setKeyword("");
                  setGroupId("");
                }}
                disabled={!hasFilters}
              >
                清空筛选
              </Button>
            </div>
          </DashboardToolbar>

          <div className="overflow-hidden rounded-xl border border-slate-200/70 bg-white">
            {isLoading ? (
              <EmptyState icon={RefreshCw} title="正在加载设备列表" description="设备数据每 10 秒自动同步。" className="py-16" />
            ) : filteredDevices.length === 0 ? (
              <EmptyState
                icon={WifiOff}
                title={hasFilters ? "未找到匹配设备" : "暂无在线设备"}
                description={hasFilters ? "请尝试其他关键词或清空筛选。" : "设备上线后会显示在这里。"}
                className="py-16"
              />
            ) : (
              <div className="divide-y divide-slate-200/60">
                {filteredDevices.map((device: DeviceRecord) => {
                  const status = DEVICE_STATUS_META[device.runtime?.status ?? 2] || DEVICE_STATUS_META[2];
                  return (
                    <Link
                      key={device.uuid}
                      href={`/devices/detail?uuid=${encodeURIComponent(device.uuid)}`}
                      className="group grid gap-3 px-4 py-4 transition hover:bg-slate-50/80 sm:grid-cols-[minmax(0,1fr)_minmax(160px,auto)_auto] sm:items-center sm:px-5"
                    >
                      <div className="min-w-0 space-y-1">
                        <div className="flex items-center gap-2">
                          <p className="truncate text-sm font-semibold text-slate-900">{device.meta.name}</p>
                          <Badge variant="outline" className={status.className}>
                            {status.label}
                          </Badge>
                        </div>
                        <div className="flex items-center gap-1 text-xs text-slate-500">
                          <Fingerprint className="h-3.5 w-3.5" />
                          <span className="truncate font-mono">{device.uuid}</span>
                        </div>
                      </div>
                      <div className="min-w-0 text-xs text-slate-500 sm:text-right">
                        <p className="font-mono text-slate-700">{device.meta.sn}</p>
                        <p className="mt-1 truncate font-mono text-slate-400">{device.meta.mac}</p>
                      </div>
                      <div className="text-xs font-semibold text-primary/70 transition group-hover:text-primary">查看详情</div>
                    </Link>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </DashboardPanel>
    </div>
  );
}
