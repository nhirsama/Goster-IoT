"use client";

import Link from "next/link";
import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";

import { AuthAlert, AuthField, AuthShell } from "@/components/auth-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api, ApiError } from "@/lib/api-client";
import { Github, Lock, Network, User } from "lucide-react";
import { queryKeys } from "@/lib/query-keys";

const loginSchema = z.object({
  username: z.string().min(1, "请输入用户名"),
  password: z.string().min(1, "请输入密码"),
  remember_me: z.boolean().optional(),
});

type LoginFormValues = z.infer<typeof loginSchema>;

export default function LoginPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [globalError, setGlobalError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors },
  } = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { remember_me: false },
  });

  const onSubmit = async (data: LoginFormValues) => {
    setLoading(true);
    setGlobalError(null);
    try {
      await api.post("/api/v1/auth/login", {
        ...data,
        remember_me: !!data.remember_me,
      });
      queryClient.invalidateQueries({ queryKey: queryKeys.authMe });
      router.push("/");
    } catch (err: unknown) {
      if (err instanceof ApiError && err.errorDetail) {
        const { field, reason } = err.errorDetail;
        if (field && (field === "username" || field === "password")) {
          setError(field, { type: "server", message: reason || "校验失败" });
        } else {
          setGlobalError(reason || err.message || "用户名或密码错误");
        }
      } else if (err instanceof Error) {
        setGlobalError(err.message || "系统内部错误，请稍后再试");
      } else {
        setGlobalError("系统内部错误，请稍后再试");
      }
    } finally {
      setLoading(false);
    }
  };

  const apiBaseUrl = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080")
    .replace(/\/api\/v1$/, "")
    .replace(/\/$/, "");
  const githubAuthUrl = `${apiBaseUrl}/auth/oauth2/github`;

  return (
    <AuthShell
      icon={Network}
      title="登录控制台"
      description="继续管理设备、认证队列和租户资源"
      footer={
        <>
          需要新账户？{" "}
          <Link href="/register" className="font-semibold text-primary transition hover:text-primary/80">
            申请接入
          </Link>
        </>
      }
    >
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
        {globalError ? <AuthAlert>{globalError}</AuthAlert> : null}

        <AuthField label="用户名" icon={User} error={errors.username?.message}>
          <Input
            className={`h-12 bg-white pl-10 text-slate-900 placeholder:text-slate-400 ${
              errors.username ? "border-rose-300 focus-visible:ring-rose-100" : ""
            }`}
            placeholder="admin"
            {...register("username")}
          />
        </AuthField>

        <AuthField label="密码" icon={Lock} error={errors.password?.message}>
          <Input
            type="password"
            className={`h-12 bg-white pl-10 text-slate-900 placeholder:text-slate-400 ${
              errors.password ? "border-rose-300 focus-visible:ring-rose-100" : ""
            }`}
            placeholder="••••••••"
            {...register("password")}
          />
        </AuthField>

        <label className="flex cursor-pointer items-center justify-between gap-3 rounded-xl border border-slate-200 bg-slate-50/70 px-3 py-2.5 text-sm text-slate-600">
          <span>记住这次登录</span>
          <input type="checkbox" className="h-4 w-4 rounded border-slate-300 accent-primary" {...register("remember_me")} />
        </label>

        <Button className="h-12 w-full text-base" type="submit" disabled={loading}>
          {loading ? "正在验证..." : "登录系统"}
        </Button>

        <div className="relative flex items-center py-1">
          <div className="flex-grow border-t border-slate-200" />
          <span className="mx-4 flex-shrink-0 text-xs font-semibold uppercase tracking-widest text-slate-400">
            或者使用
          </span>
          <div className="flex-grow border-t border-slate-200" />
        </div>

        <Button asChild type="button" variant="outline" className="h-12 w-full bg-white font-semibold">
          <a href={githubAuthUrl}>
            <Github className="h-5 w-5" />
            GitHub 账号登录
          </a>
        </Button>
      </form>
    </AuthShell>
  );
}
