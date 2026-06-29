"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { queryKeys } from "@/lib/query-keys";
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
import { Card, CardContent } from "@/components/ui/card";
import { Shield, ShieldAlert, Users } from "lucide-react";

type UserType = components["schemas"]["User"];
type TenantRole = components["schemas"]["TenantRole"];

const ROLE_LABELS: Record<TenantRole, { label: string; className: string }> = {
  tenant_admin: { label: "租户管理员", className: "border-purple-200 bg-purple-50 text-purple-700" },
  tenant_rw: { label: "租户读写", className: "border-blue-200 bg-blue-50 text-blue-700" },
  tenant_ro: { label: "租户只读", className: "border-slate-200 bg-slate-100 text-slate-600" },
};

export default function UserManagementPage() {
  const { user: currentUser, isAuthenticated, isLoading: authLoading } = useAuth();

  const { data: userData, isLoading: usersLoading } = useQuery({
    queryKey: queryKeys.users,
    queryFn: () => api.get<components["schemas"]["UserListData"]>("/api/v1/users"),
    enabled: isAuthenticated && currentUser?.permission === 3,
  });

  if (authLoading) {
    return <EmptyState icon={Users} title="正在校验登录状态" description="请稍候..." className="py-24" />;
  }

  if (!isAuthenticated) {
    return <EmptyState icon={ShieldAlert} title="需要登录" description="请先登录后再访问用户管理页面。" className="py-24" />;
  }

  if (currentUser?.permission !== 3) {
    return <EmptyState icon={ShieldAlert} title="权限不足" description="只有当前租户管理员可以访问账号列表。" className="py-24" />;
  }

  const users = userData?.items || [];

  return (
    <div className="space-y-6">
        <PageHeader
          icon={Users}
          title="账号列表"
          description="查看账号在当前租户下的角色。角色调整请在租户管理中完成。"
        />

      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader className="bg-slate-50/50">
                <TableRow className="border-slate-200/70 hover:bg-transparent">
                  <TableHead className="h-12 pl-6 text-slate-500">用户名</TableHead>
                  <TableHead className="h-12 text-slate-500">当前租户角色</TableHead>
                  <TableHead className="h-12 text-slate-500">注册时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {usersLoading ? (
                  <TableRow>
                    <TableCell colSpan={3}>
                      <EmptyState icon={Users} title="正在加载用户数据" description="请稍候..." className="py-16" />
                    </TableCell>
                  </TableRow>
                ) : users.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={3}>
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
                      {u.tenant_role ? (
                        <Badge variant="outline" className={ROLE_LABELS[u.tenant_role].className}>
                          {ROLE_LABELS[u.tenant_role].label}
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="border-slate-200 bg-slate-100 text-slate-500">
                          未加入当前租户
                        </Badge>
                      )}
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
