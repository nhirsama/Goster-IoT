"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, getApiErrorMessage } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { queryKeys } from "@/lib/query-keys";
import { useUx } from "@/components/providers/ux-provider";

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
import { Ban, RefreshCw, Cpu, ShieldAlert } from "lucide-react";
import { Badge } from "@/components/ui/badge";

type DeviceRecord = components["schemas"]["DeviceRecord"];

export default function BlacklistPage() {
  const { isAuthenticated, user } = useAuth();
  const queryClient = useQueryClient();
  const { toast, confirm: askConfirm } = useUx();

  // 同时获取 refused (1) 和 revoked (4) 状态的设备，这里我们在前端过滤或者如果后端支持多状态查询
  // 原系统通过不同的 query/view 或者复用接口实现。我们这里直接请求 revoked 状态以获取被撤销/拒绝的设备
  const { data: revokedData, isLoading: revokedLoading } = useQuery({
    queryKey: queryKeys.devicesByStatus("revoked"),
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "revoked" }),
    enabled: isAuthenticated && (user?.permission || 0) >= 1,
  });

  const { data: refusedData, isLoading: refusedLoading } = useQuery({
    queryKey: queryKeys.devicesByStatus("refused"),
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "refused" }),
    enabled: isAuthenticated && (user?.permission || 0) >= 1,
  });

  const unblockMutation = useMutation({
    mutationFn: (uuid: string) => api.post(`/api/v1/devices/${uuid}/unblock`),
    onSuccess: () => {
      toast.success("设备已移出黑名单");
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("revoked") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("refused") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("pending") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("authenticated") });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "解除屏蔽失败，请稍后重试"));
    },
  });

  if (!isAuthenticated || (user?.permission || 0) < 1) return null;

  const isLoading = revokedLoading || refusedLoading;
  const merged = [...(revokedData?.items || []), ...(refusedData?.items || [])];
  const deviceMap = new Map<string, DeviceRecord>();
  merged.forEach((device: DeviceRecord) => {
    deviceMap.set(device.uuid, device);
  });
  const devices = Array.from(deviceMap.values());

  return (
    <div className="space-y-6 fade-in animate-in slide-in-from-bottom-2">
      <div className="flex flex-col md:flex-row md:items-end justify-between gap-4 mb-8">
        <div className="flex items-center gap-3">
          <div className="bg-rose-100 p-3 rounded-2xl text-rose-600">
            <Ban className="h-6 w-6" />
          </div>
          <div>
            <h1 className="text-2xl font-black text-slate-900 tracking-tight">设备黑名单</h1>
            <p className="text-slate-500 font-medium">被拒绝认证或已吊销的设备列表</p>
          </div>
        </div>
        <Button 
          variant="outline" 
          className="bg-white shadow-sm border-slate-200 hover:bg-slate-50 rounded-xl"
          onClick={() => {
            queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("revoked") });
            queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("refused") });
          }}
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
                <TableHead className="font-bold text-slate-500 h-12">状态</TableHead>
                <TableHead className="text-right pr-6 font-bold text-slate-500 h-12">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={4} className="text-center py-20 text-slate-400">正在获取黑名单设备...</TableCell>
                </TableRow>
              ) : devices.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="text-center py-20">
                    <div className="flex flex-col items-center gap-3">
                      <div className="bg-slate-50 p-4 rounded-full">
                        <ShieldAlert className="h-10 w-10 text-slate-300" />
                      </div>
                      <div className="space-y-1">
                        <p className="text-slate-900 font-bold">黑名单为空</p>
                        <p className="text-slate-400 text-sm">当前没有被屏蔽或吊销的设备</p>
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
                      <Badge variant="outline" className={device.meta.authenticate_status === 1 ? "bg-rose-50 text-rose-700 border-rose-200" : "bg-slate-100 text-slate-600 border-slate-200"}>
                        {device.meta.authenticate_status === 1 ? "已拒绝" : "已撤销"}
                      </Badge>
                    </TableCell>
                    <TableCell className="pr-6 text-right">
                      {/* 只有 ReadWrite(2) 及以上权限才能解除屏蔽 */}
                      {(user?.permission || 0) >= 2 && (
                        <div className="flex justify-end">
                          <Button
                            size="sm"
                            variant="outline"
                            className="h-9 bg-blue-50 text-blue-700 border-blue-200 hover:bg-blue-600 hover:text-white rounded-xl px-4 transition-all shadow-sm"
                            onClick={async () => {
                              const ok = await askConfirm({
                                title: "移出黑名单",
                                description: "确定要将该设备移出黑名单吗？操作后设备将进入待认证状态。",
                                confirmText: "确认移出",
                                cancelText: "取消",
                              });
                              if (ok) {
                                unblockMutation.mutate(device.uuid);
                              } 
                            }}
                            disabled={unblockMutation.isPending}
                          >
                            <RefreshCw className="h-4 w-4 mr-2" />
                            解除屏蔽
                          </Button>
                        </div>
                      )}
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
