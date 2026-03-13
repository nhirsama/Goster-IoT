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
  Wrench,
  Bell,
  Ban,
  UserCog,
  Users,
  LogOut,
  ChevronRight,
  Home,
  Server,
} from "lucide-react";
import { Button } from "@/components/ui/button";

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
    queryFn: () => api.get("/api/v1/devices", { status: "authenticated" }),
    enabled: isAuthenticated,
  });

  if (authLoading || !isAuthenticated) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-slate-50">
        <div className="h-8 w-8 rounded-full border-4 border-blue-600 border-t-transparent animate-spin"></div>
      </div>
    );
  }

  if (user?.permission === 0) {
    return (
      <div className="container flex flex-col items-center justify-center min-h-screen text-center p-4 bg-slate-50 mx-auto">
        <UserCog className="h-20 w-20 text-slate-400 mb-4" />
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

  return (
    <div className="flex h-screen bg-[#f8fafc] font-sans text-slate-900 overflow-hidden">
      {/* Desktop Sidebar */}
      <aside className="hidden lg:flex flex-col w-72 bg-white border-r border-slate-200 shadow-sm z-20">
        <div className="p-6">
          <Link href="/" className="flex items-center gap-3 text-decoration-none group">
            <div className="bg-blue-600 p-2 rounded-xl shadow-md shadow-blue-200 group-hover:bg-blue-700 transition-colors">
              <Network className="h-6 w-6 text-white" />
            </div>
            <span className="text-2xl font-black tracking-tight text-slate-900">Goster IoT</span>
          </Link>
        </div>

        <div className="flex-1 overflow-y-auto px-4 custom-scrollbar">
          <div className="mb-6">
            <div className="flex items-center gap-2 px-2 mb-3 text-xs font-bold text-slate-400 uppercase tracking-widest">
              <Monitor className="h-4 w-4" />
              <span>设备监控</span>
            </div>
            <div className="space-y-1">
              {devices.length === 0 ? (
                <div className="px-4 py-6 text-center bg-slate-50 rounded-xl border border-dashed border-slate-200">
                  <Server className="h-8 w-8 text-slate-300 mx-auto mb-2" />
                  <p className="text-sm font-medium text-slate-500">暂无在线设备</p>
                </div>
              ) : (
                devices.map((device: DeviceRecord) => (
                  <Link
                    key={device.uuid}
                    href={`/devices/${device.uuid}`}
                    className={`flex items-center justify-between p-3 rounded-xl transition-all ${pathname === `/devices/${device.uuid}` ? 'bg-blue-50 border border-blue-100 shadow-sm' : 'hover:bg-slate-50 border border-transparent'}`}
                  >
                    <div className="flex items-center gap-3 overflow-hidden">
                      <span className={`h-2.5 w-2.5 rounded-full flex-shrink-0 ${device.runtime?.status === 1 ? 'bg-emerald-500' : device.runtime?.status === 2 ? 'bg-amber-500' : 'bg-slate-300'}`}></span>
                      <div className="truncate">
                        <p className={`text-sm font-bold truncate ${pathname === `/devices/${device.uuid}` ? 'text-blue-700' : 'text-slate-700'}`}>{device.meta.name}</p>
                        <p className="text-[10px] font-mono text-slate-400 truncate">{device.uuid.split("-")[0]}...</p>
                      </div>
                    </div>
                    <ChevronRight className={`h-4 w-4 flex-shrink-0 ${pathname === `/devices/${device.uuid}` ? 'text-blue-400' : 'text-slate-300 opacity-0 group-hover:opacity-100'}`} />
                  </Link>
                ))
              )}
            </div>
          </div>

          <div className="mb-6">
            <div className="flex items-center gap-2 px-2 mb-3 text-xs font-bold text-slate-400 uppercase tracking-widest">
              <Wrench className="h-4 w-4" />
              <span>管理</span>
            </div>
            <div className="space-y-1">
              {permission >= 2 && (
                <Link href="/pending" className={`flex items-center gap-3 p-3 rounded-xl transition-all ${pathname === '/pending' ? 'bg-amber-50 border border-amber-100 text-amber-700 shadow-sm' : 'text-slate-600 hover:bg-slate-50 border border-transparent'}`}>
                  <Bell className="h-4 w-4" />
                  <span className="text-sm font-bold">待处理认证</span>
                </Link>
              )}
              {permission >= 1 && (
                <Link href="/blacklist" className={`flex items-center gap-3 p-3 rounded-xl transition-all ${pathname === '/blacklist' ? 'bg-rose-50 border border-rose-100 text-rose-700 shadow-sm' : 'text-slate-600 hover:bg-slate-50 border border-transparent'}`}>
                  <Ban className="h-4 w-4" />
                  <span className="text-sm font-bold">黑名单</span>
                </Link>
              )}
            </div>
          </div>

          {permission >= 3 && (
            <div className="mb-6">
              <div className="flex items-center gap-2 px-2 mb-3 text-xs font-bold text-slate-400 uppercase tracking-widest">
                <UserCog className="h-4 w-4" />
                <span>系统</span>
              </div>
              <div className="space-y-1">
                <Link href="/users" className={`flex items-center gap-3 p-3 rounded-xl transition-all ${pathname === '/users' ? 'bg-purple-50 border border-purple-100 text-purple-700 shadow-sm' : 'text-slate-600 hover:bg-slate-50 border border-transparent'}`}>
                  <Users className="h-4 w-4" />
                  <span className="text-sm font-bold">用户管理</span>
                </Link>
              </div>
            </div>
          )}
        </div>

        <div className="p-4 border-t border-slate-100 mt-auto">
          <Button variant="outline" className="w-full justify-center text-slate-500 border-slate-200 hover:bg-rose-50 hover:text-rose-600 transition-colors rounded-xl" onClick={() => logout()}>
            <LogOut className="h-4 w-4 mr-2" />
            退出登录
          </Button>
          <div className="text-center mt-4 space-y-1">
            <p className="text-xs font-bold text-slate-700">{user?.username}</p>
          </div>
        </div>
      </aside>

      {/* Mobile Header */}
      <div className="lg:hidden flex flex-col flex-1 w-full h-full">
        <header className="flex-shrink-0 h-16 bg-white/80 backdrop-blur-md border-b border-slate-200 flex items-center justify-between px-4 z-20">
          <Link href="/" className="flex items-center gap-2">
            <Network className="h-6 w-6 text-blue-600" />
            <span className="text-lg font-black tracking-tight text-slate-900">Goster IoT</span>
          </Link>
          <Button variant="ghost" size="icon" onClick={() => logout()} className="text-slate-400 hover:text-rose-600">
             <LogOut className="h-5 w-5" />
          </Button>
        </header>

        {/* Main Content (Mobile) */}
        <main className="flex-1 overflow-y-auto bg-[#f8fafc] relative">
          <div className="absolute inset-0 bg-[radial-gradient(#e2e8f0_1px,transparent_1px)] [background-size:24px_24px] pointer-events-none"></div>
          <div className="relative z-10 w-full h-full p-4">
            {children}
          </div>
        </main>

        {/* Mobile Bottom Nav */}
        <nav className="flex-shrink-0 h-16 bg-white border-t border-slate-200 flex items-center justify-around px-2 pb-safe z-20">
          <Link href="/" className={`flex flex-col items-center justify-center w-full h-full gap-1 ${pathname === '/' ? 'text-blue-600' : 'text-slate-400'}`}>
            <Home className="h-5 w-5" />
            <span className="text-[10px] font-bold">首页</span>
          </Link>
        </nav>
      </div>

      {/* Main Content (Desktop) */}
      <main className="hidden lg:block flex-1 overflow-y-auto bg-[#f8fafc] relative">
         <div className="absolute inset-0 bg-[radial-gradient(#e2e8f0_1px,transparent_1px)] [background-size:24px_24px] pointer-events-none"></div>
         <div className="relative z-10 w-full h-full max-w-6xl mx-auto p-8">
            {children}
         </div>
      </main>
    </div>
  );
}