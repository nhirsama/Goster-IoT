"use client";

import Link from "next/link";
import { useAuth } from "@/hooks/use-auth";
import { Ban, Bell, ChevronRight, LogOut, Users } from "lucide-react";
import { Button } from "@/components/ui/button";

export default function AdminPage() {
  const { user, isAuthenticated, logout } = useAuth();
  const permission = user?.permission || 0;

  if (!isAuthenticated) return null;

  return (
    <div className="space-y-6 lg:max-w-2xl">
      <div>
        <h1 className="text-xl font-black text-slate-900">管理控制台</h1>
        <p className="text-sm text-slate-500">前后端分离版的管理入口</p>
      </div>

      <div className="glass-card rounded-2xl overflow-hidden">
        {permission >= 2 && (
          <Link
            href="/pending"
            className="flex items-center justify-between px-4 py-3 hover:bg-slate-50/70 transition-colors border-b border-slate-100"
          >
            <div className="flex items-center gap-3">
              <Bell className="h-4 w-4 text-amber-500" />
              <span className="font-semibold text-slate-900">待处理认证</span>
            </div>
            <ChevronRight className="h-4 w-4 text-slate-300" />
          </Link>
        )}

        {permission >= 1 && (
          <Link
            href="/blacklist"
            className={`flex items-center justify-between px-4 py-3 hover:bg-slate-50/70 transition-colors ${
              permission >= 3 ? "border-b border-slate-100" : ""
            }`}
          >
            <div className="flex items-center gap-3">
              <Ban className="h-4 w-4 text-rose-500" />
              <span className="font-semibold text-slate-900">黑名单管理</span>
            </div>
            <ChevronRight className="h-4 w-4 text-slate-300" />
          </Link>
        )}

        {permission >= 3 && (
          <Link
            href="/users"
            className="flex items-center justify-between px-4 py-3 hover:bg-slate-50/70 transition-colors"
          >
            <div className="flex items-center gap-3">
              <Users className="h-4 w-4 text-blue-500" />
              <span className="font-semibold text-slate-900">用户管理</span>
            </div>
            <ChevronRight className="h-4 w-4 text-slate-300" />
          </Link>
        )}
      </div>

      <Button
        variant="outline"
        className="w-full justify-center text-rose-600 border-rose-200 hover:bg-rose-50"
        onClick={() => logout()}
      >
        <LogOut className="h-4 w-4 mr-2" />
        退出登录
      </Button>
    </div>
  );
}
