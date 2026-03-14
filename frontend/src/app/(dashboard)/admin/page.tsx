"use client";

import Link from "next/link";
import { useAuth } from "@/hooks/use-auth";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { PageHeader } from "@/components/dashboard/page-header";
import { Ban, Bell, LogOut, Settings2, Shield, Users } from "lucide-react";
import type { LucideIcon } from "lucide-react";

type Entry = {
  href: string;
  title: string;
  description: string;
  icon: LucideIcon;
  minPermission: number;
};

const entries: Entry[] = [
  {
    href: "/pending",
    title: "待处理认证",
    description: "审批新设备接入请求，控制系统入口。",
    icon: Bell,
    minPermission: 2,
  },
  {
    href: "/blacklist",
    title: "黑名单管理",
    description: "管理被拒绝和已吊销设备，支持解除屏蔽。",
    icon: Ban,
    minPermission: 1,
  },
  {
    href: "/users",
    title: "用户管理",
    description: "配置账号角色与系统访问权限。",
    icon: Users,
    minPermission: 3,
  },
];

export default function AdminPage() {
  const { user, isAuthenticated, logout } = useAuth();
  const permission = user?.permission || 0;

  if (!isAuthenticated) return null;

  const availableEntries = entries.filter((entry) => permission >= entry.minPermission);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Settings2}
        title="管理控制台"
        description="按模块执行审批、风控和权限管理操作。"
        action={
          <Badge variant="outline" className="rounded-full bg-white/80 text-slate-600">
            <Shield className="mr-1 h-3.5 w-3.5 text-primary" />
            当前权限等级 {permission}
          </Badge>
        }
      />

      <div className="grid gap-4 xl:grid-cols-[minmax(0,3fr)_minmax(0,1.2fr)]">
        <div className="grid auto-rows-fr gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {availableEntries.map((entry) => (
            <Link key={entry.href} href={entry.href}>
              <Card className="elevate-hover grid h-full cursor-pointer grid-rows-[1fr_auto]">
                <CardHeader className="flex h-full flex-col border-b border-slate-200/70">
                  <div className="mb-2 inline-flex w-fit rounded-xl border border-primary/20 bg-primary/10 p-2 text-primary">
                    <entry.icon className="h-4 w-4" />
                  </div>
                  <CardTitle className="text-base font-semibold">{entry.title}</CardTitle>
                  <CardDescription className="mt-1 text-sm text-slate-500">{entry.description}</CardDescription>
                </CardHeader>
                <CardContent className="pt-4 text-xs font-medium text-slate-500">进入模块</CardContent>
              </Card>
            </Link>
          ))}
        </div>

        <Card className="xl:sticky xl:top-6 xl:h-fit">
          <CardHeader className="border-b border-slate-200/70">
            <CardTitle className="text-base font-semibold">会话操作</CardTitle>
            <CardDescription>结束当前会话并返回登录页。</CardDescription>
          </CardHeader>
          <CardContent className="pt-4">
            <Button
              variant="outline"
              className="w-full justify-center border-rose-200 text-rose-600 hover:bg-rose-50 hover:text-rose-700"
              onClick={() => logout()}
            >
              <LogOut className="h-4 w-4" />
              退出登录
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
