"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { 
  LogOut, 
  RefreshCw, 
  CheckCircle, 
  XCircle, 
  Trash2, 
  LayoutDashboard, 
  Users, 
  Activity,
  Cpu,
  ShieldCheck,
  ChevronRight
} from "lucide-react";

type DeviceRecord = components["schemas"]["DeviceRecord"];

const STATUS_MAP: Record<number, { label: string; color: string; variant: "default" | "secondary" | "destructive" | "outline" }> = {
  0: { label: "已认证", color: "bg-emerald-50 text-emerald-700 border-emerald-200", variant: "outline" },
  1: { label: "已拒绝", color: "bg-rose-50 text-rose-700 border-rose-200", variant: "outline" },
  2: { label: "待审批", color: "bg-amber-50 text-amber-700 border-amber-200", variant: "outline" },
  4: { label: "已撤销", color: "bg-slate-100 text-slate-600 border-slate-200", variant: "outline" },
};

export default function Dashboard() {
  const { user, isAuthenticated, isLoading: authLoading, logout } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [statusFilter, setStatusFilter] = useState("all");

  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      router.push("/login");
    }
  }, [isAuthenticated, authLoading, router]);

  const { data: deviceData, isLoading: devicesLoading } = useQuery({
    queryKey: ["devices", statusFilter],
    queryFn: () => api.get("/api/v1/devices", { status: statusFilter }),
    enabled: isAuthenticated,
  });

  const approveMutation = useMutation({
    mutationFn: (uuid: string) => api.post(`/api/v1/devices/${uuid}/approve`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["devices"] }),
  });

  const revokeMutation = useMutation({
    mutationFn: (uuid: string) => api.post(`/api/v1/devices/${uuid}/revoke`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["devices"] }),
  });

  const unblockMutation = useMutation({
    mutationFn: (uuid: string) => api.post(`/api/v1/devices/${uuid}/unblock`),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["devices"] }),
  });

  if (authLoading || !isAuthenticated) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-slate-50">
        <div className="flex flex-col items-center gap-4">
          <div className="h-12 w-12 rounded-full border-4 border-blue-600 border-t-transparent animate-spin"></div>
          <p className="text-slate-500 font-medium">正在加载系统...</p>
        </div>
      </div>
    );
  }

  const devices = deviceData?.items || [];

  return (
    <div className="min-h-screen bg-[#f8fafc] font-sans text-slate-900">
      {/* 顶部导航 */}
      <header className="bg-white/80 backdrop-blur-md border-b sticky top-0 z-50 border-slate-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-8">
            <div className="flex items-center gap-2">
              <div className="bg-blue-600 p-1.5 rounded-lg shadow-md shadow-blue-100">
                <ShieldCheck className="h-5 w-5 text-white" />
              </div>
              <span className="text-xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-blue-600 to-indigo-600">
                Goster IoT
              </span>
            </div>
            
            <nav className="hidden md:flex items-center gap-1">
              <Button variant="ghost" size="sm" className="bg-blue-50 text-blue-700 hover:bg-blue-100 rounded-lg px-4">
                <LayoutDashboard className="h-4 w-4 mr-2" />
                仪表盘
              </Button>
              {user?.permission === 3 && (
                <Button variant="ghost" size="sm" className="text-slate-600 hover:bg-slate-100 rounded-lg px-4" onClick={() => router.push("/users")}>
                  <Users className="h-4 w-4 mr-2" />
                  用户管理
                </Button>
              )}
            </nav>
          </div>

          <div className="flex items-center gap-4">
            <div className="hidden sm:flex flex-col items-end">
              <span className="text-sm font-bold text-slate-700">{user?.username}</span>
              <span className="text-[10px] font-bold uppercase tracking-wider text-slate-400">
                {user?.permission === 3 ? "超级管理员" : "普通用户"}
              </span>
            </div>
            <Button variant="outline" size="icon" className="rounded-full h-9 w-9 border-slate-200 hover:bg-rose-50 hover:text-rose-600 hover:border-rose-100 transition-colors" onClick={() => logout()}>
              <LogOut className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
        <div className="space-y-8">
          {/* 页面标题区 */}
          <div className="flex flex-col md:flex-row md:items-end justify-between gap-4">
            <div>
              <h1 className="text-3xl font-extrabold text-slate-900 tracking-tight">设备管理</h1>
              <p className="text-slate-500 mt-1 font-medium">监控、审批并管理接入系统的 IoT 节点</p>
            </div>
            <Button 
              variant="outline" 
              className="bg-white shadow-sm border-slate-200 hover:bg-slate-50 rounded-xl px-5 py-5"
              onClick={() => queryClient.invalidateQueries({ queryKey: ["devices"] })}
            >
              <RefreshCw className={`h-4 w-4 mr-2 ${devicesLoading ? 'animate-spin' : ''}`} />
              刷新数据
            </Button>
          </div>

          {/* 状态过滤 */}
          <Tabs defaultValue="all" onValueChange={setStatusFilter} className="w-full">
            <TabsList className="bg-slate-100 p-1 rounded-xl w-full sm:w-auto h-auto">
              <TabsTrigger value="all" className="rounded-lg py-2 px-6 data-[state=active]:bg-white data-[state=active]:shadow-sm">
                全部设备
              </TabsTrigger>
              <TabsTrigger value="authenticated" className="rounded-lg py-2 px-6 data-[state=active]:bg-white data-[state=active]:shadow-sm">
                已认证
              </TabsTrigger>
              <TabsTrigger value="pending" className="rounded-lg py-2 px-6 data-[state=active]:bg-white data-[state=active]:shadow-sm">
                待审批
              </TabsTrigger>
              <TabsTrigger value="revoked" className="rounded-lg py-2 px-6 data-[state=active]:bg-white data-[state=active]:shadow-sm">
                已撤销/拒绝
              </TabsTrigger>
            </TabsList>
          </Tabs>

          {/* 设备列表卡片 */}
          <Card className="border-none shadow-xl shadow-slate-200/50 bg-white overflow-hidden rounded-2xl">
            <CardContent className="p-0">
              <Table>
                <TableHeader className="bg-slate-50/50">
                  <TableRow className="border-slate-100 hover:bg-transparent">
                    <TableHead className="font-bold text-slate-500 pl-6 h-12">设备名称 / 序列号</TableHead>
                    <TableHead className="font-bold text-slate-500 h-12">硬件 / 固件版本</TableHead>
                    <TableHead className="font-bold text-slate-500 h-12">MAC 地址</TableHead>
                    <TableHead className="font-bold text-slate-500 h-12">认证状态</TableHead>
                    <TableHead className="font-bold text-slate-500 h-12">运行状态</TableHead>
                    <TableHead className="text-right pr-6 font-bold text-slate-500 h-12">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {devicesLoading ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center py-20">
                        <div className="flex flex-col items-center gap-2">
                          <RefreshCw className="h-8 w-8 text-blue-500 animate-spin" />
                          <p className="text-slate-400 font-medium">正在获取设备数据...</p>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : devices.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center py-20">
                        <div className="flex flex-col items-center gap-3">
                          <div className="bg-slate-50 p-4 rounded-full">
                            <Cpu className="h-10 w-10 text-slate-300" />
                          </div>
                          <div className="space-y-1">
                            <p className="text-slate-900 font-bold">暂无相关设备</p>
                            <p className="text-slate-400 text-sm">当前筛选条件下未发现任何 IoT 设备记录</p>
                          </div>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : (
                    devices.map((device: DeviceRecord) => (
                      <TableRow key={device.uuid} className="group border-slate-50 hover:bg-slate-50/50 transition-colors">
                        <TableCell className="pl-6 py-4">
                          <div className="flex items-center gap-3">
                            <div className="bg-blue-50 p-2 rounded-lg text-blue-600 group-hover:bg-blue-600 group-hover:text-white transition-all">
                              <Cpu className="h-5 w-5" />
                            </div>
                            <div>
                              <div className="font-bold text-slate-900 flex items-center gap-2">
                                {device.meta.name}
                                {device.meta.authenticate_status === 0 && (
                                  <Button
                                    variant="ghost"
                                    size="icon"
                                    className="h-6 w-6 rounded-full text-slate-400 hover:text-blue-600 hover:bg-blue-50"
                                    onClick={() => router.push(`/devices/${device.uuid}/metrics`)}
                                    title="查看运行指标"
                                  >
                                    <Activity className="h-3.5 w-3.5" />
                                  </Button>
                                )}
                              </div>
                              <div className="text-[10px] font-mono text-slate-400 uppercase tracking-tighter">{device.meta.sn}</div>
                            </div>
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className="text-sm font-semibold text-slate-700">H: {device.meta.hw_version}</div>
                          <div className="text-[11px] text-slate-400 font-medium">S: {device.meta.sw_version}</div>
                        </TableCell>
                        <TableCell className="font-mono text-xs text-slate-500">
                          {device.meta.mac}
                        </TableCell>
                        <TableCell>
                          <Badge 
                            variant={STATUS_MAP[device.meta.authenticate_status]?.variant}
                            className={`${STATUS_MAP[device.meta.authenticate_status]?.color} rounded-lg px-2.5 py-0.5 border font-semibold text-[11px] shadow-sm`}
                          >
                            {STATUS_MAP[device.meta.authenticate_status]?.label || "未知"}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center gap-2">
                            <span className={`h-2 w-2 rounded-full ${device.runtime?.status === 1 ? "bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]" : "bg-slate-300"}`}></span>
                            <span className="text-sm font-medium text-slate-600">
                              {device.runtime?.status === 1 ? "在线" : device.runtime?.status === 2 ? "延迟" : "离线"}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell className="pr-6 text-right">
                          <div className="flex justify-end gap-2">
                            {device.meta.authenticate_status === 2 && (
                              <>
                                <Button
                                  size="sm"
                                  variant="outline"
                                  className="h-8 bg-emerald-50 text-emerald-700 border-emerald-200 hover:bg-emerald-600 hover:text-white rounded-lg px-3 transition-all"
                                  onClick={() => approveMutation.mutate(device.uuid)}
                                >
                                  <CheckCircle className="h-3.5 w-3.5 mr-1.5" />
                                  批准
                                </Button>
                                <Button
                                  size="sm"
                                  variant="outline"
                                  className="h-8 bg-rose-50 text-rose-700 border-rose-200 hover:bg-rose-600 hover:text-white rounded-lg px-3 transition-all"
                                  onClick={() => revokeMutation.mutate(device.uuid)}
                                >
                                  <XCircle className="h-3.5 w-3.5 mr-1.5" />
                                  拒绝
                                </Button>
                              </>
                            )}
                            {device.meta.authenticate_status === 0 && (
                              <Button
                                size="sm"
                                variant="outline"
                                className="h-8 text-slate-500 border-slate-200 hover:bg-rose-50 hover:text-rose-600 hover:border-rose-100 rounded-lg px-3 transition-all"
                                onClick={() => revokeMutation.mutate(device.uuid)}
                              >
                                <LogOut className="h-3.5 w-3.5 mr-1.5" />
                                吊销
                              </Button>
                            )}
                            {(device.meta.authenticate_status === 1 || device.meta.authenticate_status === 4) && (
                              <Button
                                size="sm"
                                variant="outline"
                                className="h-8 bg-blue-50 text-blue-700 border-blue-200 hover:bg-blue-600 hover:text-white rounded-lg px-3 transition-all"
                                onClick={() => unblockMutation.mutate(device.uuid)}
                              >
                                <RefreshCw className="h-3.5 w-3.5 mr-1.5" />
                                重新激活
                              </Button>
                            )}
                            <Button 
                              variant="ghost" 
                              size="icon" 
                              className="h-8 w-8 rounded-lg text-slate-300 hover:bg-rose-50 hover:text-rose-500"
                              onClick={() => router.push(`/devices/${device.uuid}/metrics`)}
                            >
                              <ChevronRight className="h-4 w-4" />
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      </main>
    </div>
  );
}
