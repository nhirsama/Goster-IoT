"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, getActiveTenantId, setActiveTenantId } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { getPermissionRoleLabel } from "@/lib/dashboard-meta";
import { queryKeys } from "@/lib/query-keys";
import { EmptyState } from "@/components/dashboard/empty-state";
import { Button } from "@/components/ui/button";
import {
  Bell,
  Home,
  Layers,
  LogOut,
  Network,
  Shield,
  Wifi,
  Ban,
  RefreshCw,
  Building2,
  Check,
  ChevronsUpDown,
  Settings,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useAuth } from "@/hooks/use-auth";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

type TenantStatus = components["schemas"]["TenantStatus"];
type Tenant = components["schemas"]["Tenant"];
type TenantListData = components["schemas"]["TenantListResponse"]["data"];

type NavEntry = {
  href: string;
  label: string;
  icon: LucideIcon;
  minPermission: number;
};

const managementEntries: NavEntry[] = [
  { href: "/invitations", label: "租户邀请", icon: Bell, minPermission: 1 },
  { href: "/pending", label: "待处理认证", icon: Bell, minPermission: 2 },
  { href: "/blacklist", label: "黑名单", icon: Ban, minPermission: 1 },
  { href: "/tenants", label: "租户管理", icon: Building2, minPermission: 3 },
];

function clsx(...values: Array<string | false>) {
  return values.filter(Boolean).join(" ");
}

export default function DashboardShell({
  children,
}: {
  children: ReactNode;
}) {
  const queryClient = useQueryClient();
  const router = useRouter();
  const pathname = usePathname();
  const { user, isAuthenticated, isLoading: authLoading } = useAuth();
  const permission = user?.permission || 0;
  const tenantRoles = user?.tenant_roles || {};

  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      router.replace("/login");
    }
  }, [authLoading, isAuthenticated, router]);

  const logoutMutation = useMutation({
    mutationFn: () => api.post("/api/v1/auth/logout"),
    onSuccess: () => {
      queryClient.setQueryData(queryKeys.authMe, null);
      router.push("/login");
    },
  });

  useQuery({
    queryKey: queryKeys.devicesByStatus("authenticated"),
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "authenticated" }),
    enabled: permission > 0,
    refetchInterval: 10000,
  });

  const { data: tenantData } = useQuery({
    queryKey: queryKeys.tenants,
    queryFn: () => api.get<TenantListData>("/api/v1/tenants"),
    enabled: permission > 0,
    retry: false,
  });

  const roleTenantItems: Tenant[] = Object.entries(tenantRoles).map(([id, role]) => ({
    id,
    name: id === "tenant_legacy" ? "legacy" : id,
    status: "active" as TenantStatus,
    role,
    created_at: "",
  }));
  const tenantItems = tenantData?.items?.length ? tenantData.items : roleTenantItems;
  const storedTenantId = getActiveTenantId();
  const preferredTenantId = storedTenantId || user?.active_tenant || tenantItems[0]?.id || "tenant_legacy";
  const activeTenant = tenantItems.find((tenant) => tenant.id === preferredTenantId) || tenantItems[0];
  const activeTenantId = activeTenant?.id || preferredTenantId;

  useEffect(() => {
    if (activeTenantId && storedTenantId !== activeTenantId) {
      setActiveTenantId(activeTenantId);
    }
  }, [activeTenantId, storedTenantId]);

  const handleTenantSwitch = (tenantId: string) => {
    setActiveTenantId(tenantId);
    queryClient.invalidateQueries();
    router.refresh();
  };

  if (authLoading) {
    return <EmptyState icon={RefreshCw} title="正在校验会话状态" description="请稍候..." className="min-h-screen py-24" />;
  }

  if (!isAuthenticated || !user) {
    return <EmptyState icon={Shield} title="需要登录" description="正在跳转到登录页。" className="min-h-screen py-24" />;
  }

  if (user?.permission === 0) {
    return (
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="glass-card w-full max-w-lg rounded-3xl p-8 text-center">
          <div className="mx-auto mb-4 w-fit rounded-2xl bg-slate-100 p-3 text-slate-500">
            <Shield className="h-6 w-6" />
          </div>
          <h1 className="text-2xl font-semibold text-slate-900">账户待审核</h1>
          <p className="mt-2 text-sm text-slate-500">您的账户已注册成功，等待管理员分配权限后即可使用系统功能。</p>
          <Button
            variant="outline"
            className="mt-6 border-rose-200 text-rose-600 hover:bg-rose-50"
            onClick={() => logoutMutation.mutate()}
            disabled={logoutMutation.isPending}
          >
            退出登录
          </Button>
        </div>
      </div>
    );
  }

  const availableManagementEntries = managementEntries.filter((entry) => permission >= entry.minPermission);
  const managementDefaultHref = availableManagementEntries[0]?.href || "/blacklist";
  const mobileHomeActive = pathname === "/";
  const mobileDevicesActive = pathname === "/devices" || pathname === "/devices/detail";
  const mobileAdminActive = pathname === "/admin" || availableManagementEntries.some((entry) => pathname === entry.href);

  return (
    <div className="relative min-h-screen bg-transparent text-slate-900">
      <header className="fixed left-0 right-0 top-0 z-40 border-b border-slate-200/70 bg-white/85 backdrop-blur lg:hidden">
        <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4">
          <Link href="/" className="flex items-center gap-2">
            <div className="rounded-lg bg-primary p-1.5 text-primary-foreground shadow-sm">
              <Network className="h-4 w-4" />
            </div>
            <span className="text-sm font-semibold text-slate-900">Goster IoT</span>
          </Link>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            onClick={() => logoutMutation.mutate()}
            disabled={logoutMutation.isPending}
          >
            <LogOut className="h-4 w-4 text-slate-500" />
          </Button>
        </div>
      </header>

      <aside className="fixed inset-y-0 left-0 z-30 hidden h-screen shrink-0 border-r border-slate-200/70 bg-white/70 backdrop-blur-xl lg:flex lg:w-64 lg:flex-col xl:w-72">
        <div className="border-b border-slate-200/70 px-4 py-4 shrink-0">
          <Link href="/" className="flex items-center gap-2.5">
            <div className="rounded-xl bg-primary p-2 text-primary-foreground shadow-sm">
              <Network className="h-5 w-5" />
            </div>
            <div className="min-w-0">
              <p className="text-base font-semibold text-slate-900 truncate">Goster IoT</p>
              <p className="text-xs text-slate-500 truncate">{activeTenant?.name || "默认租户"}</p>
            </div>
          </Link>
        </div>

        <div className="flex-1 overflow-y-auto px-3 py-3 min-h-0">
          <nav className="space-y-0.5">
            <Link
              href="/"
              className={clsx(
                "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition",
                pathname === "/"
                  ? "bg-primary/10 text-primary"
                  : "text-slate-700 hover:bg-slate-100"
              )}
            >
              <Home className="h-4 w-4" />
              概览
            </Link>
            <Link
              href="/devices"
              className={clsx(
                "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition",
                pathname === "/devices" || pathname === "/devices/detail"
                  ? "bg-primary/10 text-primary"
                  : "text-slate-700 hover:bg-slate-100"
              )}
            >
              <Wifi className="h-4 w-4" />
              设备管理
            </Link>
            {availableManagementEntries.map((entry) => {
              const active = pathname === entry.href;
              return (
                <Link
                  key={entry.href}
                  href={entry.href}
                  className={clsx(
                    "flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition",
                    active
                      ? "bg-primary/10 text-primary"
                      : "text-slate-700 hover:bg-slate-100"
                  )}
                >
                  <entry.icon className="h-4 w-4" />
                  {entry.label}
                </Link>
              );
            })}
          </nav>
        </div>

        <div className="border-t border-slate-200/70 px-3 py-3 shrink-0">
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <button className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left transition hover:bg-slate-100 min-w-0" />
              }
            >
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-slate-200 text-slate-700 font-medium text-sm">
                {user?.username?.charAt(0).toUpperCase() || "U"}
              </div>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-slate-900">{user?.username}</p>
                <p className="truncate text-xs text-slate-500">{getPermissionRoleLabel(permission)}</p>
              </div>
              <ChevronsUpDown className="h-4 w-4 shrink-0 text-slate-400" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" side="top" className="w-64">
              <DropdownMenuGroup>
                <div className="px-2 py-1.5">
                  <p className="text-sm font-medium text-slate-900">{user?.username}</p>
                  <p className="text-xs text-slate-500">{getPermissionRoleLabel(permission)}</p>
                </div>
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuLabel>切换租户</DropdownMenuLabel>
                {tenantItems.length === 0 ? (
                  <DropdownMenuItem disabled>暂无可用租户</DropdownMenuItem>
                ) : (
                  tenantItems.map((tenant) => (
                    <DropdownMenuItem
                      key={tenant.id}
                      className="cursor-pointer"
                      onClick={() => handleTenantSwitch(tenant.id)}
                    >
                      <Building2 className="h-4 w-4" />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-medium">{tenant.name || tenant.id}</span>
                        <span className="block truncate font-mono text-[11px] text-slate-500">{tenant.id}</span>
                      </span>
                      {tenant.id === activeTenantId ? <Check className="h-4 w-4 text-primary" /> : null}
                    </DropdownMenuItem>
                  ))
                )}
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuItem className="cursor-pointer" onClick={() => router.push("/settings")}>
                  <Settings className="h-4 w-4" />
                  设置
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuItem
                  className="cursor-pointer text-rose-600 focus:text-rose-600"
                  onClick={() => logoutMutation.mutate()}
                  disabled={logoutMutation.isPending}
                >
                  <LogOut className="h-4 w-4" />
                  退出登录
                </DropdownMenuItem>
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </aside>

      <main className="relative min-h-screen pb-20 pt-16 lg:pb-0 lg:pl-64 lg:pt-0 xl:pl-72">
        <div className="mx-auto w-full max-w-7xl p-4 lg:p-8">{children}</div>
      </main>

      <nav className="fixed bottom-0 left-0 right-0 z-40 border-t border-slate-200/70 bg-white/90 backdrop-blur lg:hidden">
        <div className="grid h-16 grid-cols-3">
          <Link
            href="/"
            className={clsx(
              "flex flex-col items-center justify-center gap-1 text-xs font-medium",
              mobileHomeActive ? "text-primary" : "text-slate-500"
            )}
          >
            <Home className="h-4 w-4" />
            首页
          </Link>
          <Link
            href="/devices"
            className={clsx(
              "flex flex-col items-center justify-center gap-1 text-xs font-medium",
              mobileDevicesActive ? "text-primary" : "text-slate-500"
            )}
          >
            <Wifi className="h-4 w-4" />
            设备
          </Link>
          <Link
            href={managementDefaultHref}
            className={clsx(
              "flex flex-col items-center justify-center gap-1 text-xs font-medium",
              mobileAdminActive ? "text-primary" : "text-slate-500"
            )}
          >
            <Layers className="h-4 w-4" />
            管理
          </Link>
        </div>
      </nav>
    </div>
  );
}
