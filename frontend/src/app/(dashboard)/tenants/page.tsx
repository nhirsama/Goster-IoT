"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, getActiveTenantId, getApiErrorMessage, setActiveTenantId } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { queryKeys } from "@/lib/query-keys";
import { useAuth } from "@/hooks/use-auth";
import { useUx } from "@/components/providers/ux-provider";
import { PageHeader } from "@/components/dashboard/page-header";
import { EmptyState } from "@/components/dashboard/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Building2, Check, Plus, RefreshCw, ShieldAlert, Trash2, UserPlus, Users } from "lucide-react";

type TenantStatus = "active" | "suspended" | "archived";
type TenantRole = "tenant_admin" | "tenant_rw" | "tenant_ro";
type Tenant = {
  id: string;
  name: string;
  status: TenantStatus;
  role?: TenantRole;
  created_at: string;
  updated_at?: string | null;
};
type TenantUser = {
  tenant_id: string;
  username: string;
  role: TenantRole;
  created_at: string;
};
type TenantListData = {
  items: Tenant[];
  total: number;
};
type TenantUserListData = {
  items: TenantUser[];
  total: number;
};
type UserListData = components["schemas"]["UserListData"];

const statusMeta: Record<TenantStatus, { label: string; className: string }> = {
  active: { label: "活跃", className: "border-emerald-200 bg-emerald-50 text-emerald-700" },
  suspended: { label: "暂停", className: "border-amber-200 bg-amber-50 text-amber-700" },
  archived: { label: "归档", className: "border-slate-200 bg-slate-100 text-slate-600" },
};

const roleMeta: Record<TenantRole, { label: string; className: string }> = {
  tenant_admin: { label: "租户管理员", className: "border-purple-200 bg-purple-50 text-purple-700" },
  tenant_rw: { label: "租户读写", className: "border-blue-200 bg-blue-50 text-blue-700" },
  tenant_ro: { label: "租户只读", className: "border-slate-200 bg-slate-100 text-slate-600" },
};

const statusOptions: TenantStatus[] = ["active", "suspended", "archived"];
const roleOptions: TenantRole[] = ["tenant_admin", "tenant_rw", "tenant_ro"];
const emptyTenants: Tenant[] = [];

function formatDate(value?: string | null) {
  if (!value) return "-";
  return new Date(value).toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

export default function TenantsPage() {
  const queryClient = useQueryClient();
  const { user, isAuthenticated, isLoading: authLoading, refetch: refetchAuth } = useAuth();
  const { toast, confirm } = useUx();
  const [keyword, setKeyword] = useState("");
  const [selectedTenantId, setSelectedTenantId] = useState(getActiveTenantId() || user?.active_tenant || "tenant_legacy");
  const [newTenantName, setNewTenantName] = useState("");
  const [newTenantStatus, setNewTenantStatus] = useState<TenantStatus>("active");
  const [memberUsername, setMemberUsername] = useState("");
  const [memberRole, setMemberRole] = useState<TenantRole>("tenant_ro");

  const tenantsQuery = useQuery({
    queryKey: queryKeys.tenants,
    queryFn: () => api.get<TenantListData>("/api/v1/tenants"),
    enabled: isAuthenticated,
  });

  const tenants = tenantsQuery.data?.items || emptyTenants;
  const selectedTenant = tenants.find((tenant) => tenant.id === selectedTenantId) || tenants[0];
  const canManageSelectedTenant = selectedTenant?.role === "tenant_admin";

  const tenantUsersQuery = useQuery({
    queryKey: queryKeys.tenantUsers(selectedTenantId),
    queryFn: () => api.get<TenantUserListData>(`/api/v1/tenants/${encodeURIComponent(selectedTenantId)}/users`),
    enabled: isAuthenticated && canManageSelectedTenant && !!selectedTenantId,
  });

  const usersQuery = useQuery({
    queryKey: queryKeys.users,
    queryFn: () => api.get<UserListData>("/api/v1/users"),
    enabled: isAuthenticated && canManageSelectedTenant,
  });

  const createTenantMutation = useMutation({
    mutationFn: () =>
      api.post<Tenant>("/api/v1/tenants", {
        name: newTenantName.trim(),
        status: newTenantStatus,
      }),
    onSuccess: (tenant) => {
      toast.success("租户已创建");
      setNewTenantName("");
      setNewTenantStatus("active");
      setSelectedTenantId(tenant.id);
      setActiveTenantId(tenant.id);
      queryClient.invalidateQueries({ queryKey: queryKeys.tenants });
      queryClient.invalidateQueries({ queryKey: queryKeys.authMe });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "创建租户失败"));
    },
  });

  const updateTenantMutation = useMutation({
    mutationFn: ({ tenantId, status }: { tenantId: string; status: TenantStatus }) =>
      api.patch<Tenant>(`/api/v1/tenants/${encodeURIComponent(tenantId)}`, { status }),
    onSuccess: () => {
      toast.success("租户状态已更新");
      queryClient.invalidateQueries({ queryKey: queryKeys.tenants });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "更新租户失败"));
    },
  });

  const addTenantUserMutation = useMutation({
    mutationFn: () =>
      api.post(`/api/v1/tenants/${encodeURIComponent(selectedTenantId)}/users`, {
        username: memberUsername.trim(),
        role: memberRole,
      }),
    onSuccess: () => {
      toast.success("租户成员已保存");
      setMemberUsername("");
      setMemberRole("tenant_ro");
      queryClient.invalidateQueries({ queryKey: queryKeys.tenantUsers(selectedTenantId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.authMe });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "保存租户成员失败"));
    },
  });

  const removeTenantUserMutation = useMutation({
    mutationFn: ({ tenantId, username }: { tenantId: string; username: string }) =>
      api.delete(`/api/v1/tenants/${encodeURIComponent(tenantId)}/users/${encodeURIComponent(username)}`),
    onSuccess: () => {
      toast.success("租户成员已移除");
      queryClient.invalidateQueries({ queryKey: queryKeys.tenantUsers(selectedTenantId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.authMe });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "移除租户成员失败"));
    },
  });

  const filteredTenants = useMemo(() => {
    const normalized = keyword.trim().toLowerCase();
    if (!normalized) return tenants;
    return tenants.filter((tenant) =>
      [tenant.id, tenant.name, tenant.status].some((value) => value.toLowerCase().includes(normalized))
    );
  }, [keyword, tenants]);
  const members = tenantUsersQuery.data?.items || [];
  const knownUsers = usersQuery.data?.items || [];

  const switchTenant = async (tenantId: string) => {
    setActiveTenantId(tenantId);
    setSelectedTenantId(tenantId);
    await refetchAuth();
    queryClient.invalidateQueries();
    toast.success("当前租户已切换");
  };

  if (authLoading) {
    return <EmptyState icon={RefreshCw} title="正在校验会话状态" description="请稍候..." className="py-24" />;
  }

  if (!isAuthenticated) {
    return <EmptyState icon={ShieldAlert} title="需要登录" description="请先登录后再访问租户管理。" className="py-24" />;
  }

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Building2}
        title="租户管理"
        description="管理租户主档、成员角色和当前工作租户。"
        action={
          <Dialog>
            <DialogTrigger render={<Button />}>
              <Plus className="h-4 w-4" />
              新建租户
            </DialogTrigger>
            <DialogContent className="max-w-md">
              <DialogHeader>
                <DialogTitle>新建租户</DialogTitle>
                <DialogDescription>租户 ID 会根据名称自动生成，可在后端保持稳定引用。</DialogDescription>
              </DialogHeader>
              <div className="space-y-3">
                <label className="space-y-1">
                  <span className="text-xs font-semibold text-slate-500">租户名称</span>
                  <Input value={newTenantName} onChange={(event) => setNewTenantName(event.target.value)} />
                </label>
                <label className="space-y-1">
                  <span className="text-xs font-semibold text-slate-500">初始状态</span>
                  <select
                    value={newTenantStatus}
                    onChange={(event) => setNewTenantStatus(event.target.value as TenantStatus)}
                    className="h-9 w-full rounded-lg border border-input bg-background px-2.5 text-sm outline-none"
                  >
                    {statusOptions.map((status) => (
                      <option key={status} value={status}>
                        {statusMeta[status].label}
                      </option>
                    ))}
                  </select>
                </label>
                <Button
                  className="w-full"
                  disabled={!newTenantName.trim() || createTenantMutation.isPending}
                  onClick={() => createTenantMutation.mutate()}
                >
                  创建租户
                </Button>
              </div>
            </DialogContent>
          </Dialog>
        }
      />

      <div className="grid gap-6 xl:grid-cols-[1.2fr_1fr]">
        <Card>
          <CardHeader className="border-b border-slate-200/70 pb-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <CardTitle className="text-base font-semibold text-slate-900">租户列表</CardTitle>
              <Badge variant="outline" className="w-fit rounded-full bg-white/70 text-slate-600">
                共 {filteredTenants.length} 个
              </Badge>
            </div>
            <Input
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              placeholder="搜索租户名称 / ID / 状态"
              className="h-10"
            />
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader className="bg-slate-50/50">
                <TableRow>
                  <TableHead className="pl-6">租户</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>创建时间</TableHead>
                  <TableHead className="pr-6 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tenantsQuery.isLoading ? (
                  <TableRow>
                    <TableCell colSpan={4}>
                      <EmptyState icon={RefreshCw} title="正在加载租户" description="请稍候..." className="py-16" />
                    </TableCell>
                  </TableRow>
                ) : filteredTenants.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={4}>
                      <EmptyState icon={Building2} title="暂无租户" description="创建租户后会显示在这里。" className="py-16" />
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredTenants.map((tenant) => (
                    <TableRow key={tenant.id} className={tenant.id === selectedTenantId ? "bg-primary/5" : ""}>
                      <TableCell className="pl-6">
                        <button
                          className="block min-w-0 text-left"
                          onClick={() => setSelectedTenantId(tenant.id)}
                        >
                          <span className="block truncate font-semibold text-slate-900">{tenant.name}</span>
                          <span className="block truncate font-mono text-xs text-slate-500">{tenant.id}</span>
                          {tenant.role ? (
                            <span className="mt-1 inline-flex">
                              <Badge variant="outline" className={roleMeta[tenant.role].className}>
                                {roleMeta[tenant.role].label}
                              </Badge>
                            </span>
                          ) : null}
                        </button>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className={statusMeta[tenant.status]?.className}>
                          {statusMeta[tenant.status]?.label || tenant.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-sm text-slate-500">{formatDate(tenant.created_at)}</TableCell>
                      <TableCell className="pr-6 text-right">
                        <div className="flex justify-end gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => switchTenant(tenant.id)}
                          >
                            <Check className="h-4 w-4" />
                            切换
                          </Button>
                          <select
                            value={tenant.status}
                            onChange={(event) =>
                              updateTenantMutation.mutate({
                                tenantId: tenant.id,
                                status: event.target.value as TenantStatus,
                              })
                            }
                            disabled={tenant.role !== "tenant_admin" || updateTenantMutation.isPending}
                            className="h-8 rounded-lg border border-slate-200 bg-white px-2 text-sm"
                          >
                            {statusOptions.map((status) => (
                              <option key={status} value={status}>
                                {statusMeta[status].label}
                              </option>
                            ))}
                          </select>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

          <Card>
          <CardHeader className="border-b border-slate-200/70 pb-4">
            <div className="space-y-1">
              <CardTitle className="text-base font-semibold text-slate-900">租户成员</CardTitle>
              <p className="font-mono text-xs text-slate-500">{selectedTenant?.id || selectedTenantId}</p>
            </div>
          </CardHeader>
            <CardContent className="space-y-4 p-4">
            {canManageSelectedTenant ? (
              <div className="grid gap-2 sm:grid-cols-[1fr_auto]">
              <div className="grid gap-2 sm:grid-cols-2">
                <div>
                  <Input
                    list="tenant-users"
                    value={memberUsername}
                    onChange={(event) => setMemberUsername(event.target.value)}
                    placeholder="用户名"
                    className="h-9"
                  />
                  <datalist id="tenant-users">
                    {knownUsers.map((knownUser) => (
                      <option key={knownUser.username} value={knownUser.username} />
                    ))}
                  </datalist>
                </div>
                <select
                  value={memberRole}
                  onChange={(event) => setMemberRole(event.target.value as TenantRole)}
                  className="h-9 rounded-lg border border-input bg-background px-2.5 text-sm outline-none"
                >
                  {roleOptions.map((role) => (
                    <option key={role} value={role}>
                      {roleMeta[role].label}
                    </option>
                  ))}
                </select>
              </div>
              <Button
                className="h-9"
                disabled={!selectedTenantId || !memberUsername.trim() || addTenantUserMutation.isPending}
                onClick={() => addTenantUserMutation.mutate()}
              >
                <UserPlus className="h-4 w-4" />
                保存成员
              </Button>
              </div>
            ) : (
              <EmptyState icon={ShieldAlert} title="权限不足" description="只有该租户管理员可以管理成员。" className="py-10" />
            )}

            {canManageSelectedTenant ? (
              <div className="rounded-lg border border-slate-200">
              {tenantUsersQuery.isLoading ? (
                <EmptyState icon={RefreshCw} title="正在加载成员" description="请稍候..." className="py-16" />
              ) : members.length === 0 ? (
                <EmptyState icon={Users} title="暂无租户成员" description="添加成员后会显示在这里。" className="py-16" />
              ) : (
                <div className="divide-y divide-slate-200/70">
                  {members.map((member) => (
                    <div key={`${member.tenant_id}:${member.username}`} className="flex items-center justify-between gap-3 p-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-semibold text-slate-900">{member.username}</p>
                        <p className="text-xs text-slate-500">{formatDate(member.created_at)}</p>
                      </div>
                      <div className="flex shrink-0 items-center gap-2">
                        <Badge variant="outline" className={roleMeta[member.role]?.className}>
                          {roleMeta[member.role]?.label || member.role}
                        </Badge>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-rose-600 hover:bg-rose-50"
                          disabled={removeTenantUserMutation.isPending}
                          onClick={async () => {
                            const ok = await confirm({
                              title: "移除租户成员",
                              description: `确定要从 ${selectedTenantId} 移除 ${member.username} 吗？`,
                              confirmText: "移除",
                              cancelText: "取消",
                              tone: "danger",
                            });
                            if (ok) {
                              removeTenantUserMutation.mutate({
                                tenantId: selectedTenantId,
                                username: member.username,
                              });
                            }
                          }}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
              </div>
            ) : null}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
