"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { Fingerprint, Server, ChevronRight, WifiOff } from "lucide-react";
import { Button } from "@/components/ui/button";

type DeviceRecord = components["schemas"]["DeviceRecord"];

export default function DevicesPage() {
  const { isAuthenticated } = useAuth();

  const { data: deviceData, isLoading, refetch } = useQuery({
    queryKey: ["devices", "authenticated"],
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "authenticated" }),
    enabled: isAuthenticated,
    refetchInterval: 10000,
  });

  const devices = deviceData?.items || [];

  if (!isAuthenticated) return null;

  return (
    <div className="space-y-4 lg:max-w-2xl">
      <div className="flex items-end justify-between">
        <div>
          <h1 className="text-xl font-black text-slate-900">在线设备</h1>
          <p className="text-sm text-slate-500">与原版一致，每 10 秒自动刷新</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => refetch()}>
          刷新
        </Button>
      </div>

      <div className="rounded-2xl border border-slate-200 bg-white overflow-hidden">
        {isLoading ? (
          <div className="py-16 text-center text-slate-400">正在加载设备列表...</div>
        ) : devices.length === 0 ? (
          <div className="py-16 text-center text-slate-400">
            <WifiOff className="h-10 w-10 mx-auto mb-2 text-slate-300" />
            暂无在线设备
          </div>
        ) : (
          <div className="divide-y divide-slate-100">
            {devices.map((device: DeviceRecord) => (
              <Link
                key={device.uuid}
                href={`/devices/${device.uuid}`}
                className="flex items-center justify-between px-4 py-3 hover:bg-slate-50 transition-colors"
              >
                <div className="flex items-center gap-3 min-w-0">
                  <span
                    className={`h-2.5 w-2.5 rounded-full shrink-0 ${
                      device.runtime?.status === 1
                        ? "bg-emerald-500"
                        : device.runtime?.status === 2
                          ? "bg-amber-500"
                          : "bg-slate-300"
                    }`}
                  />
                  <div className="min-w-0">
                    <p className="font-bold text-slate-900 truncate">{device.meta.name}</p>
                    <div className="flex items-center gap-1 text-xs text-slate-400 mt-0.5">
                      <Fingerprint className="h-3 w-3" />
                      <span className="font-mono truncate">{device.uuid}</span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <Server className="h-4 w-4 text-slate-300" />
                  <ChevronRight className="h-4 w-4 text-slate-300" />
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
