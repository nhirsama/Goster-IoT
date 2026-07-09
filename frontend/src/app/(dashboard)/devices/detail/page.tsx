"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, getApiErrorMessage } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { metricRangeOptions } from "@/lib/dashboard-meta";
import { queryKeys } from "@/lib/query-keys";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useMemo, useState } from "react";
import { useUx } from "@/components/providers/ux-provider";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import { EmptyState } from "@/components/dashboard/empty-state";
import { PageHeader } from "@/components/dashboard/page-header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { 
  Server, 
  Key, 
  Settings, 
  Ban, 
  Trash2, 
  Copy, 
  Check, 
  Cpu,
  MonitorSmartphone,
  Fingerprint,
  ShieldAlert,
  Wifi,
  Activity,
  DoorClosed,
  DoorOpen,
  SendHorizontal,
  Braces,
  Eraser,
  Wand2,
  AlertCircle,
  ChevronLeft,
  RefreshCw,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

type MetricsData = components["schemas"]["MetricsData"];
type MetricPoint = components["schemas"]["MetricPoint"];
type MetricRange = (typeof metricRangeOptions)[number]["value"];
type DownlinkCommand = "config_push" | "ota_data" | "action_exec" | "screen_wy";
type AccessSignal = 0 | 1 | null;
type AccessControlState = {
  uuid: string;
  signal_a: AccessSignal;
  signal_b: AccessSignal;
  open: boolean | null;
  evaluated_at_ms: number | null;
  status_text: string | null;
};
type DoorState = "open" | "closed" | "unknown";
type DownlinkQuickTemplate = {
  label: string;
  hint: string;
  template: string;
};
type EnqueueCommandResponse = {
  command_id: number;
  uuid: string;
  command: DownlinkCommand;
  cmd_id: number;
  status: "queued" | "sent" | "acked" | "failed";
  enqueued_at?: string | null;
};

const downlinkCommandOptions: {
  value: DownlinkCommand;
  label: string;
  hint: string;
  template: string;
  quickTemplates?: DownlinkQuickTemplate[];
}[] = [
  {
    value: "action_exec",
    label: "远程动作 (ACTION_EXEC)",
    hint: "适合重启、切换模式、触发一次性动作。",
    template: '{\n  "op": "reboot",\n  "delay_ms": 1000\n}',
    quickTemplates: [
      {
        label: "门禁校准",
        hint: "access_control_calibrate",
        template: '{\n  "op": "access_control_calibrate",\n  "duration_ms": 3000,\n  "reason": "manual_admin"\n}',
      },
      {
        label: "远程开门",
        hint: "door_unlock",
        template: '{\n  "op": "door_unlock",\n  "duration_ms": 3000,\n  "reason": "manual_admin"\n}',
      },
    ],
  },
  {
    value: "config_push",
    label: "配置下发 (CONFIG_PUSH)",
    hint: "适合更新采样间隔、阈值、运行策略等配置。",
    template: '{\n  "sampling_interval_ms": 5000,\n  "upload_batch_size": 32,\n  "enabled": true\n}',
  },
  {
    value: "ota_data",
    label: "OTA 数据 (OTA_DATA)",
    hint: "适合下发 OTA 分片元数据和分片内容。",
    template: '{\n  "job_id": "ota-001",\n  "chunk_index": 0,\n  "chunk_total": 1,\n  "data_b64": ""\n}',
  },
  {
    value: "screen_wy",
    label: "屏幕刷新 (SCREEN_WY)",
    hint: "适合更新屏幕标题、文本、提示状态。",
    template: '{\n  "title": "Goster IoT",\n  "lines": [\"online\", \"sync ok\"]\n}',
  },
];

const accessDoorMeta = {
  open: {
    label: "开门",
    description: "signal_a 与 signal_b 均为 1",
    icon: DoorOpen,
    badgeClass: "border-emerald-200 bg-emerald-50 text-emerald-700",
    panelClass: "border-emerald-200 bg-emerald-50/80 text-emerald-800",
  },
  closed: {
    label: "关门",
    description: "至少一个输入信号为 0",
    icon: DoorClosed,
    badgeClass: "border-slate-200 bg-slate-100 text-slate-700",
    panelClass: "border-slate-200 bg-slate-50 text-slate-700",
  },
  unknown: {
    label: "未知",
    description: "输入信号缺失或尚未评估",
    icon: AlertCircle,
    badgeClass: "border-amber-200 bg-amber-50 text-amber-700",
    panelClass: "border-amber-200 bg-amber-50/80 text-amber-800",
  },
} satisfies Record<
  DoorState,
  {
    label: string;
    description: string;
    icon: typeof DoorOpen;
    badgeClass: string;
    panelClass: string;
  }
>;

function parsePayloadText(raw: string): { ok: true; value: unknown; pretty: string } | { ok: false; message: string } {
  const trimmed = raw.trim();
  if (!trimmed) {
    return { ok: true, value: undefined, pretty: "" };
  }
  try {
    const value = JSON.parse(trimmed) as unknown;
    return { ok: true, value, pretty: JSON.stringify(value, null, 2) };
  } catch (error) {
    return {
      ok: false,
      message: error instanceof Error ? error.message : "payload 必须是合法 JSON",
    };
  }
}

function getAccessSignalMeta(signal?: AccessSignal) {
  if (signal === 1) {
    return {
      value: "1",
      label: "高电平",
      dotClass: "bg-emerald-500",
      panelClass: "border-emerald-200 bg-emerald-50/80 text-emerald-800",
    };
  }
  if (signal === 0) {
    return {
      value: "0",
      label: "低电平",
      dotClass: "bg-slate-400",
      panelClass: "border-slate-200 bg-slate-50 text-slate-700",
    };
  }
  return {
    value: "--",
    label: "未知",
    dotClass: "bg-amber-500",
    panelClass: "border-amber-200 bg-amber-50/80 text-amber-800",
  };
}

function getAccessDoorState(accessControl?: AccessControlState): DoorState {
  if (accessControl?.signal_a !== 0 && accessControl?.signal_a !== 1) {
    return "unknown";
  }
  if (accessControl.signal_b !== 0 && accessControl.signal_b !== 1) {
    return "unknown";
  }
  return accessControl.signal_a === 1 && accessControl.signal_b === 1 ? "open" : "closed";
}

function formatAccessEvaluatedAt(value?: number | null): string {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return "未知";
  }
  return new Date(value).toLocaleString("zh-CN", { hour12: false });
}

function formatAccessOpen(open?: boolean | null): string {
  if (open === true) return "true / 开";
  if (open === false) return "false / 关";
  return "null / 未知";
}

export default function DeviceMetricsPage() {
  return (
    <Suspense fallback={<EmptyState icon={Activity} title="正在加载设备详情" description="请稍候..." className="py-24" />}>
      <DeviceMetricsPageContent />
    </Suspense>
  );
}

function DeviceMetricsPageContent() {
  const searchParams = useSearchParams();
  const uuid = searchParams.get("uuid") || "";
  const router = useRouter();
  const queryClient = useQueryClient();
  const { user, isAuthenticated, isLoading: authLoading } = useAuth();
  const { toast, confirm: askConfirm } = useUx();
  const [range, setRange] = useState<MetricRange>("1h");
  const [copied, setCopied] = useState(false);
  const [accessAutoRefresh, setAccessAutoRefresh] = useState(false);
  const [command, setCommand] = useState<DownlinkCommand>("action_exec");
  const [payloadText, setPayloadText] = useState(downlinkCommandOptions[0].template);
  const [lastEnqueuedCommand, setLastEnqueuedCommand] = useState<EnqueueCommandResponse | null>(null);

  // 获取设备详情
  const { data: device, isLoading: deviceLoading } = useQuery({
    queryKey: queryKeys.device(uuid),
    queryFn: () => api.get<components["schemas"]["DeviceRecord"]>(`/api/v1/devices/${uuid}`),
    enabled: !!uuid && isAuthenticated,
  });

  // 获取指标数据
  const { data: metricsData } = useQuery<MetricsData>({
    queryKey: queryKeys.metrics(uuid, range),
    queryFn: () => api.get(`/api/v1/metrics/${uuid}`, { range }),
    enabled: !!uuid && isAuthenticated,
    refetchInterval: 30000,
  });

  // 获取门禁模块状态
  const {
    data: accessControl,
    isLoading: accessControlLoading,
    isFetching: accessControlFetching,
    isError: accessControlIsError,
    error: accessControlError,
    refetch: refetchAccessControl,
    dataUpdatedAt: accessControlUpdatedAt,
  } = useQuery<AccessControlState>({
    queryKey: queryKeys.accessControl(uuid),
    queryFn: () => api.get<AccessControlState>(`/api/v1/access-control/${uuid}`),
    enabled: !!uuid && isAuthenticated,
    refetchInterval: accessAutoRefresh ? 5000 : false,
  });

  // 操作 Mutations
  const refreshTokenMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/devices/${uuid}/token/refresh`),
    onSuccess: () => {
      toast.success("设备令牌已重置");
      queryClient.invalidateQueries({ queryKey: queryKeys.device(uuid) });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("authenticated") });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "重置令牌失败，请稍后重试"));
    },
  });

  const revokeMutation = useMutation({
    mutationFn: () => api.post(`/api/v1/devices/${uuid}/revoke`),
    onSuccess: () => {
      toast.success("设备已吊销");
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("authenticated") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("refused") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("revoked") });
      router.push("/");
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "吊销设备失败，请稍后重试"));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => api.delete(`/api/v1/devices/${uuid}`),
    onSuccess: () => {
      toast.success("设备已删除");
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("authenticated") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("refused") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("revoked") });
      router.push("/");
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "删除设备失败，请稍后重试"));
    },
  });

  const enqueueCommandMutation = useMutation({
    mutationFn: async () => {
      const parsed = parsePayloadText(payloadText);
      if (!parsed.ok) {
        throw new Error(parsed.message);
      }
      return api.post<EnqueueCommandResponse>(`/api/v1/devices/${uuid}/commands`, {
        command,
        payload: parsed.value,
      });
    },
    onSuccess: (result) => {
      setLastEnqueuedCommand(result);
      toast.success(`指令已入队 #${result.command_id}`);
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "指令下发失败，请稍后重试"));
    },
  });

  const selectedCommandOption = downlinkCommandOptions.find((item) => item.value === command) || downlinkCommandOptions[0];
  const parsedPayload = useMemo(() => parsePayloadText(payloadText), [payloadText]);

  const handleCopyToken = () => {
    if (device?.meta?.token) {
      navigator.clipboard.writeText(device.meta.token);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const chartData = useMemo(() => {
    if (!metricsData?.points) return [];
    type ChartRow = { ts: number; time: string; temp?: number; hum?: number; lux?: number };
    const map = new Map<number, ChartRow>();
    metricsData.points.forEach((p: MetricPoint) => {
      const date = new Date(p.ts);
      // 类似原版时间格式: MM-dd HH:mm 或 HH:mm
      const timeStr = range === '1h' || range === '6h' 
        ? date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        : `${date.getMonth() + 1}-${date.getDate()} ${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
        
      const entry = map.get(p.ts) || { ts: p.ts, time: timeStr };
      if (p.type === 1) entry.temp = p.value;
      if (p.type === 2) entry.hum = p.value;
      if (p.type === 4) entry.lux = p.value;
      map.set(p.ts, entry);
    });
    return Array.from(map.values()).sort((a, b) => a.ts - b.ts);
  }, [metricsData, range]);

  if (authLoading) {
    return <EmptyState icon={Activity} title="正在校验会话状态" description="请稍候..." className="py-24" />;
  }

  if (!isAuthenticated) {
    return <EmptyState icon={ShieldAlert} title="需要登录" description="请先登录后再访问设备详情。" className="py-24" />;
  }

  if (!uuid) {
    return <EmptyState icon={ShieldAlert} title="缺少设备 UUID" description="请从设备列表进入详情页。" className="py-24" />;
  }

  if (deviceLoading) {
    return <EmptyState icon={Activity} title="正在加载设备详情" description="请稍候..." className="py-24" />;
  }

  if (!device) {
    return <EmptyState icon={Server} title="无法加载设备信息" description="设备不存在或已被删除。" className="py-24" />;
  }

  const permission = user?.permission || 0;

  const deviceOnline = device.runtime?.status === 1;
  const deviceDelayed = device.runtime?.status === 2;
  const accessSignalAMeta = getAccessSignalMeta(accessControl?.signal_a);
  const accessSignalBMeta = getAccessSignalMeta(accessControl?.signal_b);
  const accessDoorState = getAccessDoorState(accessControl);
  const accessDoorStatus = accessDoorMeta[accessDoorState];
  const AccessDoorIcon = accessDoorStatus.icon;
  const accessControlStatusText =
    accessControl?.status_text?.trim() ||
    (accessControlIsError ? getApiErrorMessage(accessControlError, "门禁模块状态暂不可用") : accessDoorStatus.description);

  return (
    <div className="space-y-6 fade-in animate-in slide-in-from-bottom-2">
      <PageHeader
        icon={Server}
        title={device.meta.name}
        description="查看设备主档、访问令牌、下行指令和指标数据。"
        action={
          <Button variant="outline" onClick={() => router.push("/devices")}>
            <ChevronLeft className="h-4 w-4" />
            返回设备列表
          </Button>
        }
      />

      <Card className="overflow-hidden rounded-2xl border-slate-200/70 bg-white/85 shadow-lg shadow-slate-200/50">
        <CardContent className="flex flex-wrap items-center justify-between gap-4 p-6">
          <div className="flex items-center gap-4">
            <div className="bg-slate-100 p-4 rounded-full text-blue-600 shadow-inner">
              <Server className="h-8 w-8" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-2xl font-black text-slate-900 tracking-tight">{device.meta.name}</h2>
                <Badge
                  variant="outline"
                  className={
                    deviceOnline
                      ? "border-emerald-200 bg-emerald-50 text-emerald-700"
                      : deviceDelayed
                        ? "border-amber-200 bg-amber-50 text-amber-700"
                        : "border-slate-200 bg-slate-100 text-slate-600"
                  }
                >
                  <span
                    className={`h-1.5 w-1.5 rounded-full ${
                      deviceOnline ? "bg-emerald-500" : deviceDelayed ? "bg-amber-500" : "bg-slate-400"
                    }`}
                  />
                  {deviceOnline ? "在线" : deviceDelayed ? "延迟" : "离线"}
                </Badge>
              </div>
              <div className="flex items-center gap-1.5 text-slate-500 mt-1">
                <Fingerprint className="h-4 w-4" />
                <span className="font-mono text-sm">{device.uuid}</span>
              </div>
            </div>
          </div>

          {permission >= 2 && (
            <div className="flex items-center gap-3">
              <Button 
                variant="outline" 
                className="border-blue-200 text-blue-700 hover:bg-blue-50 shadow-sm font-bold"
                onClick={async () => {
                  const ok = await askConfirm({
                    title: "重置设备令牌",
                    description: "确定要重置该设备的 Token 吗？这将导致设备断开连接。",
                    confirmText: "确认重置",
                    cancelText: "取消",
                    tone: "danger",
                  });
                  if (ok) {
                    refreshTokenMutation.mutate();
                  } 
                }}
              >
                <Key className="h-4 w-4 mr-2" />
                重置令牌
              </Button>

              <DropdownMenu>
                <DropdownMenuTrigger>
                  <Button variant="outline" size="icon" className="bg-slate-50 border-slate-200 text-slate-600 hover:bg-slate-100" asChild>
                    <div>
                      <Settings className="h-4 w-4" />
                    </div>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48">
                  <DropdownMenuGroup>
                    <DropdownMenuLabel className="text-xs font-black text-slate-400 uppercase tracking-widest">
                      危险操作
                    </DropdownMenuLabel>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      className="text-rose-600 focus:text-rose-700 focus:bg-rose-50 cursor-pointer font-bold"
                      onClick={async () => {
                        const ok = await askConfirm({
                          title: "吊销设备认证",
                          description: "确定要吊销该设备的认证吗？设备将无法连接。",
                          confirmText: "确认吊销",
                          cancelText: "取消",
                          tone: "danger",
                        });
                        if (ok) {
                          revokeMutation.mutate();
                        }
                      }}
                    >
                      <Ban className="h-4 w-4 mr-2" />
                      吊销认证
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      className="text-rose-600 focus:text-rose-700 focus:bg-rose-50 cursor-pointer font-bold"
                      onClick={async () => {
                        const ok = await askConfirm({
                          title: "删除设备",
                          description: "确定要永久删除该设备吗？所有历史数据将丢失且无法恢复。",
                          confirmText: "确认删除",
                          cancelText: "取消",
                          tone: "danger",
                        });
                        if (ok) {
                          deleteMutation.mutate();
                        }
                      }}
                    >
                      <Trash2 className="h-4 w-4 mr-2" />
                      删除设备
                    </DropdownMenuItem>
                  </DropdownMenuGroup>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Info Grid - 1:1复刻原版的四宫格 */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          { label: "硬件版本", value: device.meta.hw_version, icon: Cpu },
          { label: "软件版本", value: device.meta.sw_version, icon: MonitorSmartphone },
          { label: "序列号", value: device.meta.sn, icon: Fingerprint, mono: true },
          { label: "MAC 地址", value: device.meta.mac, icon: Wifi, mono: true },
        ].map((item, i) => (
          <Card key={i} className="border-none shadow-sm bg-white overflow-hidden group">
            <CardContent className="p-4 relative">
              <item.icon className="absolute right-[-10px] bottom-[-10px] h-16 w-16 text-slate-50 opacity-50 group-hover:scale-110 group-hover:-rotate-6 transition-transform" />
              <p className="text-[10px] font-black text-slate-400 uppercase tracking-wider mb-1">{item.label}</p>
              <p className={`text-base font-bold text-slate-800 truncate ${item.mono ? 'font-mono tracking-tight' : ''}`} title={item.value}>
                {item.value}
              </p>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Token Card */}
      <Card className="border-none shadow-sm bg-white">
        <CardContent className="p-4 flex flex-col sm:flex-row justify-between sm:items-center gap-3">
          <span className="text-[11px] font-black text-slate-400 uppercase tracking-widest shrink-0">当前访问令牌</span>
          <div className="bg-slate-100 p-2.5 rounded-lg flex items-center justify-between gap-4 flex-1 overflow-hidden border border-slate-200">
            {permission >= 2 ? (
              <>
                <code className="text-sm font-bold text-slate-800 break-all">{device.meta.token}</code>
                <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0 text-slate-500 hover:text-blue-600 hover:bg-white shadow-sm transition-all" onClick={handleCopyToken}>
                  {copied ? <Check className="h-4 w-4 text-emerald-500" /> : <Copy className="h-4 w-4" />}
                </Button>
              </>
            ) : (
              <code className="text-sm font-bold text-slate-400">******** (权限不足)</code>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Access Control Card */}
      <Card className="border-none shadow-sm bg-white">
        <CardHeader className="pb-3">
          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex items-start gap-3">
              <div className="rounded-xl bg-blue-50 p-2.5 text-blue-600">
                <AccessDoorIcon className="h-5 w-5" />
              </div>
              <div>
                <div className="flex flex-wrap items-center gap-2">
                  <CardTitle className="text-base font-bold text-slate-900">门禁模块</CardTitle>
                  <Badge variant="outline" className={accessDoorStatus.badgeClass}>
                    {accessDoorStatus.label}
                  </Badge>
                  {accessAutoRefresh ? (
                    <Badge variant="outline" className="border-blue-200 bg-blue-50 text-blue-700">
                      5s 自动刷新
                    </Badge>
                  ) : null}
                </div>
                <p className="mt-1 text-xs text-slate-500">
                  按 signal_a / signal_b 判定：两个信号均为 1 时开门，否则关门；未知信号显示未知。
                </p>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={accessControlFetching}
                onClick={() => refetchAccessControl()}
              >
                <RefreshCw className={`h-4 w-4 ${accessControlFetching ? "animate-spin" : ""}`} />
                {accessControlFetching ? "刷新中" : "刷新"}
              </Button>
              <Button
                variant={accessAutoRefresh ? "secondary" : "outline"}
                size="sm"
                onClick={() => setAccessAutoRefresh((value) => !value)}
              >
                自动刷新：{accessAutoRefresh ? "开" : "关"}
              </Button>
            </div>
          </div>
        </CardHeader>

        <CardContent className="space-y-4">
          {accessControlLoading && !accessControl ? (
            <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-500">
              <RefreshCw className="h-4 w-4 animate-spin" />
              正在读取门禁模块状态...
            </div>
          ) : null}

          <div className="grid gap-3 md:grid-cols-3">
            {[
              { title: "输入信号 A", meta: accessSignalAMeta },
              { title: "输入信号 B", meta: accessSignalBMeta },
            ].map((item) => (
              <div key={item.title} className={`rounded-xl border p-4 ${item.meta.panelClass}`}>
                <p className="text-[10px] font-black uppercase tracking-widest opacity-70">{item.title}</p>
                <div className="mt-3 flex items-center justify-between">
                  <span className="text-2xl font-black font-mono">{item.meta.value}</span>
                  <span className="inline-flex items-center gap-2 text-sm font-bold">
                    <span className={`h-2.5 w-2.5 rounded-full ${item.meta.dotClass}`} />
                    {item.meta.label}
                  </span>
                </div>
              </div>
            ))}

            <div className={`rounded-xl border p-4 ${accessDoorStatus.panelClass}`}>
              <p className="text-[10px] font-black uppercase tracking-widest opacity-70">门状态</p>
              <div className="mt-3 flex items-center justify-between">
                <span className="text-2xl font-black">{accessDoorStatus.label}</span>
                <AccessDoorIcon className="h-7 w-7" />
              </div>
              <p className="mt-2 text-xs font-semibold opacity-75">{accessDoorStatus.description}</p>
            </div>
          </div>

          <div className="grid gap-2 rounded-xl border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600 sm:grid-cols-3">
            <div>
              <p className="font-black uppercase tracking-wider text-slate-400">评估时间</p>
              <p className="mt-1 font-semibold">{formatAccessEvaluatedAt(accessControl?.evaluated_at_ms)}</p>
            </div>
            <div>
              <p className="font-black uppercase tracking-wider text-slate-400">后端 open</p>
              <p className="mt-1 font-mono font-semibold">{formatAccessOpen(accessControl?.open)}</p>
            </div>
            <div>
              <p className="font-black uppercase tracking-wider text-slate-400">前端更新时间</p>
              <p className="mt-1 font-semibold">
                {accessControlUpdatedAt
                  ? new Date(accessControlUpdatedAt).toLocaleTimeString("zh-CN", { hour12: false })
                  : "尚未刷新"}
              </p>
            </div>
          </div>

          <div
            className={`flex items-start gap-2 rounded-xl border px-3 py-2 text-xs ${
              accessControlIsError
                ? "border-rose-200 bg-rose-50 text-rose-700"
                : "border-blue-100 bg-blue-50/70 text-blue-700"
            }`}
          >
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
            <span>{accessControlStatusText}</span>
          </div>
        </CardContent>
      </Card>

      {permission >= 2 && (
        <Card className="border-none shadow-sm bg-white">
          <CardHeader className="pb-3">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <CardTitle className="text-base font-bold text-slate-900">云端指令下发</CardTitle>
                <p className="mt-1 text-xs text-slate-500">指令会进入设备下行队列，设备下一次保持会话或上线时由网关发送。</p>
              </div>
              <div className="flex items-center gap-2">
                <span className={`h-2.5 w-2.5 rounded-full ${parsedPayload.ok ? "bg-emerald-500" : "bg-rose-500"}`} />
                <span className="text-xs font-semibold text-slate-500">{parsedPayload.ok ? "JSON 有效" : "JSON 无效"}</span>
              </div>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="grid gap-3 md:grid-cols-[1fr_1.6fr]">
              <label className="space-y-1">
                <span className="text-xs font-black text-slate-400 uppercase tracking-widest">指令类型</span>
                <select
                  value={command}
                  onChange={(event) => {
                    const nextCommand = event.target.value as DownlinkCommand;
                    const option = downlinkCommandOptions.find((item) => item.value === nextCommand);
                    setCommand(nextCommand);
                    setPayloadText(option?.template || "");
                    setLastEnqueuedCommand(null);
                  }}
                  className="h-9 w-full rounded-lg border border-slate-200 bg-white px-2.5 text-sm font-semibold text-slate-700 outline-none transition-colors focus:border-blue-300 focus:ring-3 focus:ring-blue-100"
                >
                  {downlinkCommandOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
                <span className="block text-xs text-slate-500">{selectedCommandOption.hint}</span>
              </label>

              <label className="space-y-1">
                <span className="text-xs font-black text-slate-400 uppercase tracking-widest">Payload (JSON)</span>
                <textarea
                  value={payloadText}
                  onChange={(event) => setPayloadText(event.target.value)}
                  className={`min-h-[150px] w-full rounded-lg border bg-white p-3 text-xs font-mono text-slate-700 outline-none transition-colors focus:ring-3 ${
                    parsedPayload.ok
                      ? "border-slate-200 focus:border-blue-300 focus:ring-blue-100"
                      : "border-rose-200 focus:border-rose-300 focus:ring-rose-100"
                  }`}
                  placeholder='{"op":"reboot"}'
                />
              </label>
            </div>

            {selectedCommandOption.quickTemplates?.length ? (
              <div className="rounded-lg border border-blue-100 bg-blue-50/60 p-3">
                <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <p className="text-xs font-black text-blue-700 uppercase tracking-widest">门禁动作模板</p>
                    <p className="mt-0.5 text-xs text-blue-600">快速填充 ACTION_EXEC payload，可继续编辑后入队。</p>
                  </div>
                </div>
                <div className="flex flex-wrap gap-2">
                  {selectedCommandOption.quickTemplates.map((template) => (
                    <Button
                      key={template.hint}
                      variant="outline"
                      size="sm"
                      className="border-blue-200 bg-white text-blue-700 hover:bg-blue-50"
                      title={template.hint}
                      onClick={() => setPayloadText(template.template)}
                    >
                      <Wand2 className="h-4 w-4" />
                      {template.label}
                    </Button>
                  ))}
                </div>
              </div>
            ) : null}

            {!parsedPayload.ok ? (
              <div className="flex items-start gap-2 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-700">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                <span>{parsedPayload.message}</span>
              </div>
            ) : null}

            <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-slate-200 bg-slate-50 p-3">
              <div className="flex flex-wrap gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPayloadText(selectedCommandOption.template)}
                >
                  <Wand2 className="h-4 w-4" />
                  使用模板
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={!parsedPayload.ok || !payloadText.trim()}
                  onClick={() => {
                    if (parsedPayload.ok) {
                      setPayloadText(parsedPayload.pretty);
                    }
                  }}
                >
                  <Braces className="h-4 w-4" />
                  格式化
                </Button>
                <Button variant="outline" size="sm" onClick={() => setPayloadText("")}>
                  <Eraser className="h-4 w-4" />
                  清空
                </Button>
              </div>

              <Button
                className="h-9 bg-blue-600 text-white hover:bg-blue-700"
                disabled={enqueueCommandMutation.isPending || !parsedPayload.ok}
                onClick={() => enqueueCommandMutation.mutate()}
              >
                <SendHorizontal className="h-4 w-4 mr-2" />
                {enqueueCommandMutation.isPending ? "入队中..." : "加入下行队列"}
              </Button>
            </div>

            {lastEnqueuedCommand ? (
              <div className="grid gap-2 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-800 sm:grid-cols-4">
                <div>
                  <p className="font-black uppercase tracking-wider text-emerald-600">Command ID</p>
                  <p className="mt-1 font-mono font-semibold">#{lastEnqueuedCommand.command_id}</p>
                </div>
                <div>
                  <p className="font-black uppercase tracking-wider text-emerald-600">CmdID</p>
                  <p className="mt-1 font-mono font-semibold">0x{lastEnqueuedCommand.cmd_id.toString(16).padStart(4, "0")}</p>
                </div>
                <div>
                  <p className="font-black uppercase tracking-wider text-emerald-600">状态</p>
                  <p className="mt-1 font-semibold">{lastEnqueuedCommand.status}</p>
                </div>
                <div>
                  <p className="font-black uppercase tracking-wider text-emerald-600">入队时间</p>
                  <p className="mt-1 font-semibold">
                    {lastEnqueuedCommand.enqueued_at
                      ? new Date(lastEnqueuedCommand.enqueued_at).toLocaleTimeString("zh-CN", { hour12: false })
                      : "-"}
                  </p>
                </div>
              </div>
            ) : null}
          </CardContent>
        </Card>
      )}

      {/* Chart Container - 1:1复刻原版的三曲线单表 */}
      <Card className="border-none shadow-xl shadow-slate-200/50 bg-white">
        <CardHeader className="border-b border-slate-100 flex flex-row items-center justify-between pb-4">
          <div className="flex items-center gap-2">
            <Activity className="h-5 w-5 text-blue-600" />
            <CardTitle className="text-lg font-bold text-slate-900">数据监控</CardTitle>
          </div>
          
            <div className="flex bg-slate-100 p-1 rounded-lg">
            {metricRangeOptions.map((r) => (
              <button
                key={r.value}
                onClick={() => setRange(r.value)}
                className={`px-3 py-1.5 text-xs font-bold rounded-md transition-all ${range === r.value ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-500 hover:text-slate-700'}`}
              >
                {r.label}
              </button>
            ))}
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {!metricsData?.points || metricsData.points.length === 0 ? (
            <div className="h-[350px] flex flex-col items-center justify-center text-slate-400 bg-slate-50/50">
              <Activity className="h-12 w-12 text-slate-200 mb-3" />
              <p className="font-medium">该时间段内暂无数据</p>
            </div>
          ) : (
            <div className="h-[400px] w-full p-4 pt-6">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#f1f5f9" />
                  <XAxis dataKey="time" axisLine={false} tickLine={false} tick={{fill: '#94a3b8', fontSize: 11}} minTickGap={30} />
                  
                  {/* YAxis 1: 温湿度 */}
                  <YAxis yAxisId="y" orientation="left" axisLine={false} tickLine={false} tick={{fill: '#64748b', fontSize: 11}} />
                  {/* YAxis 2: 光照 */}
                  <YAxis yAxisId="y1" orientation="right" axisLine={false} tickLine={false} tick={{fill: '#f59e0b', fontSize: 11}} />
                  
                  <Tooltip 
                    contentStyle={{borderRadius: '8px', border: '1px solid #e2e8f0', boxShadow: '0 10px 15px -3px rgba(0,0,0,0.1)'}}
                    labelStyle={{fontWeight: 'bold', color: '#0f172a', marginBottom: '4px'}}
                  />
                  <Legend verticalAlign="top" height={36} iconType="circle" wrapperStyle={{fontSize: '12px', fontWeight: 'bold'}} />
                  
                  {/* 完全复原原版配色的三条线 */}
                  <Line yAxisId="y" type="monotone" dataKey="temp" name="温度 (°C)" stroke="#ef4444" strokeWidth={2.5} dot={false} activeDot={{r: 6, strokeWidth: 0}} connectNulls />
                  <Line yAxisId="y" type="monotone" dataKey="hum" name="湿度 (%)" stroke="#3b82f6" strokeWidth={2.5} dot={false} activeDot={{r: 6, strokeWidth: 0}} connectNulls />
                  <Line yAxisId="y1" type="monotone" dataKey="lux" name="光照 (Lux)" stroke="#f59e0b" strokeWidth={2.5} dot={false} activeDot={{r: 6, strokeWidth: 0}} connectNulls />
                </LineChart>
              </ResponsiveContainer>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
