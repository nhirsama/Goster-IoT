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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Ban, RefreshCw, ShieldAlert } from "lucide-react";

type DeviceRecord = components["schemas"]["DeviceRecord"];

export default function BlacklistPage() {
  const { isAuthenticated, user } = useAuth();
  const queryClient = useQueryClient();
  const { toast, confirm: askConfirm } = useUx();

  const { data: revokedData, isLoading: revokedLoading, isFetching: revokedFetching } = useQuery({
    queryKey: queryKeys.devicesByStatus("revoked"),
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "revoked" }),
    enabled: isAuthenticated && (user?.permission || 0) >= 1,
  });

  const { data: refusedData, isLoading: refusedLoading, isFetching: refusedFetching } = useQuery({
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

  if (!isAuthenticated) {
    return (
      <EmptyState
        icon={ShieldAlert}
        title="需要登录"
        description="请先登录后再访问黑名单页面。"
        className="py-24"
      />
    );
  }

  if ((user?.permission || 0) < 1) {
    return (
      <EmptyState
        icon={ShieldAlert}
        title="权限不足"
        description="此页面至少需要只读权限（ReadOnly）。"
        className="py-24"
      />
    );
  }

  const isLoading = revokedLoading || refusedLoading;
  const isFetching = revokedFetching || refusedFetching;
  const merged = [...(revokedData?.items || []), ...(refusedData?.items || [])];
  const deviceMap = new Map<string, DeviceRecord>();
  merged.forEach((device: DeviceRecord) => {
    deviceMap.set(device.uuid, device);
  });
  const devices = Array.from(deviceMap.values());

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Ban}
        title="设备黑名单"
        description="管理被拒绝认证或已吊销的设备。"
        action={
          <Button
            variant="outline"
            onClick={() => {
              queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("revoked") });
              queryClient.invalidateQueries({ queryKey: queryKeys.devicesByStatus("refused") });
            }}
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
                <TableHead className="h-12 text-slate-500">状态</TableHead>
                <TableHead className="h-12 pr-6 text-right text-slate-500">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={4}>
                    <EmptyState icon={RefreshCw} title="正在获取黑名单设备" description="请稍候..." className="py-16" />
                  </TableCell>
                </TableRow>
              ) : devices.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4}>
                    <EmptyState icon={ShieldAlert} title="黑名单为空" description="当前没有被屏蔽或吊销的设备。" className="py-16" />
                  </TableCell>
                </TableRow>
              ) : (
                devices.map((device: DeviceRecord) => (
                  <TableRow key={device.uuid} className="border-slate-100/70">
                    <TableCell className="pl-6 py-4">
                      <div className="font-mono text-xs text-slate-600">{device.uuid}</div>
                      <div className="mt-1 font-mono text-xs text-slate-400">{device.meta.mac}</div>
                    </TableCell>
                    <TableCell className="font-medium text-slate-900">{device.meta.name}</TableCell>
                    <TableCell>
                      <Badge
                        variant="outline"
                        className={
                          device.meta.authenticate_status === 1
                            ? "border-rose-200 bg-rose-50 text-rose-700"
                            : "border-slate-200 bg-slate-100 text-slate-600"
                        }
                      >
                        {device.meta.authenticate_status === 1 ? "已拒绝" : "已撤销"}
                      </Badge>
                    </TableCell>
                    <TableCell className="pr-6 text-right">
                      {(user?.permission || 0) >= 2 ? (
                        <Button
                          size="sm"
                          variant="outline"
                          className="border-primary/20 text-primary hover:bg-primary/10"
                          onClick={async () => {
                            const ok = await askConfirm({
                              title: "移出黑名单",
                              description: "设备移出黑名单后将进入待认证状态。",
                              confirmText: "确认移出",
                              cancelText: "取消",
                            });
                            if (ok) unblockMutation.mutate(device.uuid);
                          }}
                          disabled={unblockMutation.isPending}
                        >
                          <RefreshCw className="h-4 w-4" />
                          解除屏蔽
                        </Button>
                      ) : null}
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
