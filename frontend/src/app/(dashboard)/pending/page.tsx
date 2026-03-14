"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { queryKeys } from "@/lib/query-keys";

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Bell, CheckCircle, XCircle, RefreshCw, Cpu } from "lucide-react";

type DeviceRecord = components["schemas"]["DeviceRecord"];

export default function PendingDevicesPage() {
  const { isAuthenticated, user } = useAuth();
  const queryClient = useQueryClient();

  const { data: deviceData, isLoading } = useQuery({
    queryKey: queryKeys.devicesByStatus("pending"),
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "pending" }),
    enabled: isAuthenticated && (user?.permission || 0) >= 2,
  });

  const approveMutation = useMutation({
    mutationFn: (uuid: string) => api.post(`/api/v1/devices/${uuid}/approve`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("pending") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("authenticated") });
    },
  });

  const revokeMutation = useMutation({
    mutationFn: (uuid: string) => api.post(`/api/v1/devices/${uuid}/revoke`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("pending") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("authenticated") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("refused") });
    },
  });

  if (!isAuthenticated || (user?.permission || 0) < 2) return null;

  const devices = deviceData?.items || [];

  return (
    <div className="space-y-6 fade-in animate-in slide-in-from-bottom-2">
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-4 mb-8">
        <div className="flex items-center gap-3">
          <div className="bg-amber-100 p-3 rounded-2xl text-amber-600">
            <Bell className="h-6 w-6" />
          </div>
          <div>
            <h1 className="text-2xl font-black text-slate-900 tracking-tight">待处理认证</h1>
            <p className="text-slate-500 font-medium">审批新接入系统的 IoT 设备</p>
          </div>
        </div>
        <Button 
          variant="outline" 
          className="bg-white shadow-sm border-slate-200 hover:bg-slate-50 rounded-xl"
          onClick={() => queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("pending") })}
        >
          <RefreshCw className={`h-4 w-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
          刷新列表
        </Button>
      </div>

      <Card className="border-none shadow-xl shadow-slate-200/50 bg-white overflow-hidden rounded-2xl">
        <CardContent className="p-0">
          <Table>
            <TableHeader className="bg-slate-50/50">
              <TableRow className="border-slate-100 hover:bg-transparent">
                <TableHead className="font-bold text-slate-500 pl-6 h-12">设备标识 (UUID / MAC)</TableHead>
                <TableHead className="font-bold text-slate-500 h-12">设备名称</TableHead>
                <TableHead className="font-bold text-slate-500 h-12">序列号 (SN)</TableHead>
                <TableHead className="text-right pr-6 font-bold text-slate-500 h-12">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={4} className="text-center py-20 text-slate-400">正在获取待审批设备...</TableCell>
                </TableRow>
              ) : devices.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="text-center py-20">
                    <div className="flex flex-col items-center gap-3">
                      <div className="bg-slate-50 p-4 rounded-full">
                        <CheckCircle className="h-10 w-10 text-emerald-400" />
                      </div>
                      <div className="space-y-1">
                        <p className="text-slate-900 font-bold">没有待处理的设备认证</p>
                        <p className="text-slate-400 text-sm">所有接入请求均已处理完毕</p>
                      </div>
                    </div>
                  </TableCell>
                </TableRow>
              ) : (
                devices.map((device: DeviceRecord) => (
                  <TableRow key={device.uuid} className="group border-slate-50 hover:bg-slate-50/50 transition-colors">
                    <TableCell className="pl-6 py-4">
                       <div className="font-mono text-sm font-bold text-slate-700">{device.uuid}</div>
                       <div className="font-mono text-xs text-slate-400 mt-1">{device.meta.mac}</div>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <Cpu className="h-4 w-4 text-slate-400" />
                        <span className="font-semibold text-slate-900">{device.meta.name}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <span className="bg-slate-100 text-slate-600 px-2.5 py-1 rounded-md text-xs font-mono font-bold">
                        {device.meta.sn}
                      </span>
                    </TableCell>
                    <TableCell className="pr-6 text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          size="sm"
                          className="h-9 bg-emerald-50 text-emerald-700 hover:bg-emerald-600 hover:text-white rounded-xl px-4 transition-all shadow-sm"
                          onClick={() => approveMutation.mutate(device.uuid)}
                          disabled={approveMutation.isPending || revokeMutation.isPending}
                        >
                          <CheckCircle className="h-4 w-4 mr-2" />
                          批准
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-9 text-rose-600 border-rose-200 hover:bg-rose-50 hover:border-rose-300 rounded-xl px-4 transition-all"
                          onClick={() => {
                            if (confirm("确定要拒绝该设备的接入请求吗？")) {
                              revokeMutation.mutate(device.uuid);
                            }
                          }}
                          disabled={approveMutation.isPending || revokeMutation.isPending}
                        >
                          <XCircle className="h-4 w-4 mr-2" />
                          拒绝
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
  );
}
