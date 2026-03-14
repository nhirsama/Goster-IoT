"use client";

import { useAuth } from "@/hooks/use-auth";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { components } from "@/lib/api-types";
import { useRouter, usePathname } from "next/navigation";
import { useEffect } from "react";
import Link from "next/link";
import {
  Network,
  Monitor,
  Bell,
  Ban,
  Users,
  LogOut,
  ChevronRight,
  Server,
  Fingerprint,
  Home,
  Layers,
  Menu,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

type DeviceRecord = components["schemas"]["DeviceRecord"];

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { user, isAuthenticated, isLoading: authLoading, logout } = useAuth();
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      router.push("/login");
    }
  }, [isAuthenticated, authLoading, router]);

  const { data: deviceData } = useQuery({
    queryKey: ["devices", "authenticated"],
    queryFn: () => api.get<components["schemas"]["DeviceListData"]>("/api/v1/devices", { status: "authenticated" }),
    enabled: isAuthenticated,
    refetchInterval: 10000, // 与原版一致：10秒刷新设备列表
  });

  if (authLoading || !isAuthenticated) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-slate-900">
        <div className="h-8 w-8 rounded-full border-4 border-blue-500 border-t-transparent animate-spin"></div>
      </div>
    );
  }

  // 零权限处理 (与原版逻辑一致)
  if (user?.permission === 0) {
    return (
      <div className="container flex flex-col items-center justify-center min-h-screen text-center p-4 bg-slate-50 mx-auto">
        <Ban className="h-20 w-20 text-slate-400 mb-4" />
        <h1 className="text-4xl font-black text-slate-900 mb-2 tracking-tight">账户待审核</h1>
        <p className="text-lg text-slate-500 mb-8">您的账户注册成功，但暂时没有任何权限。</p>
        <Button variant="outline" className="text-red-600 border-red-200 hover:bg-red-50" onClick={() => logout()}>
          退出登录
        </Button>
      </div>
    );
  }

  const devices = deviceData?.items || [];
  const permission = user?.permission || 0;
  const mobileHomeActive = pathname === "/";
  const mobileDevicesActive = pathname === "/devices" || pathname.startsWith("/devices/");
  const mobileAdminActive =
    pathname === "/admin" || pathname === "/pending" || pathname === "/blacklist" || pathname === "/users";

  return (
    <div className="flex h-screen bg-[#f1f5f9] font-sans text-slate-900 overflow-hidden">
      <header className="lg:hidden fixed top-0 left-0 right-0 z-30 h-14 bg-white/95 backdrop-blur border-b border-slate-200">
        <div className="h-full px-4 flex items-center justify-between">
          <Link href="/" className="flex items-center gap-2">
            <div className="bg-blue-600 p-1.5 rounded-lg shadow-sm">
              <Network className="h-4 w-4 text-white" />
            </div>
            <span className="text-base font-black text-slate-900">Goster IoT</span>
          </Link>
          <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => logout()}>
            <LogOut className="h-4 w-4 text-slate-500" />
          </Button>
        </div>
      </header>

      {/* 桌面端侧边栏 - 主流 IoT 风格：深色高对比度 */}
      <aside className="hidden lg:flex flex-col w-[280px] bg-[#0f172a] text-slate-300 shadow-2xl z-20">
        <div className="p-6 border-b border-slate-800/50">
          <Link href="/" className="flex items-center gap-3 text-decoration-none group">
            <div className="bg-blue-600 p-2 rounded-xl shadow-lg shadow-blue-900/20 group-hover:bg-blue-500 transition-colors">
              <Network className="h-6 w-6 text-white" />
            </div>
            <span className="text-2xl font-black tracking-tight text-white">Goster IoT</span>
          </Link>
        </div>

        <div className="flex-1 overflow-y-auto px-4 py-6 custom-scrollbar [&::-webkit-scrollbar]:w-1 [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:bg-slate-700">
          {/* 设备监控区 */}
          <div className="mb-8">
            <div className="flex items-center gap-2 px-2 mb-4 text-[11px] font-black text-slate-500 uppercase tracking-widest">
              <Monitor className="h-3.5 w-3.5" />
              <span>设备监控</span>
            </div>
            <div className="space-y-1.5">
              {devices.length === 0 ? (
                <div className="px-4 py-8 text-center bg-slate-800/30 rounded-xl border border-dashed border-slate-700/50">
                  <Server className="h-6 w-6 text-slate-600 mx-auto mb-2" />
                  <p className="text-xs font-medium text-slate-500">暂无在线设备</p>
                </div>
              ) : (
                devices.map((device: DeviceRecord) => {
                  const isActive = pathname === `/devices/${device.uuid}`;
                  return (
                    <Link
                      key={device.uuid}
                      href={`/devices/${device.uuid}`}
                      className={`flex items-center justify-between p-3 rounded-xl transition-all border ${isActive ? 'bg-blue-600/10 border-blue-500/30 shadow-inner' : 'border-transparent hover:bg-slate-800/50 hover:border-slate-700/50'}`}
                    >
                      <div className="flex items-center gap-3 overflow-hidden">
                        {/* 状态点 - 发光效果 */}
                        <div className="relative flex-shrink-0">
                           <span className={`absolute inset-0 rounded-full blur-sm opacity-50 ${device.runtime?.status === 1 ? 'bg-emerald-400' : device.runtime?.status === 2 ? 'bg-amber-400' : 'bg-slate-500'}`}></span>
                           <span className={`relative block h-2.5 w-2.5 rounded-full ${device.runtime?.status === 1 ? 'bg-emerald-500' : device.runtime?.status === 2 ? 'bg-amber-500' : 'bg-slate-600'}`}></span>
                        </div>
                        <div className="truncate">
                          <p className={`text-sm font-bold truncate ${isActive ? 'text-blue-400' : 'text-slate-200'}`}>{device.meta.name}</p>
                          <div className="flex items-center gap-1 mt-0.5">
                            <Fingerprint className="h-3 w-3 text-slate-600" />
                            <p className="text-[10px] font-mono text-slate-500 truncate">{device.uuid.split("-")[0]}</p>
                          </div>
                        </div>
                      </div>
                      <ChevronRight className={`h-4 w-4 flex-shrink-0 transition-transform ${isActive ? 'text-blue-500 translate-x-1' : 'text-slate-700 opacity-0 group-hover:opacity-100'}`} />
                    </Link>
                  );
                })
              )}
            </div>
          </div>

          {/* 管理区 */}
          <div className="mb-8">
            <div className="flex items-center gap-2 px-2 mb-4 text-[11px] font-black text-slate-500 uppercase tracking-widest">
              <span>管理控制台</span>
            </div>
            <div className="space-y-1.5">
              {permission >= 2 && (
                <Link href="/pending" className={`flex items-center justify-between p-3 rounded-xl transition-all border ${pathname === '/pending' ? 'bg-amber-500/10 border-amber-500/30 text-amber-400' : 'border-transparent text-slate-400 hover:bg-slate-800/50 hover:text-slate-200'}`}>
                  <div className="flex items-center gap-3">
                    <Bell className="h-4 w-4" />
                    <span className="text-sm font-bold">待处理认证</span>
                  </div>
                  <ChevronRight className="h-4 w-4 opacity-30" />
                </Link>
              )}
              {permission >= 1 && (
                <Link href="/blacklist" className={`flex items-center justify-between p-3 rounded-xl transition-all border ${pathname === '/blacklist' ? 'bg-rose-500/10 border-rose-500/30 text-rose-400' : 'border-transparent text-slate-400 hover:bg-slate-800/50 hover:text-slate-200'}`}>
                   <div className="flex items-center gap-3">
                    <Ban className="h-4 w-4" />
                    <span className="text-sm font-bold">黑名单</span>
                  </div>
                  <ChevronRight className="h-4 w-4 opacity-30" />
                </Link>
              )}
            </div>
          </div>

          {/* 系统区 */}
          {permission >= 3 && (
            <div className="mb-8">
              <div className="flex items-center gap-2 px-2 mb-4 text-[11px] font-black text-slate-500 uppercase tracking-widest">
                <span>系统设置</span>
              </div>
              <div className="space-y-1.5">
                <Link href="/users" className={`flex items-center justify-between p-3 rounded-xl transition-all border ${pathname === '/users' ? 'bg-purple-500/10 border-purple-500/30 text-purple-400' : 'border-transparent text-slate-400 hover:bg-slate-800/50 hover:text-slate-200'}`}>
                   <div className="flex items-center gap-3">
                    <Users className="h-4 w-4" />
                    <span className="text-sm font-bold">用户管理</span>
                  </div>
                  <ChevronRight className="h-4 w-4 opacity-30" />
                </Link>
              </div>
            </div>
          )}
        </div>

        <div className="p-4 border-t border-slate-800/50 bg-[#0b1120]">
          <div className="flex items-center justify-between mb-4 px-2">
            <div className="flex flex-col">
              <span className="text-sm font-bold text-white">{user?.username}</span>
              <span className="text-[10px] text-slate-500 uppercase font-mono tracking-wider">
                {permission === 3 ? 'Admin' : permission === 2 ? 'ReadWrite' : 'ReadOnly'}
              </span>
            </div>
            <Badge variant="outline" className="border-slate-700 text-slate-400 bg-slate-800/50">在线</Badge>
          </div>
          <Button variant="ghost" className="w-full justify-start text-rose-400 hover:text-rose-300 hover:bg-rose-500/10 transition-colors rounded-xl" onClick={() => logout()}>
            <LogOut className="h-4 w-4 mr-3" />
            <span className="font-bold">退出系统</span>
          </Button>
        </div>
      </aside>

      {/* 桌面端主内容区 */}
      <main className="flex-1 overflow-y-auto bg-[#f8fafc] relative pt-14 lg:pt-0 pb-20 lg:pb-0">
         {/* 微妙的网格背景，增加科技感 */}
         <div className="absolute inset-0 bg-[linear-gradient(to_right,#e2e8f0_1px,transparent_1px),linear-gradient(to_bottom,#e2e8f0_1px,transparent_1px)] bg-[size:3rem_3rem] [mask-image:radial-gradient(ellipse_60%_60%_at_50%_0%,#000_70%,transparent_100%)] pointer-events-none opacity-50"></div>
         <div className="relative z-10 w-full h-full max-w-7xl mx-auto p-4 lg:p-8">
            {children}
         </div>
      </main>

      <nav className="lg:hidden fixed bottom-0 left-0 right-0 z-30 bg-white/95 backdrop-blur border-t border-slate-200">
        <div className="grid grid-cols-3 h-16">
          <Link
            href="/"
            className={`flex flex-col items-center justify-center gap-1 text-xs font-bold transition-colors ${
              mobileHomeActive ? "text-blue-600" : "text-slate-500"
            }`}
          >
            <Home className="h-4 w-4" />
            首页
          </Link>
          <Link
            href="/devices"
            className={`flex flex-col items-center justify-center gap-1 text-xs font-bold transition-colors ${
              mobileDevicesActive ? "text-blue-600" : "text-slate-500"
            }`}
          >
            <Menu className="h-4 w-4" />
            设备
          </Link>
          <Link
            href="/admin"
            className={`flex flex-col items-center justify-center gap-1 text-xs font-bold transition-colors ${
              mobileAdminActive ? "text-blue-600" : "text-slate-500"
            }`}
          >
            <Layers className="h-4 w-4" />
            管理
          </Link>
        </div>
      </nav>
    </div>
  );
}
