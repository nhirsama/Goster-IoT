"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { useParams, useRouter } from "next/navigation";
import { useState, useMemo } from "react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  AreaChart,
  Area,
} from "recharts";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ChevronLeft, RefreshCw, Thermometer, Droplets, Sun, Calendar } from "lucide-react";

type MetricsData = components["schemas"]["MetricsData"];
type MetricPoint = components["schemas"]["MetricPoint"];

export default function DeviceMetricsPage() {
  const { uuid } = useParams<{ uuid: string }>();
  const router = useRouter();
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const [range, setRange] = useState("1h");

  const { data: device } = useQuery({
    queryKey: ["device", uuid],
    queryFn: () => api.get(`/api/v1/devices/${uuid}`),
    enabled: !!uuid && isAuthenticated,
  });

  const { data: metricsData, isLoading: metricsLoading, refetch } = useQuery<MetricsData>({
    queryKey: ["metrics", uuid, range],
    queryFn: () => api.get(`/api/v1/metrics/${uuid}`, { range }),
    enabled: !!uuid && isAuthenticated,
    refetchInterval: 30000,
  });

  const chartData = useMemo(() => {
    if (!metricsData?.points) return [];
    const map = new Map<number, any>();
    metricsData.points.forEach((p: MetricPoint) => {
      const entry = map.get(p.ts) || { 
        ts: p.ts, 
        time: new Date(p.ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) 
      };
      if (p.type === 1) entry.temp = p.value;
      if (p.type === 2) entry.hum = p.value;
      if (p.type === 4) entry.lux = p.value;
      map.set(p.ts, entry);
    });
    return Array.from(map.values()).sort((a, b) => a.ts - b.ts);
  }, [metricsData]);

  if (authLoading || !isAuthenticated) return null;

  return (
    <div className="min-h-screen bg-[#f8fafc] pb-12 font-sans">
      <header className="bg-white/80 backdrop-blur-md border-b sticky top-0 z-50 border-slate-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <Button variant="ghost" size="sm" className="rounded-xl hover:bg-slate-100" onClick={() => router.back()}>
              <ChevronLeft className="h-4 w-4 mr-1" />
              返回
            </Button>
            <div className="h-4 w-px bg-slate-200 mx-2"></div>
            <h1 className="text-lg font-bold text-slate-900">设备运行指标</h1>
          </div>
          <Button variant="outline" size="sm" className="rounded-xl border-slate-200" onClick={() => refetch()}>
            <RefreshCw className={`h-4 w-4 mr-2 ${metricsLoading ? 'animate-spin' : ''}`} />
            刷新
          </Button>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-8">
        {device && (
          <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-6 bg-white p-6 rounded-2xl shadow-sm border border-slate-100">
            <div className="space-y-1">
              <div className="flex items-center gap-3">
                <h2 className="text-2xl font-black text-slate-900 tracking-tight">{device.meta.name}</h2>
                <Badge className={device.runtime?.status === 1 ? "bg-emerald-50 text-emerald-700 border-emerald-200" : "bg-slate-100 text-slate-500 border-slate-200"} variant="outline">
                   <span className={`h-1.5 w-1.5 rounded-full mr-2 ${device.runtime?.status === 1 ? "bg-emerald-500" : "bg-slate-400"}`}></span>
                  {device.runtime?.status_text === "online" ? "在线" : "离线"}
                </Badge>
              </div>
              <p className="text-sm font-medium text-slate-400 font-mono uppercase tracking-widest">
                SN: {device.meta.sn} | MAC: {device.meta.mac}
              </p>
            </div>
            <div className="flex gap-3">
              <div className="bg-slate-50 px-4 py-2 rounded-xl border border-slate-100">
                <p className="text-[10px] font-bold text-slate-400 uppercase tracking-tighter">硬件版本</p>
                <p className="text-sm font-bold text-slate-700">{device.meta.hw_version}</p>
              </div>
              <div className="bg-slate-50 px-4 py-2 rounded-xl border border-slate-100">
                <p className="text-[10px] font-bold text-slate-400 uppercase tracking-tighter">固件版本</p>
                <p className="text-sm font-bold text-slate-700">{device.meta.sw_version}</p>
              </div>
            </div>
          </div>
        )}

        <div className="space-y-6">
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
            <div className="flex items-center gap-2 text-slate-900 font-bold">
              <Calendar className="h-5 w-5 text-blue-600" />
              <h3>历史趋势分析</h3>
            </div>
            <Tabs defaultValue="1h" onValueChange={setRange} className="bg-slate-100 p-1 rounded-xl">
              <TabsList className="bg-transparent h-8">
                <TabsTrigger value="1h" className="rounded-lg data-[state=active]:bg-white data-[state=active]:shadow-sm px-4">最近 1 小时</TabsTrigger>
                <TabsTrigger value="6h" className="rounded-lg data-[state=active]:bg-white data-[state=active]:shadow-sm px-4">6 小时</TabsTrigger>
                <TabsTrigger value="24h" className="rounded-lg data-[state=active]:bg-white data-[state=active]:shadow-sm px-4">24 小时</TabsTrigger>
                <TabsTrigger value="7d" className="rounded-lg data-[state=active]:bg-white data-[state=active]:shadow-sm px-4">7 天</TabsTrigger>
              </TabsList>
            </Tabs>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
            <Card className="border-none shadow-xl shadow-slate-200/50 rounded-2xl overflow-hidden bg-white">
              <CardHeader className="border-b border-slate-50 pb-4">
                <div className="flex items-center gap-2">
                  <div className="p-2 bg-red-50 rounded-lg text-red-500">
                    <Thermometer className="h-5 w-5" />
                  </div>
                  <CardTitle className="text-lg font-bold">温度与湿度</CardTitle>
                </div>
                <CardDescription>设备实时回传的温湿度曲线</CardDescription>
              </CardHeader>
              <CardContent className="pt-8">
                <div className="h-[350px] w-full">
                  <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={chartData}>
                      <defs>
                        <linearGradient id="colorTemp" x1="0" y1="0" x2="0" y2="1">
                          <stop offset="5%" stopColor="#ef4444" stopOpacity={0.1}/>
                          <stop offset="95%" stopColor="#ef4444" stopOpacity={0}/>
                        </linearGradient>
                        <linearGradient id="colorHum" x1="0" y1="0" x2="0" y2="1">
                          <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.1}/>
                          <stop offset="95%" stopColor="#3b82f6" stopOpacity={0}/>
                        </linearGradient>
                      </defs>
                      <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#f1f5f9" />
                      <XAxis dataKey="time" axisLine={false} tickLine={false} tick={{fill: '#94a3b8', fontSize: 12}} />
                      <YAxis yAxisId="left" axisLine={false} tickLine={false} tick={{fill: '#ef4444', fontSize: 12}} />
                      <YAxis yAxisId="right" orientation="right" axisLine={false} tickLine={false} tick={{fill: '#3b82f6', fontSize: 12}} />
                      <Tooltip 
                        contentStyle={{borderRadius: '12px', border: 'none', boxShadow: '0 10px 15px -3px rgba(0,0,0,0.1)'}}
                      />
                      <Legend verticalAlign="top" height={36}/>
                      <Area 
                        yAxisId="left"
                        type="monotone" 
                        dataKey="temp" 
                        name="温度 (°C)" 
                        stroke="#ef4444" 
                        fillOpacity={1} 
                        fill="url(#colorTemp)" 
                        strokeWidth={3}
                        connectNulls
                      />
                      <Area 
                        yAxisId="right"
                        type="monotone" 
                        dataKey="hum" 
                        name="湿度 (%)" 
                        stroke="#3b82f6" 
                        fillOpacity={1} 
                        fill="url(#colorHum)" 
                        strokeWidth={3}
                        connectNulls
                      />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>

            <Card className="border-none shadow-xl shadow-slate-200/50 rounded-2xl overflow-hidden bg-white">
              <CardHeader className="border-b border-slate-50 pb-4">
                <div className="flex items-center gap-2">
                  <div className="p-2 bg-amber-50 rounded-lg text-amber-500">
                    <Sun className="h-5 w-5" />
                  </div>
                  <CardTitle className="text-lg font-bold">光照强度 (Lux)</CardTitle>
                </div>
                <CardDescription>环境光敏传感器数据趋势</CardDescription>
              </CardHeader>
              <CardContent className="pt-8">
                <div className="h-[350px] w-full">
                  <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={chartData}>
                      <defs>
                        <linearGradient id="colorLux" x1="0" y1="0" x2="0" y2="1">
                          <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.1}/>
                          <stop offset="95%" stopColor="#f59e0b" stopOpacity={0}/>
                        </linearGradient>
                      </defs>
                      <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#f1f5f9" />
                      <XAxis dataKey="time" axisLine={false} tickLine={false} tick={{fill: '#94a3b8', fontSize: 12}} />
                      <YAxis axisLine={false} tickLine={false} tick={{fill: '#f59e0b', fontSize: 12}} />
                      <Tooltip 
                        contentStyle={{borderRadius: '12px', border: 'none', boxShadow: '0 10px 15px -3px rgba(0,0,0,0.1)'}}
                      />
                      <Area 
                        type="monotone" 
                        dataKey="lux" 
                        name="光照值 (Lux)" 
                        stroke="#f59e0b" 
                        fillOpacity={1} 
                        fill="url(#colorLux)" 
                        strokeWidth={3}
                        connectNulls
                      />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      </main>
    </div>
  );
}
