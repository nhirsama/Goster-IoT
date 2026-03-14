"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, getApiErrorMessage } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { queryKeys } from "@/lib/query-keys";
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
import { Card, CardContent } from "@/components/ui/card";
import { ChevronLeft, UserCog, ShieldCheck, User, Calendar, Shield } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";

type UserType = components["schemas"]["User"];
type PermissionType = components["schemas"]["PermissionType"];

const PERMISSION_LABELS: Record<number, { label: string; color: string }> = {
  0: { label: "无权限", color: "bg-slate-100 text-slate-500" },
  1: { label: "只读", color: "bg-blue-50 text-blue-600 border-blue-100" },
  2: { label: "读写", color: "bg-indigo-50 text-indigo-600 border-indigo-100" },
  3: { label: "超级管理员", color: "bg-purple-50 text-purple-700 border-purple-200" },
};
const PERMISSION_ENTRIES = Object.entries(PERMISSION_LABELS) as Array<[string, { label: string; color: string }]>;

export default function UserManagementPage() {
  const { user: currentUser, isAuthenticated, isLoading: authLoading } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    if (!authLoading && (!isAuthenticated || currentUser?.permission !== 3)) {
      router.push("/");
    }
  }, [isAuthenticated, currentUser, authLoading, router]);

  const { data: userData, isLoading: usersLoading } = useQuery({
    queryKey: queryKeys.users,
    queryFn: () => api.get<components["schemas"]["UserListData"]>("/api/v1/users"),
    enabled: isAuthenticated && currentUser?.permission === 3,
  });

  const updatePermissionMutation = useMutation({
    mutationFn: ({ username, permission }: { username: string; permission: PermissionType }) =>
      api.post(`/api/v1/users/${encodeURIComponent(username)}/permission`, { permission }),
    onSuccess: () => {
      setActionError(null);
      queryClient.invalidateQueries({ queryKey: queryKeys.users });
    },
    onError: (error: unknown) => {
      setActionError(getApiErrorMessage(error, "权限更新失败，请稍后重试"));
    },
  });

  if (authLoading || !isAuthenticated || currentUser?.permission !== 3) {
    return null;
  }

  const users = userData?.items || [];

  return (
    <div className="min-h-screen bg-[#f8fafc] pb-12 font-sans">
      <header className="bg-white/80 backdrop-blur-md border-b sticky top-0 z-50 border-slate-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <Button variant="ghost" size="sm" className="rounded-xl hover:bg-slate-100" onClick={() => router.push("/")}>
              <ChevronLeft className="h-4 w-4 mr-1" />
              返回仪表盘
            </Button>
            <div className="h-4 w-px bg-slate-200 mx-2"></div>
            <h1 className="text-lg font-bold text-slate-900">系统用户管理</h1>
          </div>
        </div>
      </header>

      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-8">
        {actionError && (
          <div className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
            {actionError}
          </div>
        )}
        <div className="flex flex-col md:flex-row justify-between items-start md:items-end gap-4">
          <div>
            <h2 className="text-3xl font-black text-slate-900 tracking-tight">用户列表</h2>
            <p className="text-slate-500 mt-1 font-medium">配置系统访问权限与角色分配</p>
          </div>
        </div>

        <Card className="border-none shadow-xl shadow-slate-200/50 bg-white overflow-hidden rounded-2xl">
          <CardContent className="p-0">
            <Table>
              <TableHeader className="bg-slate-50/50">
                <TableRow className="border-slate-100 hover:bg-transparent">
                  <TableHead className="font-bold text-slate-500 pl-6 h-12">用户信息</TableHead>
                  <TableHead className="font-bold text-slate-500 h-12">当前角色</TableHead>
                  <TableHead className="font-bold text-slate-500 h-12">注册时间</TableHead>
                  <TableHead className="text-right pr-6 font-bold text-slate-500 h-12">管理操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {usersLoading ? (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center py-20 text-slate-400">正在加载用户数据...</TableCell>
                  </TableRow>
                ) : (
                  users.map((u: UserType) => (
                    <TableRow key={u.username} className="group border-slate-50 hover:bg-slate-50/50 transition-colors">
                      <TableCell className="pl-6 py-5">
                        <div className="flex items-center gap-3">
                          <div className="h-10 w-10 rounded-full bg-slate-100 flex items-center justify-center text-slate-400 group-hover:bg-blue-600 group-hover:text-white transition-all">
                            <User className="h-5 w-5" />
                          </div>
                          <span className="font-bold text-slate-900 text-lg">{u.username}</span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className={`${PERMISSION_LABELS[u.permission]?.color} rounded-lg px-3 py-1 font-bold text-xs shadow-sm`}>
                          {PERMISSION_LABELS[u.permission]?.label}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-2 text-slate-500 text-sm font-medium">
                          <Calendar className="h-3.5 w-3.5" />
                          {new Date(u.created_at).toLocaleString("zh-CN", {
                            year: "numeric",
                            month: "2-digit",
                            day: "2-digit",
                            hour: "2-digit",
                            minute: "2-digit",
                            second: "2-digit",
                            hour12: false,
                          })}
                        </div>
                      </TableCell>
                      <TableCell className="text-right pr-6">
                        <Dialog>
                          <DialogTrigger
                            render={
                              <Button
                                variant="outline"
                                size="sm"
                                className="rounded-xl border-slate-200 hover:bg-blue-50 hover:text-blue-600 hover:border-blue-100 transition-all font-bold"
                                disabled={u.username === currentUser.username || updatePermissionMutation.isPending}
                              />
                            }
                          >
                            <UserCog className="h-4 w-4 mr-2" />
                            更改权限
                          </DialogTrigger>
                          <DialogContent className="rounded-3xl border-none shadow-2xl">
                            <DialogHeader>
                              <DialogTitle className="text-xl font-black">调整用户权限</DialogTitle>
                              <DialogDescription className="font-medium">
                                正在为用户 <span className="text-blue-600 font-bold">{u.username}</span> 分配新的系统角色
                              </DialogDescription>
                            </DialogHeader>
                            <div className="grid gap-3 py-6">
                              {PERMISSION_ENTRIES.map(([val, { label }]) => (
                                <Button
                                  key={val}
                                  variant={u.permission === Number(val) ? "default" : "outline"}
                                  className={`justify-between h-14 rounded-2xl px-6 font-bold transition-all ${u.permission === Number(val) ? "bg-blue-600 shadow-lg shadow-blue-100" : "border-slate-100 hover:border-blue-200 hover:bg-blue-50/50"}`}
                                  onClick={() => {
                                    if (u.username === currentUser.username && Number(val) !== 3) {
                                      setActionError("出于安全限制，当前登录管理员不能在前端把自己的权限降级。");
                                      return;
                                    }
                                    updatePermissionMutation.mutate({ 
                                      username: u.username, 
                                      permission: Number(val) as PermissionType 
                                    });
                                  }}
                                  disabled={updatePermissionMutation.isPending}
                                >
                                  <div className="flex items-center gap-3">
                                    <Shield className={`h-5 w-5 ${u.permission === Number(val) ? "text-blue-100" : "text-slate-400"}`} />
                                    {label}
                                  </div>
                                  {u.permission === Number(val) && <ShieldCheck className="h-5 w-5 text-white" />}
                                </Button>
                              ))}
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
      </main>
    </div>
  );
}
