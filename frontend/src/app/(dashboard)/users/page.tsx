"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Check, Shield, ShieldCheck, UserCog, Users } from "lucide-react";

type UserType = components["schemas"]["User"];
type PermissionType = components["schemas"]["PermissionType"];

const PERMISSION_LABELS: Record<number, { label: string; className: string }> = {
  0: { label: "无权限", className: "bg-slate-100 text-slate-500 border-slate-200" },
  1: { label: "只读", className: "bg-blue-50 text-blue-600 border-blue-200" },
  2: { label: "读写", className: "bg-indigo-50 text-indigo-600 border-indigo-200" },
  3: { label: "超级管理员", className: "bg-purple-50 text-purple-700 border-purple-200" },
};

const PERMISSION_ENTRIES = Object.entries(PERMISSION_LABELS) as Array<
  [string, { label: string; className: string }]
>;

export default function UserManagementPage() {
  const { user: currentUser, isAuthenticated, isLoading: authLoading } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();
  const { toast } = useUx();

  useEffect(() => {
    if (!authLoading && (!isAuthenticated || currentUser?.permission !== 3)) {
      router.push("/");
    }
  }, [authLoading, currentUser, isAuthenticated, router]);

  const { data: userData, isLoading: usersLoading } = useQuery({
    queryKey: queryKeys.users,
    queryFn: () => api.get<components["schemas"]["UserListData"]>("/api/v1/users"),
    enabled: isAuthenticated && currentUser?.permission === 3,
  });

  const updatePermissionMutation = useMutation({
    mutationFn: ({ username, permission }: { username: string; permission: PermissionType }) =>
      api.post(`/api/v1/users/${encodeURIComponent(username)}/permission`, { permission }),
    onSuccess: () => {
      toast.success("用户权限已更新");
      queryClient.invalidateQueries({ queryKey: queryKeys.users });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "权限更新失败，请稍后重试"));
    },
  });

  if (authLoading || !isAuthenticated || currentUser?.permission !== 3) {
    return null;
  }

  const users = userData?.items || [];

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Users}
        title="用户管理"
        description="配置系统访问权限与角色分配。"
      />

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader className="bg-slate-50/50">
              <TableRow className="border-slate-200/70 hover:bg-transparent">
                <TableHead className="h-12 pl-6 text-slate-500">用户名</TableHead>
                <TableHead className="h-12 text-slate-500">当前角色</TableHead>
                <TableHead className="h-12 text-slate-500">注册时间</TableHead>
                <TableHead className="h-12 pr-6 text-right text-slate-500">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {usersLoading ? (
                <TableRow>
                  <TableCell colSpan={4}>
                    <EmptyState icon={Users} title="正在加载用户数据" description="请稍候..." className="py-16" />
                  </TableCell>
                </TableRow>
              ) : users.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4}>
                    <EmptyState icon={Users} title="暂无用户数据" description="当前还没有可管理的账号。" className="py-16" />
                  </TableCell>
                </TableRow>
              ) : (
                users.map((u: UserType) => (
                  <TableRow key={u.username} className="border-slate-100/70">
                    <TableCell className="pl-6 py-4">
                      <div className="flex items-center gap-2">
                        <div className="rounded-lg bg-slate-100 p-2 text-slate-500">
                          <Shield className="h-3.5 w-3.5" />
                        </div>
                        <span className="font-medium text-slate-900">{u.username}</span>
                        {u.username === currentUser.username ? (
                          <Badge variant="outline" className="border-primary/20 bg-primary/10 text-primary">
                            当前用户
                          </Badge>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className={PERMISSION_LABELS[u.permission]?.className}>
                        {PERMISSION_LABELS[u.permission]?.label}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm text-slate-500">
                      {new Date(u.created_at).toLocaleString("zh-CN", {
                        year: "numeric",
                        month: "2-digit",
                        day: "2-digit",
                        hour: "2-digit",
                        minute: "2-digit",
                        second: "2-digit",
                        hour12: false,
                      })}
                    </TableCell>
                    <TableCell className="pr-6 text-right">
                      <Dialog>
                        <DialogTrigger
                          render={
                            <Button
                              variant="outline"
                              size="sm"
                              disabled={u.username === currentUser.username || updatePermissionMutation.isPending}
                            />
                          }
                        >
                          <UserCog className="h-4 w-4" />
                          更改权限
                        </DialogTrigger>
                        <DialogContent className="max-w-md rounded-2xl">
                          <DialogHeader>
                            <DialogTitle>调整用户权限</DialogTitle>
                            <DialogDescription>
                              用户 <span className="font-medium text-primary">{u.username}</span>
                            </DialogDescription>
                          </DialogHeader>
                          <div className="grid gap-2 pt-2">
                            {PERMISSION_ENTRIES.map(([val, { label, className }]) => {
                              const numericPermission = Number(val);
                              const selected = u.permission === numericPermission;
                              return (
                                <Button
                                  key={val}
                                  variant={selected ? "default" : "outline"}
                                  className="h-11 justify-between"
                                  onClick={() => {
                                    if (u.username === currentUser.username && numericPermission !== 3) {
                                      toast.error("不能将当前管理员权限降级。");
                                      return;
                                    }
                                    updatePermissionMutation.mutate({
                                      username: u.username,
                                      permission: numericPermission as PermissionType,
                                    });
                                  }}
                                  disabled={updatePermissionMutation.isPending}
                                >
                                  <span className="flex items-center gap-2">
                                    <Badge variant="outline" className={className}>
                                      {label}
                                    </Badge>
                                  </span>
                                  {selected ? <Check className="h-4 w-4" /> : <ShieldCheck className="h-4 w-4 text-slate-300" />}
                                </Button>
                              );
                            })}
                          </div>
                        </DialogContent>
                      </Dialog>
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
