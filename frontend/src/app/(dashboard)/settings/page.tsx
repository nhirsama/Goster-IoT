"use client";

import { useRouter } from "next/navigation";
import { useAuth } from "@/hooks/use-auth";
import { User, Shield } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export default function SettingsPage() {
  const router = useRouter();
  const { user } = useAuth();
  const permission = user?.permission || 0;

  return (
    <div className="mx-auto max-w-4xl space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-slate-900">设置</h1>
        <p className="mt-1 text-sm text-slate-500">管理您的账户和租户设置</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <User className="h-5 w-5" />
            账户信息
          </CardTitle>
          <CardDescription>您的个人账户信息</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 px-4 py-3">
            <div>
              <p className="text-sm font-medium text-slate-900">用户名</p>
              <p className="text-sm text-slate-500">{user?.username}</p>
            </div>
          </div>
          <div className="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 px-4 py-3">
            <div>
              <p className="text-sm font-medium text-slate-900">权限级别</p>
              <p className="text-sm text-slate-500">
                {permission === 3
                  ? "管理员"
                  : permission === 2
                    ? "读写权限"
                    : permission === 1
                      ? "只读权限"
                      : "无权限"}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {permission >= 3 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Shield className="h-5 w-5" />
              管理功能
            </CardTitle>
            <CardDescription>平台级管理功能入口</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            <Button
              variant="outline"
              className="w-full justify-start"
              onClick={() => router.push("/users")}
            >
              <Shield className="h-4 w-4" />
              用户管理
            </Button>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
