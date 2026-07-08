"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, getApiErrorMessage } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { queryKeys } from "@/lib/query-keys";
import { useUx } from "@/components/providers/ux-provider";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { DashboardPanel, DashboardToolbar } from "@/components/dashboard/dashboard-panel";
import { EmptyState } from "@/components/dashboard/empty-state";
import { PageHeader } from "@/components/dashboard/page-header";
import { StatCard } from "@/components/dashboard/stat-card";
import { Check, Copy, Fingerprint, Plus, RefreshCw, Search, Server, ShieldAlert, Wifi, WifiOff } from "lucide-react";

type DeviceRecord = components["schemas"]["DeviceRecord"];
type CreateDeviceRequest = components["schemas"]["CreateDeviceRequest"];
type ProvisionedDeviceData = components["schemas"]["ProvisionedDeviceData"];

const DEVICE_STATUS_META: Record<number, { label: string; className: string }> = {
  0: { label: "待认证", className: "bg-amber-500/10 border-amber-500/20 text-amber-600" },
  1: { label: "在线", className: "bg-emerald-500/10 border-emerald-500/20 text-emerald-600" },
  2: { label: "休眠", className: "bg-slate-500/10 border-slate-500/20 text-slate-600" },
};

export default function DevicesPage() {
  const { isAuthenticated, isLoading: authLoading, user } = useAuth();
  const queryClient = useQueryClient();
  const { toast } = useUx();
  const [keyword, setKeyword] = useState("");
  const [groupId, setGroupId] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [createdDevice, setCreatedDevice] = useState<ProvisionedDeviceData | null>(null);
  const [createForm, setCreateForm] = useState<CreateDeviceRequest>({
    name: "",
    sn: "",
    mac: "",
    hw_version: "",
    sw_version: "",
    config_version: "",
  });
  const groupFilter = groupId.trim();
  const permission = user?.permission || 0;

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

  const createDeviceMutation = useMutation({
    mutationFn: () =>
      api.post<ProvisionedDeviceData>("/api/v1/devices", {
        name: createForm.name.trim(),
        sn: createForm.sn?.trim() || undefined,
        mac: createForm.mac?.trim() || undefined,
        hw_version: createForm.hw_version?.trim() || undefined,
        sw_version: createForm.sw_version?.trim() || undefined,
        config_version: createForm.config_version?.trim() || undefined,
      }),
    onSuccess: (result) => {
      setCreatedDevice(result);
      toast.success("设备已创建，令牌已分配");
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("authenticated") });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "设备创建失败，请稍后重试"));
    },
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
  const canCreateDevice = permission >= 2;
  const canSubmitCreate = Boolean(createForm.name.trim() && ((createForm.sn || "").trim() || (createForm.mac || "").trim()));
  const mqttHost =
    process.env.NEXT_PUBLIC_MQTT_HOST ||
    (typeof window !== "undefined" && window.location.hostname ? window.location.hostname : "your-server-host");
  const mqttPort = process.env.NEXT_PUBLIC_MQTT_PORT || "1883";
  const mqttConfigText = createdDevice
    ? [
        `host=${mqttHost}`,
        `port=${mqttPort}`,
        `client_id=${createdDevice.mqtt.client_id}`,
        `username=${createdDevice.mqtt.username}`,
        `password=${createdDevice.mqtt.password}`,
        `telemetry_topic=${createdDevice.mqtt.telemetry_topic}`,
        `heartbeat_topic=${createdDevice.mqtt.heartbeat_topic}`,
        `downlink_topic=${createdDevice.mqtt.downlink_topic}`,
      ].join("\n")
    : "";

  const updateCreateForm = (key: keyof CreateDeviceRequest, value: string) => {
    setCreateForm((current) => ({ ...current, [key]: value }));
  };

  const resetCreateDialog = () => {
    setCreatedDevice(null);
    setCreateForm({
      name: "",
      sn: "",
      mac: "",
      hw_version: "",
      sw_version: "",
      config_version: "",
    });
  };

  const copyMQTTConfig = async () => {
    if (!mqttConfigText) return;
    await navigator.clipboard.writeText(mqttConfigText);
    toast.success("MQTT 配置已复制");
  };

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
          <div className="flex flex-wrap items-center gap-2">
            {canCreateDevice && (
              <Button
                onClick={() => {
                  resetCreateDialog();
                  setCreateOpen(true);
                }}
              >
                <Plus className="h-4 w-4" />
                新增 MQTT 设备
              </Button>
            )}
            <Button variant="outline" onClick={() => refetch()} disabled={isFetching}>
              <RefreshCw className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
              刷新
            </Button>
          </div>
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

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>新增 MQTT 设备</DialogTitle>
            <DialogDescription>
              创建设备后会立即生成 MQTT client_id 和 password token，复制后写入设备固件或配置文件。
            </DialogDescription>
          </DialogHeader>

          {!createdDevice ? (
            <form
              className="space-y-4"
              onSubmit={(event) => {
                event.preventDefault();
                if (canSubmitCreate) createDeviceMutation.mutate();
              }}
            >
              <div className="grid gap-3 sm:grid-cols-2">
                <label className="space-y-1.5 sm:col-span-2">
                  <span className="text-xs font-bold text-slate-500">设备名称</span>
                  <Input
                    value={createForm.name}
                    onChange={(event) => updateCreateForm("name", event.target.value)}
                    placeholder="例如 温湿度传感器 001"
                    className="h-10"
                    required
                  />
                </label>
                <label className="space-y-1.5">
                  <span className="text-xs font-bold text-slate-500">SN</span>
                  <Input
                    value={createForm.sn || ""}
                    onChange={(event) => updateCreateForm("sn", event.target.value)}
                    placeholder="设备序列号"
                    className="h-10 font-mono"
                  />
                </label>
                <label className="space-y-1.5">
                  <span className="text-xs font-bold text-slate-500">MAC</span>
                  <Input
                    value={createForm.mac || ""}
                    onChange={(event) => updateCreateForm("mac", event.target.value)}
                    placeholder="AA:BB:CC:DD:EE:FF"
                    className="h-10 font-mono"
                  />
                </label>
                <label className="space-y-1.5">
                  <span className="text-xs font-bold text-slate-500">硬件版本</span>
                  <Input
                    value={createForm.hw_version || ""}
                    onChange={(event) => updateCreateForm("hw_version", event.target.value)}
                    placeholder="v1"
                    className="h-10"
                  />
                </label>
                <label className="space-y-1.5">
                  <span className="text-xs font-bold text-slate-500">软件版本</span>
                  <Input
                    value={createForm.sw_version || ""}
                    onChange={(event) => updateCreateForm("sw_version", event.target.value)}
                    placeholder="1.0.0"
                    className="h-10"
                  />
                </label>
              </div>
              <p className="text-xs text-slate-500">SN 和 MAC 至少填一个。系统会用它们生成稳定 UUID。</p>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
                  取消
                </Button>
                <Button type="submit" disabled={!canSubmitCreate || createDeviceMutation.isPending}>
                  {createDeviceMutation.isPending ? (
                    <RefreshCw className="h-4 w-4 animate-spin" />
                  ) : (
                    <Plus className="h-4 w-4" />
                  )}
                  创建并分配令牌
                </Button>
              </DialogFooter>
            </form>
          ) : (
            <div className="space-y-4">
              <div className="rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800">
                <div className="flex items-center gap-2 font-bold">
                  <Check className="h-4 w-4" />
                  设备已创建
                </div>
                <p className="mt-1">下面这组 MQTT 参数只需要写入设备端即可连接。</p>
              </div>
              <div className="grid gap-2 rounded-xl border border-slate-200 bg-slate-50 p-4 text-sm">
                <ConfigRow label="Host" value={mqttHost} />
                <ConfigRow label="Port" value={mqttPort} />
                <ConfigRow label="Client ID" value={createdDevice.mqtt.client_id} />
                <ConfigRow label="Username" value={createdDevice.mqtt.username} />
                <ConfigRow label="Password" value={createdDevice.mqtt.password} secret />
                <ConfigRow label="Telemetry" value={createdDevice.mqtt.telemetry_topic} />
                <ConfigRow label="Heartbeat" value={createdDevice.mqtt.heartbeat_topic} />
                <ConfigRow label="Downlink" value={createdDevice.mqtt.downlink_topic} />
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
                  关闭
                </Button>
                <Button type="button" onClick={copyMQTTConfig}>
                  <Copy className="h-4 w-4" />
                  复制 MQTT 配置
                </Button>
              </DialogFooter>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

function ConfigRow({ label, value, secret = false }: { label: string; value: string; secret?: boolean }) {
  return (
    <div className="grid gap-1 sm:grid-cols-[120px_1fr] sm:items-start">
      <span className="text-xs font-black uppercase tracking-wider text-slate-400">{label}</span>
      <code className="break-all rounded bg-white px-2 py-1 font-mono text-xs font-semibold text-slate-800">
        {secret ? value : value}
      </code>
    </div>
  );
}
