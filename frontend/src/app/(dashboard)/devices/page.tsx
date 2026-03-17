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
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { EmptyState } from "@/components/dashboard/empty-state";
import { PageHeader } from "@/components/dashboard/page-header";
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

  const { data: deviceData, isLoading, isFetching, refetch } = useQuery({
    queryKey: queryKeys.devicesByStatus("authenticated"),
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "authenticated" }),
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
    <div className="space-y-6 lg:max-w-4xl">
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

      <Card>
        <CardHeader className="border-b border-slate-200/70 pb-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <CardTitle className="text-base font-semibold text-slate-900">设备列表</CardTitle>
            <Badge variant="outline" className="w-fit rounded-full bg-white/70 text-slate-600">
              <Wifi className="mr-1 h-3.5 w-3.5 text-emerald-500" />
              共 {filteredDevices.length} 台
            </Badge>
          </div>
          <div className="relative mt-2">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <Input
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              placeholder="搜索设备名称 / UUID / SN"
              className="h-10 pl-9"
            />
          </div>
        </CardHeader>

        <CardContent className="px-0 py-0">
          {isLoading ? (
            <EmptyState icon={RefreshCw} title="正在加载设备列表" description="设备数据每 10 秒自动同步。" className="py-16" />
          ) : filteredDevices.length === 0 ? (
            <EmptyState
              icon={WifiOff}
              title={keyword ? "未找到匹配设备" : "暂无在线设备"}
              description={keyword ? "请尝试其他关键词。" : "设备上线后会显示在这里。"}
              className="py-16"
            />
          ) : (
            <div className="divide-y divide-slate-200/60">
              {filteredDevices.map((device: DeviceRecord) => {
                const status = DEVICE_STATUS_META[device.runtime?.status ?? 2] || DEVICE_STATUS_META[2];
                return (
                  <Link
                    key={device.uuid}
                    href={`/devices/${device.uuid}`}
                    className="group flex items-center justify-between gap-3 px-4 py-4 transition hover:bg-slate-50/70"
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
                    <div className="text-xs font-medium text-slate-400 transition group-hover:text-slate-600">查看详情</div>
                  </Link>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
