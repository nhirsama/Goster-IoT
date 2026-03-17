"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { getPermissionRoleLabel } from "@/lib/dashboard-meta";
import { queryKeys } from "@/lib/query-keys";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Bell,
  Fingerprint,
  Home,
  Layers,
  LogOut,
  Network,
  Shield,
  Users,
  Wifi,
  Ban,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

type DeviceRecord = components["schemas"]["DeviceRecord"];
type AuthSession = components["schemas"]["AuthSession"];

type NavEntry = {
  href: string;
  label: string;
  icon: LucideIcon;
  minPermission: number;
};

const managementEntries: NavEntry[] = [
  { href: "/pending", label: "待处理认证", icon: Bell, minPermission: 2 },
  { href: "/blacklist", label: "黑名单", icon: Ban, minPermission: 1 },
  { href: "/users", label: "用户管理", icon: Users, minPermission: 3 },
];

function clsx(...values: Array<string | false>) {
  return values.filter(Boolean).join(" ");
}

export default function DashboardShell({
  children,
  initialUser,
}: {
  children: ReactNode;
  initialUser: AuthSession;
}) {
  const queryClient = useQueryClient();
  const router = useRouter();
  const pathname = usePathname();
  const user = initialUser;
  const permission = user?.permission || 0;

  const logoutMutation = useMutation({
    mutationFn: () => api.post("/api/v1/auth/logout"),
    onSuccess: () => {
      queryClient.setQueryData(queryKeys.authMe, null);
      router.push("/login");
      router.refresh();
    },
  });

  const { data: deviceData } = useQuery({
    queryKey: queryKeys.devicesByStatus("authenticated"),
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "authenticated" }),
    enabled: permission > 0,
    refetchInterval: 10000,
  });

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

  const devices = deviceData?.items || [];
  const mobileHomeActive = pathname === "/";
  const mobileDevicesActive = pathname === "/devices" || pathname.startsWith("/devices/");
  const mobileAdminActive =
    pathname === "/admin" || pathname === "/pending" || pathname === "/blacklist" || pathname === "/users";

  return (
    <div className="relative flex min-h-screen overflow-hidden bg-transparent text-slate-900">
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

      <aside className="hidden w-[300px] shrink-0 border-r border-slate-200/70 bg-white/70 backdrop-blur-xl lg:flex lg:flex-col">
        <div className="border-b border-slate-200/70 px-5 py-5">
          <Link href="/" className="flex items-center gap-2.5">
            <div className="rounded-xl bg-primary p-2 text-primary-foreground shadow-sm">
              <Network className="h-5 w-5" />
            </div>
            <div>
              <p className="text-base font-semibold text-slate-900">Goster IoT</p>
              <p className="text-xs text-slate-500">Device Control Center</p>
            </div>
          </Link>
        </div>

        <div className="flex-1 space-y-6 overflow-y-auto px-4 py-5">
          <section className="space-y-3">
            <div className="px-2 text-[11px] font-semibold uppercase tracking-wider text-slate-400">在线设备</div>
            <div className="space-y-1.5">
              {devices.length === 0 ? (
                <div className="rounded-2xl border border-dashed border-slate-200 bg-white/70 px-4 py-8 text-center">
                  <Wifi className="mx-auto h-5 w-5 text-slate-300" />
                  <p className="mt-2 text-xs text-slate-400">暂无在线设备</p>
                </div>
              ) : (
                devices.map((device: DeviceRecord) => {
                  const active = pathname === `/devices/${device.uuid}`;
                  return (
                    <Link
                      key={device.uuid}
                      href={`/devices/${device.uuid}`}
                      className={clsx(
                        "block rounded-xl border px-3 py-2.5 transition",
                        active
                          ? "border-primary/30 bg-primary/10"
                          : "border-transparent bg-white/80 hover:border-slate-200 hover:bg-slate-50"
                      )}
                    >
                      <div className="flex items-center justify-between gap-3">
                        <div className="min-w-0">
                          <p className={clsx("truncate text-sm font-medium", active ? "text-primary" : "text-slate-800")}>
                            {device.meta.name}
                          </p>
                          <div className="mt-1 flex items-center gap-1 text-xs text-slate-500">
                            <Fingerprint className="h-3 w-3" />
                            <span className="truncate font-mono">{device.uuid.split("-")[0]}</span>
                          </div>
                        </div>
                        <span
                          className={clsx(
                            "h-2.5 w-2.5 shrink-0 rounded-full",
                            device.runtime?.status === 1
                              ? "bg-emerald-500"
                              : device.runtime?.status === 2
                                ? "bg-amber-500"
                                : "bg-slate-300"
                          )}
                        />
                      </div>
                    </Link>
                  );
                })
              )}
            </div>
          </section>

          <section className="space-y-3">
            <div className="px-2 text-[11px] font-semibold uppercase tracking-wider text-slate-400">管理模块</div>
            <div className="space-y-1.5">
              <Link
                href="/admin"
                className={clsx(
                  "flex items-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-medium transition",
                  pathname === "/admin"
                    ? "border-primary/30 bg-primary/10 text-primary"
                    : "border-transparent bg-white/80 text-slate-700 hover:border-slate-200 hover:bg-slate-50"
                )}
              >
                <Layers className="h-4 w-4" />
                管理控制台
              </Link>

              {managementEntries
                .filter((entry) => permission >= entry.minPermission)
                .map((entry) => {
                  const active = pathname === entry.href;
                  return (
                    <Link
                      key={entry.href}
                      href={entry.href}
                      className={clsx(
                        "flex items-center gap-2 rounded-xl border px-3 py-2.5 text-sm font-medium transition",
                        active
                          ? "border-primary/30 bg-primary/10 text-primary"
                          : "border-transparent bg-white/80 text-slate-700 hover:border-slate-200 hover:bg-slate-50"
                      )}
                    >
                      <entry.icon className="h-4 w-4" />
                      {entry.label}
                    </Link>
                  );
                })}
            </div>
          </section>
        </div>

        <div className="border-t border-slate-200/70 px-4 py-4">
          <div className="mb-3 flex items-center justify-between rounded-xl border border-slate-200 bg-white/80 px-3 py-2">
            <div>
              <p className="text-sm font-medium text-slate-900">{user?.username}</p>
              <p className="text-xs text-slate-500">{getPermissionRoleLabel(permission)}</p>
            </div>
            <Badge variant="outline" className="rounded-full bg-slate-50 text-slate-600">
              在线
            </Badge>
          </div>
          <Button
            variant="outline"
            className="w-full justify-start border-rose-200 text-rose-600 hover:bg-rose-50 hover:text-rose-700"
            onClick={() => logoutMutation.mutate()}
            disabled={logoutMutation.isPending}
          >
            <LogOut className="h-4 w-4" />
            退出系统
          </Button>
        </div>
      </aside>

      <main className="relative flex-1 overflow-y-auto pb-20 pt-16 lg:pb-0 lg:pt-0">
        <div className="mx-auto h-full w-full max-w-7xl p-4 lg:p-8">{children}</div>
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
            href="/admin"
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
