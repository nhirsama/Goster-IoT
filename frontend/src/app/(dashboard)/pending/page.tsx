"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, getApiErrorMessage } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { queryKeys } from "@/lib/query-keys";
import { useUx } from "@/components/providers/ux-provider";
import { PageHeader } from "@/components/dashboard/page-header";
import { EmptyState } from "@/components/dashboard/empty-state";
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
import { Card, CardContent } from "@/components/ui/card";
import { Bell, CheckCircle2, RefreshCw, ShieldAlert, XCircle } from "lucide-react";

type DeviceRecord = components["schemas"]["DeviceRecord"];

export default function PendingDevicesPage() {
  const { isAuthenticated, user } = useAuth();
  const queryClient = useQueryClient();
  const { toast, confirm: askConfirm } = useUx();

  const { data: deviceData, isLoading, isFetching } = useQuery({
    queryKey: queryKeys.devicesByStatus("pending"),
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "pending" }),
    enabled: isAuthenticated && (user?.permission || 0) >= 2,
  });

  const approveMutation = useMutation({
    mutationFn: (uuid: string) => api.post(`/api/v1/devices/${uuid}/approve`),
    onSuccess: () => {
      toast.success("设备已通过认证");
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("pending") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("authenticated") });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "设备批准失败，请稍后重试"));
    },
  });

  const revokeMutation = useMutation({
    mutationFn: (uuid: string) => api.post(`/api/v1/devices/${uuid}/revoke`),
    onSuccess: () => {
      toast.success("设备已拒绝接入");
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("pending") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("authenticated") });
      queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("refused") });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "设备拒绝操作失败，请稍后重试"));
    },
  });

  if (!isAuthenticated) {
    return (
      <EmptyState
        icon={ShieldAlert}
        title="需要登录"
        description="请先登录后再访问待审批页面。"
        className="py-24"
      />
    );
  }

  if ((user?.permission || 0) < 2) {
    return (
      <EmptyState
        icon={ShieldAlert}
        title="权限不足"
        description="此页面需要读写权限（ReadWrite）及以上。"
        className="py-24"
      />
    );
  }

  const devices = deviceData?.items || [];

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Bell}
        title="待处理认证"
        description="审批新接入系统的 IoT 设备。"
        action={
          <Button
            variant="outline"
            onClick={() => queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("pending") })}
            disabled={isFetching}
          >
            <RefreshCw className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
            刷新列表
          </Button>
        }
      />

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader className="bg-slate-50/50">
              <TableRow className="border-slate-200/70 hover:bg-transparent">
                <TableHead className="h-12 pl-6 text-slate-500">设备标识</TableHead>
                <TableHead className="h-12 text-slate-500">设备名称</TableHead>
                <TableHead className="h-12 text-slate-500">SN</TableHead>
                <TableHead className="h-12 pr-6 text-right text-slate-500">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={4}>
                    <EmptyState icon={RefreshCw} title="正在获取待审批设备" description="请稍候..." className="py-16" />
                  </TableCell>
                </TableRow>
              ) : devices.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4}>
                    <EmptyState icon={CheckCircle2} title="没有待处理设备" description="当前接入请求已全部处理完成。" className="py-16" />
                  </TableCell>
                </TableRow>
              ) : (
                devices.map((device: DeviceRecord) => (
                  <TableRow key={device.uuid} className="border-slate-100/70">
                    <TableCell className="pl-6 py-4">
                      <div className="font-mono text-xs text-slate-600">{device.uuid}</div>
                      <div className="mt-1 font-mono text-xs text-slate-400">{device.meta.mac}</div>
                    </TableCell>
                    <TableCell>
                      <div className="font-medium text-slate-900">{device.meta.name}</div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="bg-slate-100 text-slate-600">
                        {device.meta.sn}
                      </Badge>
                    </TableCell>
                    <TableCell className="pr-6 text-right">
                      <div className="flex justify-end gap-2">
                        <Button
                          size="sm"
                          className="bg-emerald-600 text-white hover:bg-emerald-500"
                          onClick={() => approveMutation.mutate(device.uuid)}
                          disabled={approveMutation.isPending || revokeMutation.isPending}
                        >
                          <CheckCircle2 className="h-4 w-4" />
                          批准
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          className="border-rose-200 text-rose-600 hover:bg-rose-50"
                          onClick={async () => {
                            const ok = await askConfirm({
                              title: "拒绝设备接入",
                              description: "确定要拒绝该设备的接入请求吗？",
                              confirmText: "确认拒绝",
                              cancelText: "取消",
                              tone: "danger",
                            });
                            if (ok) revokeMutation.mutate(device.uuid);
                          }}
                          disabled={approveMutation.isPending || revokeMutation.isPending}
                        >
                          <XCircle className="h-4 w-4" />
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
