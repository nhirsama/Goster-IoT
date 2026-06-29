"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Building2,
  Check,
  Clock3,
  Crown,
  Layers3,
  Plus,
  RefreshCw,
  Search,
  ShieldAlert,
  ShieldCheck,
  Trash2,
  UserPlus,
  Users,
} from "lucide-react";
import { api, getActiveTenantId, getApiErrorMessage, setActiveTenantId } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { cn } from "@/lib/utils";
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

type TenantStatus = "active" | "suspended" | "archived";
type TenantRole = "tenant_admin" | "tenant_rw" | "tenant_ro";
type TenantTab = "members" | "overview" | "settings";
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

const statusMeta: Record<TenantStatus, { label: string; className: string; dotClassName: string }> = {
  active: {
    label: "活跃",
    className: "border-emerald-200 bg-emerald-50 text-emerald-700",
    dotClassName: "bg-emerald-500",
  },
  suspended: {
    label: "暂停",
    className: "border-amber-200 bg-amber-50 text-amber-700",
    dotClassName: "bg-amber-500",
  },
  archived: {
    label: "归档",
    className: "border-slate-200 bg-slate-100 text-slate-600",
    dotClassName: "bg-slate-400",
  },
};

const roleMeta: Record<TenantRole, { label: string; className: string; icon: typeof ShieldCheck }> = {
  tenant_admin: {
    label: "租户管理员",
    className: "border-violet-200 bg-violet-50 text-violet-700",
    icon: Crown,
  },
  tenant_rw: {
    label: "租户读写",
    className: "border-cyan-200 bg-cyan-50 text-cyan-700",
    icon: ShieldCheck,
  },
  tenant_ro: {
    label: "租户只读",
    className: "border-slate-200 bg-slate-100 text-slate-600",
    icon: ShieldAlert,
  },
};

const statusOptions: TenantStatus[] = ["active", "suspended", "archived"];
const roleOptions: TenantRole[] = ["tenant_admin", "tenant_rw", "tenant_ro"];
const emptyTenants: Tenant[] = [];
const emptyMembers: TenantUser[] = [];

const tenantTabs: Array<{ value: TenantTab; label: string; icon: typeof Users }> = [
  { value: "members", label: "成员", icon: Users },
  { value: "overview", label: "概览", icon: Building2 },
  { value: "settings", label: "策略", icon: ShieldCheck },
];

function asText(value: unknown, fallback = "") {
  return typeof value === "string" ? value : fallback;
}

function normalizeTenantStatus(value: unknown): TenantStatus {
  return statusOptions.includes(value as TenantStatus) ? (value as TenantStatus) : "active";
}

function normalizeTenantRole(value: unknown): TenantRole | undefined {
  return roleOptions.includes(value as TenantRole) ? (value as TenantRole) : undefined;
}

function getStatusMeta(status?: TenantStatus) {
  return statusMeta[status || "active"];
}

function getRoleMeta(role?: TenantRole) {
  return role ? roleMeta[role] : undefined;
}

function normalizeTenant(raw: Tenant): Tenant {
  const id = asText(raw.id, "tenant_legacy").trim() || "tenant_legacy";
  return {
    ...raw,
    id,
    name: asText(raw.name, id).trim() || id,
    status: normalizeTenantStatus(raw.status),
    role: normalizeTenantRole(raw.role),
    created_at: asText(raw.created_at),
    updated_at: raw.updated_at ? asText(raw.updated_at) : null,
  };
}

function normalizeTenantUser(raw: TenantUser): TenantUser {
  return {
    ...raw,
    tenant_id: asText(raw.tenant_id),
    username: asText(raw.username),
    role: normalizeTenantRole(raw.role) || "tenant_ro",
    created_at: asText(raw.created_at),
  };
}

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

function SelectField({
  value,
  onChange,
  disabled,
  children,
  className,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <select
      value={value}
      onChange={(event) => onChange(event.target.value)}
      disabled={disabled}
      className={cn(
        "h-9 rounded-lg border border-input bg-white px-2.5 text-sm font-medium text-slate-700 outline-none transition focus:border-primary/60 focus:ring-2 focus:ring-primary/15 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400",
        className
      )}
    >
      {children}
    </select>
  );
}

function StatItem({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Building2;
  label: string;
  value: string | number;
}) {
  return (
    <div className="flex min-h-20 items-center gap-3 rounded-lg border border-slate-200/80 bg-white px-4 py-3">
      <div className="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-slate-100 text-slate-600">
        <Icon className="h-4 w-4" />
      </div>
      <div className="min-w-0">
        <p className="text-xs font-medium text-slate-500">{label}</p>
        <p className="truncate text-lg font-semibold text-slate-950">{value}</p>
      </div>
    </div>
  );
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
  const [activeTab, setActiveTab] = useState<TenantTab>("members");

  const tenantsQuery = useQuery({
    queryKey: queryKeys.tenants,
    queryFn: () => api.get<TenantListData>("/api/v1/tenants"),
    enabled: isAuthenticated,
  });

  const tenants = useMemo(
    () => (tenantsQuery.data?.items || emptyTenants).map((tenant) => normalizeTenant(tenant)),
    [tenantsQuery.data?.items]
  );
  const selectedTenant = tenants.find((tenant) => tenant.id === selectedTenantId) || tenants[0];
  const effectiveSelectedTenantId = selectedTenant?.id || selectedTenantId;
  const canManageSelectedTenant = selectedTenant?.role === "tenant_admin";
  const activeTenantId = getActiveTenantId() || user?.active_tenant;
  const roleCounts = useMemo(
    () =>
      tenants.reduce(
        (counts, tenant) => {
          if (tenant.role === "tenant_admin") counts.admin += 1;
          if (tenant.role === "tenant_rw") counts.rw += 1;
          if (tenant.role === "tenant_ro") counts.ro += 1;
          return counts;
        },
        { admin: 0, rw: 0, ro: 0 }
      ),
    [tenants]
  );

  const tenantUsersQuery = useQuery({
    queryKey: queryKeys.tenantUsers(effectiveSelectedTenantId),
    queryFn: () => api.get<TenantUserListData>(`/api/v1/tenants/${encodeURIComponent(effectiveSelectedTenantId)}/users`),
    enabled: isAuthenticated && canManageSelectedTenant && !!effectiveSelectedTenantId,
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
      api.post(`/api/v1/tenants/${encodeURIComponent(effectiveSelectedTenantId)}/users`, {
        username: memberUsername.trim(),
        role: memberRole,
      }),
    onSuccess: () => {
      toast.success("邀请已发送，等待对方接受");
      setMemberUsername("");
      setMemberRole("tenant_ro");
      queryClient.invalidateQueries({ queryKey: queryKeys.tenantUsers(effectiveSelectedTenantId) });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "发送邀请失败"));
    },
  });

  const removeTenantUserMutation = useMutation({
    mutationFn: ({ tenantId, username }: { tenantId: string; username: string }) =>
      api.delete(`/api/v1/tenants/${encodeURIComponent(tenantId)}/users/${encodeURIComponent(username)}`),
    onSuccess: () => {
      toast.success("租户成员已移除");
      queryClient.invalidateQueries({ queryKey: queryKeys.tenantUsers(effectiveSelectedTenantId) });
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
      [tenant.id, tenant.name, tenant.status, tenant.role || ""].some((value) => asText(value).toLowerCase().includes(normalized))
    );
  }, [keyword, tenants]);

  const members = useMemo(
    () => (tenantUsersQuery.data?.items || emptyMembers).map((member) => normalizeTenantUser(member)),
    [tenantUsersQuery.data?.items]
  );
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
        description="按租户范围管理成员、角色和当前操作上下文。"
        action={
          <Dialog>
            <DialogTrigger render={<Button />}>
              <Plus className="h-4 w-4" />
              新建租户
            </DialogTrigger>
            <DialogContent className="max-w-md">
              <DialogHeader>
                <DialogTitle>新建租户</DialogTitle>
                <DialogDescription>创建者会自动成为该租户的管理员。</DialogDescription>
              </DialogHeader>
              <div className="space-y-3">
                <label className="space-y-1">
                  <span className="text-xs font-semibold text-slate-500">租户名称</span>
                  <Input value={newTenantName} onChange={(event) => setNewTenantName(event.target.value)} />
                </label>
                <label className="space-y-1">
                  <span className="text-xs font-semibold text-slate-500">初始状态</span>
                  <SelectField value={newTenantStatus} onChange={(value) => setNewTenantStatus(value as TenantStatus)} className="w-full">
                    {statusOptions.map((status) => (
                      <option key={status} value={status}>
                        {statusMeta[status].label}
                      </option>
                    ))}
                  </SelectField>
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

      <div className="grid gap-3 sm:grid-cols-3">
        <StatItem icon={Layers3} label="可访问租户" value={tenants.length} />
        <StatItem icon={Crown} label="可管理租户" value={roleCounts.admin} />
        <StatItem icon={Users} label="当前成员数" value={canManageSelectedTenant ? members.length : "-"} />
      </div>

      <div className="grid gap-6 lg:grid-cols-[340px_minmax(0,1fr)]">
        <Card className="lg:sticky lg:top-6 lg:self-start">
          <CardHeader className="border-b border-slate-200/70 pb-3">
            <div className="flex items-center justify-between gap-3">
              <CardTitle className="text-base font-semibold text-slate-900">租户列表</CardTitle>
              <Badge variant="outline" className="rounded-full bg-slate-50 text-slate-600">
                {filteredTenants.length}
              </Badge>
            </div>
            <div className="relative pt-2">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <Input
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                placeholder="搜索租户名称或 ID"
                className="h-9 pl-9 text-sm"
              />
            </div>
          </CardHeader>
          <CardContent className="p-3">
            {tenantsQuery.isLoading ? (
              <EmptyState icon={RefreshCw} title="加载中" description="请稍候..." className="py-16" />
            ) : filteredTenants.length === 0 ? (
              <EmptyState icon={Building2} title="暂无租户" description="创建租户后会显示在这里" className="py-16" />
            ) : (
              <div className="max-h-[520px] space-y-1.5 overflow-y-auto pr-1">
                {filteredTenants.map((tenant) => {
                  const tenantStatusMeta = getStatusMeta(tenant.status);
                  const tenantRoleMeta = getRoleMeta(tenant.role);
                  const RoleIcon = tenantRoleMeta?.icon || ShieldAlert;
                  const selected = tenant.id === effectiveSelectedTenantId;
                  const active = tenant.id === activeTenantId;

                  return (
                    <button
                      key={tenant.id}
                      type="button"
                      onClick={() => setSelectedTenantId(tenant.id)}
                      className={cn(
                        "group w-full rounded-lg border bg-white p-3 text-left transition",
                        selected
                          ? "border-primary/50 bg-primary/5 shadow-sm ring-1 ring-primary/10"
                          : "border-slate-200 hover:border-primary/30 hover:bg-slate-50"
                      )}
                    >
                      <div className="flex items-start gap-3">
                        <div className={cn(
                          "grid h-10 w-10 shrink-0 place-items-center rounded-lg transition",
                          selected ? "bg-primary/10 text-primary" : "bg-slate-100 text-slate-600 group-hover:bg-slate-200"
                        )}>
                          <Building2 className="h-4 w-4" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <span className="truncate text-sm font-semibold text-slate-900">{tenant.name}</span>
                            {active && <Check className="h-3.5 w-3.5 shrink-0 text-primary" />}
                          </div>
                          <p className="mt-0.5 truncate font-mono text-xs text-slate-500">{tenant.id}</p>
                          <div className="mt-2 flex flex-wrap items-center gap-1.5">
                            <Badge variant="outline" className={cn("h-5 gap-1 text-xs", tenantStatusMeta.className)}>
                              <span className={cn("h-1.5 w-1.5 rounded-full", tenantStatusMeta.dotClassName)} />
                              {tenantStatusMeta.label}
                            </Badge>
                            {tenantRoleMeta && (
                              <Badge variant="outline" className={cn("h-5 gap-1 text-xs", tenantRoleMeta.className)}>
                                <RoleIcon className="h-3 w-3" />
                                {tenantRoleMeta.label}
                              </Badge>
                            )}
                          </div>
                        </div>
                      </div>
                    </button>
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>

        <div className="space-y-6">
          {(() => {
            const selectedStatusMeta = getStatusMeta(selectedTenant?.status);
            const selectedRoleMeta = getRoleMeta(selectedTenant?.role);

            return (
          <section className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm">
            <div className="border-b border-slate-200 bg-slate-50/50 px-6 py-5">
              <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2.5">
                    <h2 className="text-xl font-semibold text-slate-900">
                      {selectedTenant?.name || effectiveSelectedTenantId}
                    </h2>
                    {selectedTenant && (
                      <Badge variant="outline" className={cn("h-6", selectedStatusMeta.className)}>
                        <span className={cn("mr-1 h-1.5 w-1.5 rounded-full", selectedStatusMeta.dotClassName)} />
                        {selectedStatusMeta.label}
                      </Badge>
                    )}
                    {selectedRoleMeta && (
                      <Badge variant="outline" className={cn("h-6", selectedRoleMeta.className)}>
                        {selectedRoleMeta.label}
                      </Badge>
                    )}
                  </div>
                  <p className="mt-1.5 font-mono text-xs text-slate-500">{effectiveSelectedTenantId}</p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    size="sm"
                    variant={activeTenantId === effectiveSelectedTenantId ? "secondary" : "default"}
                    onClick={() => switchTenant(effectiveSelectedTenantId)}
                    disabled={!effectiveSelectedTenantId || activeTenantId === effectiveSelectedTenantId}
                  >
                    <Check className="h-3.5 w-3.5" />
                    {activeTenantId === effectiveSelectedTenantId ? "当前租户" : "切换到此"}
                  </Button>
                  <SelectField
                    value={selectedTenant?.status || "active"}
                    onChange={(value) =>
                      updateTenantMutation.mutate({
                        tenantId: effectiveSelectedTenantId,
                        status: value as TenantStatus,
                      })
                    }
                    disabled={!canManageSelectedTenant || updateTenantMutation.isPending || !effectiveSelectedTenantId}
                    className="h-8 text-xs"
                  >
                    {statusOptions.map((status) => (
                      <option key={status} value={status}>
                        {statusMeta[status].label}
                      </option>
                    ))}
                  </SelectField>
                </div>
              </div>
            </div>

            <div>
              <div className="border-b border-slate-200 bg-white px-6 py-3">
                <div className="inline-flex h-10 items-center gap-1 rounded-lg bg-slate-100 p-1">
                  {tenantTabs.map((tab) => {
                    const TabIcon = tab.icon;
                    const selected = activeTab === tab.value;

                    return (
                      <button
                        key={tab.value}
                        type="button"
                        onClick={() => setActiveTab(tab.value)}
                        className={cn(
                          "inline-flex h-8 items-center gap-2 rounded-md px-4 text-sm font-medium transition",
                          selected ? "bg-white text-slate-900 shadow-sm" : "text-slate-600 hover:text-slate-900"
                        )}
                      >
                        <TabIcon className="h-4 w-4" />
                        {tab.label}
                      </button>
                    );
                  })}
                </div>
              </div>

              {activeTab === "members" && (
              <div className="space-y-6 p-6">
                {canManageSelectedTenant ? (
                  <div className="rounded-xl border border-slate-200 bg-slate-50 p-4">
                    <p className="mb-3 text-sm font-semibold text-slate-900">邀请成员</p>
                    <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_200px_auto]">
                      <div>
                        <Input
                          list="tenant-users"
                          value={memberUsername}
                          onChange={(event) => setMemberUsername(event.target.value)}
                          placeholder="输入用户名"
                          className="h-9 bg-white"
                        />
                        <datalist id="tenant-users">
                          {knownUsers.map((knownUser) => (
                            <option key={knownUser.username} value={knownUser.username} />
                          ))}
                        </datalist>
                      </div>
                      <SelectField value={memberRole} onChange={(value) => setMemberRole(value as TenantRole)} className="w-full h-9">
                        {roleOptions.map((role) => (
                          <option key={role} value={role}>
                            {roleMeta[role].label}
                          </option>
                        ))}
                      </SelectField>
                      <Button
                        size="sm"
                        className="h-9"
                        disabled={!effectiveSelectedTenantId || !memberUsername.trim() || addTenantUserMutation.isPending}
                        onClick={() => addTenantUserMutation.mutate()}
                      >
                        <UserPlus className="h-4 w-4" />
                        发送邀请
                      </Button>
                    </div>
                  </div>
                ) : (
                  <EmptyState icon={ShieldAlert} title="权限不足" description="只有租户管理员可以管理成员" className="py-12" />
                )}

                {canManageSelectedTenant && (
                  tenantUsersQuery.isLoading ? (
                    <EmptyState icon={RefreshCw} title="加载中" description="请稍候..." className="py-16" />
                  ) : members.length === 0 ? (
                    <EmptyState icon={Users} title="暂无成员" description="添加成员后会显示在这里" className="py-16" />
                  ) : (
                    <div className="overflow-hidden rounded-xl border border-slate-200">
                      <div className="overflow-x-auto">
                        <div className="min-w-[640px]">
                          <div className="grid grid-cols-[minmax(200px,1fr)_180px_180px_60px] gap-4 bg-slate-50 px-6 py-3 text-xs font-semibold uppercase tracking-wide text-slate-600">
                            <span>用户</span>
                            <span>角色</span>
                            <span>加入时间</span>
                            <span className="text-right">操作</span>
                          </div>
                          <div className="divide-y divide-slate-200 bg-white">
                            {members.map((member) => (
                              <div
                                key={`${member.tenant_id}:${member.username}`}
                                className="grid min-h-16 grid-cols-[minmax(200px,1fr)_180px_180px_60px] items-center gap-4 px-6 py-3"
                              >
                                <div className="min-w-0">
                                  <p className="truncate text-sm font-semibold text-slate-900">{member.username}</p>
                                  <p className="truncate font-mono text-xs text-slate-500">{member.tenant_id}</p>
                                </div>
                                <div>
                                  <Badge variant="outline" className={cn("h-6", roleMeta[member.role]?.className)}>
                                    {roleMeta[member.role]?.label || member.role}
                                  </Badge>
                                </div>
                                <p className="truncate text-sm text-slate-600">{formatDate(member.created_at)}</p>
                                <div className="text-right">
                                  <Button
                                    variant="ghost"
                                    size="icon"
                                    className="h-8 w-8 text-slate-400 hover:bg-rose-50 hover:text-rose-600"
                                    disabled={removeTenantUserMutation.isPending || member.username === user?.username}
                                    onClick={async () => {
                                      if (member.username === user?.username) {
                                        toast.error("不能移除自己");
                                        return;
                                      }
                                      const ok = await confirm({
                                        title: "移除租户成员",
                                        description: `确定要从 ${effectiveSelectedTenantId} 移除 ${member.username} 吗？`,
                                        confirmText: "移除",
                                        cancelText: "取消",
                                        tone: "danger",
                                      });
                                      if (ok) {
                                        removeTenantUserMutation.mutate({
                                          tenantId: effectiveSelectedTenantId,
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
                        </div>
                      </div>
                    </div>
                  )
                )}
              </div>
              )}

              {activeTab === "overview" && (
              <div className="p-6">
                <div className="grid gap-5 lg:grid-cols-2">
                  <div className="rounded-xl border border-slate-200 bg-white p-5">
                    <div className="mb-4 flex items-center gap-2.5 text-sm font-semibold text-slate-900">
                      <div className="rounded-lg bg-slate-100 p-2 text-slate-600">
                        <Clock3 className="h-4 w-4" />
                      </div>
                      生命周期
                    </div>
                    <dl className="space-y-3 text-sm">
                      <div className="flex items-center justify-between gap-4">
                        <dt className="text-slate-600">创建时间</dt>
                        <dd className="font-medium text-slate-900">{formatDate(selectedTenant?.created_at)}</dd>
                      </div>
                      <div className="flex items-center justify-between gap-4">
                        <dt className="text-slate-600">更新时间</dt>
                        <dd className="font-medium text-slate-900">{formatDate(selectedTenant?.updated_at)}</dd>
                      </div>
                      <div className="flex items-center justify-between gap-4">
                        <dt className="text-slate-600">当前上下文</dt>
                        <dd className="font-semibold text-slate-900">
                          {activeTenantId === effectiveSelectedTenantId ? (
                            <span className="text-primary">已选中</span>
                          ) : (
                            <span className="text-slate-500">未选中</span>
                          )}
                        </dd>
                      </div>
                    </dl>
                  </div>
                  <div className="rounded-xl border border-slate-200 bg-white p-5">
                    <div className="mb-4 flex items-center gap-2.5 text-sm font-semibold text-slate-900">
                      <div className="rounded-lg bg-slate-100 p-2 text-slate-600">
                        <ShieldCheck className="h-4 w-4" />
                      </div>
                      当前账号权限
                    </div>
                    <dl className="space-y-3 text-sm">
                      <div className="flex items-center justify-between gap-4">
                        <dt className="text-slate-600">租户角色</dt>
                        <dd>
                          {selectedRoleMeta ? (
                            <Badge variant="outline" className={selectedRoleMeta.className}>
                              {selectedRoleMeta.label}
                            </Badge>
                          ) : (
                            <span className="text-slate-500">未加入</span>
                          )}
                        </dd>
                      </div>
                      <div className="flex items-center justify-between gap-4">
                        <dt className="text-slate-600">成员管理</dt>
                        <dd className="font-medium text-slate-900">{canManageSelectedTenant ? "允许" : "不允许"}</dd>
                      </div>
                      <div className="flex items-center justify-between gap-4">
                        <dt className="text-slate-600">状态维护</dt>
                        <dd className="font-medium text-slate-900">{canManageSelectedTenant ? "允许" : "不允许"}</dd>
                      </div>
                    </dl>
                  </div>
                </div>
              </div>
              )}

              {activeTab === "settings" && (
              <div className="p-6">
                <div className="rounded-xl border border-slate-200 bg-white p-5">
                  <div className="mb-5 flex items-start justify-between gap-4">
                    <div>
                      <h3 className="text-base font-semibold text-slate-900">角色权限说明</h3>
                      <p className="mt-1 text-sm text-slate-600">所有权限均在当前租户范围内生效</p>
                    </div>
                    <Badge variant="outline" className="border-slate-300 bg-slate-100 font-mono text-xs text-slate-700">
                      X-Tenant-Id
                    </Badge>
                  </div>
                  <div className="grid gap-4 md:grid-cols-3">
                    {roleOptions.map((role) => {
                      const RoleIcon = roleMeta[role].icon;
                      return (
                        <div key={role} className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                          <div className="mb-3 flex items-center gap-2.5">
                            <div className="rounded-lg bg-white p-2 text-slate-600">
                              <RoleIcon className="h-4 w-4" />
                            </div>
                            <span className="text-sm font-semibold text-slate-900">{roleMeta[role].label}</span>
                          </div>
                          <p className="text-sm leading-relaxed text-slate-600">
                            {role === "tenant_admin"
                              ? "完整的租户管理权限，包括成员管理、状态维护和资源配置。"
                              : role === "tenant_rw"
                                ? "可读写租户内设备和数据，但不能管理成员和租户配置。"
                                : "只能查看租户内资源，无法进行任何写入或管理操作。"}
                          </p>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </div>
              )}
            </div>
          </section>
            );
          })()}
        </div>
      </div>
    </div>
  );
}
