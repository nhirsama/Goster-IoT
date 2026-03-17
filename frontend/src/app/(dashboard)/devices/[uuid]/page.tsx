"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, getApiErrorMessage } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { metricRangeOptions } from "@/lib/dashboard-meta";
import { queryKeys } from "@/lib/query-keys";
import { useParams, useRouter } from "next/navigation";
import { useState, useMemo } from "react";
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
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
  SendHorizontal
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

type MetricsData = components["schemas"]["MetricsData"];
type MetricPoint = components["schemas"]["MetricPoint"];
type MetricRange = (typeof metricRangeOptions)[number]["value"];
type DownlinkCommand = "config_push" | "ota_data" | "action_exec" | "screen_wy";
type EnqueueCommandResponse = {
  command_id: number;
  uuid: string;
  command: DownlinkCommand;
  cmd_id: number;
  status: "queued" | "sent" | "acked" | "failed";
  enqueued_at?: string | null;
};

const downlinkCommandOptions: { value: DownlinkCommand; label: string; hint: string }[] = [
  { value: "action_exec", label: "远程动作 (ACTION_EXEC)", hint: "例如重启、切换工作模式等动作指令" },
  { value: "config_push", label: "配置下发 (CONFIG_PUSH)", hint: "下发设备运行配置" },
  { value: "ota_data", label: "OTA 数据 (OTA_DATA)", hint: "下发 OTA 数据分片" },
  { value: "screen_wy", label: "屏幕刷新 (SCREEN_WY)", hint: "下发屏幕渲染数据" },
];

export default function DeviceMetricsPage() {
  const { uuid } = useParams<{ uuid: string }>();
  const router = useRouter();
  const queryClient = useQueryClient();
  const { user, isAuthenticated, isLoading: authLoading } = useAuth();
  const { toast, confirm: askConfirm } = useUx();
  const [range, setRange] = useState<MetricRange>("1h");
  const [copied, setCopied] = useState(false);
  const [command, setCommand] = useState<DownlinkCommand>("action_exec");
  const [payloadText, setPayloadText] = useState('{"op":"reboot"}');
  const [lastEnqueuedCommandId, setLastEnqueuedCommandId] = useState<number | null>(null);

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
      const raw = payloadText.trim();
      let payload: unknown = undefined;
      if (raw.length > 0) {
        try {
          payload = JSON.parse(raw);
        } catch {
          throw new Error("payload 必须是合法 JSON");
        }
      }
      return api.post<EnqueueCommandResponse>(`/api/v1/devices/${uuid}/commands`, {
        command,
        payload,
      });
    },
    onSuccess: (result) => {
      setLastEnqueuedCommandId(result.command_id);
      toast.success(`指令已入队 #${result.command_id}`);
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "指令下发失败，请稍后重试"));
    },
  });

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

  if (deviceLoading) {
    return <EmptyState icon={Activity} title="正在加载设备详情" description="请稍候..." className="py-24" />;
  }

  if (!device) {
    return <EmptyState icon={Server} title="无法加载设备信息" description="设备不存在或已被删除。" className="py-24" />;
  }

  const permission = user?.permission || 0;
  const selectedCommandOption = downlinkCommandOptions.find((item) => item.value === command) || downlinkCommandOptions[0];

  return (
    <div className="space-y-6 fade-in animate-in slide-in-from-bottom-2 max-w-6xl mx-auto">
      {/* Header Card - 1:1复刻原版顶部操作区 */}
      <Card className="border-none shadow-lg shadow-slate-200/50 rounded-2xl overflow-hidden bg-white">
        <CardContent className="flex flex-wrap items-center justify-between gap-4 p-6">
          <div className="flex items-center gap-4">
            <div className="bg-slate-100 p-4 rounded-full text-blue-600 shadow-inner">
              <Server className="h-8 w-8" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-2xl font-black text-slate-900 tracking-tight">{device.meta.name}</h2>
                <span className={`h-2.5 w-2.5 rounded-full ${device.runtime?.status === 1 ? 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]' : device.runtime?.status === 2 ? 'bg-amber-500' : 'bg-slate-300'}`}></span>
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
                  <DropdownMenuLabel className="text-xs font-black text-slate-400 uppercase tracking-widest">危险操作</DropdownMenuLabel>
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

      {permission >= 2 && (
        <Card className="border-none shadow-sm bg-white">
          <CardHeader className="pb-3">
            <CardTitle className="text-base font-bold text-slate-900">云端指令下发</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="grid gap-3 md:grid-cols-[1.2fr_2fr_auto] md:items-end">
              <label className="space-y-1">
                <span className="text-xs font-black text-slate-400 uppercase tracking-widest">指令类型</span>
                <select
                  value={command}
                  onChange={(event) => setCommand(event.target.value as DownlinkCommand)}
                  className="h-9 w-full rounded-lg border border-slate-200 bg-white px-2.5 text-sm font-semibold text-slate-700 outline-none transition-colors focus:border-blue-300 focus:ring-3 focus:ring-blue-100"
                >
                  {downlinkCommandOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>

              <label className="space-y-1">
                <span className="text-xs font-black text-slate-400 uppercase tracking-widest">Payload (JSON)</span>
                <textarea
                  value={payloadText}
                  onChange={(event) => setPayloadText(event.target.value)}
                  className="min-h-[88px] w-full rounded-lg border border-slate-200 bg-white p-2.5 text-xs font-mono text-slate-700 outline-none transition-colors focus:border-blue-300 focus:ring-3 focus:ring-blue-100"
                  placeholder='{"op":"reboot"}'
                />
              </label>

              <Button
                className="h-9 w-full md:w-auto bg-blue-600 text-white hover:bg-blue-700"
                disabled={enqueueCommandMutation.isPending}
                onClick={() => enqueueCommandMutation.mutate()}
              >
                <SendHorizontal className="h-4 w-4 mr-2" />
                {enqueueCommandMutation.isPending ? "发送中..." : "发送指令"}
              </Button>
            </div>

            <p className="text-xs text-slate-500">{selectedCommandOption.hint}</p>
            {lastEnqueuedCommandId ? (
              <p className="text-xs font-semibold text-emerald-700">
                最近一次入队成功：command_id #{lastEnqueuedCommandId}
              </p>
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
