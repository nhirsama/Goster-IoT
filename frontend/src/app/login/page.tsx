"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { api, ApiError } from "@/lib/api-client";
import { ShieldCheck, User, Lock, Github } from "lucide-react";

const loginSchema = z.object({
  username: z.string().min(1, "请输入用户名"),
  password: z.string().min(1, "请输入密码"),
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
  });

  const onSubmit = async (data: LoginFormValues) => {
    setLoading(true);
    setGlobalError(null);
    try {
      await api.post("/api/v1/auth/login", data);
      queryClient.invalidateQueries({ queryKey: ["auth-me"] });
      router.push("/");
    } catch (err: any) {
      if (err instanceof ApiError && err.errorDetail) {
        const { field, reason, type } = err.errorDetail;
        if (field && (field === "username" || field === "password")) {
          setError(field as any, { type: "server", message: reason || "校验失败" });
        } else {
          setGlobalError(reason || err.message || "用户名或密码错误");
        }
      } else {
        setGlobalError(err.message || "系统内部错误，请稍后再试");
      }
    } finally {
      setLoading(false);
    }
  };

  // 根据 .env 自动拼接 GitHub OAuth 登录地址
  const apiBaseUrl = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/api\/v1$/, "").replace(/\/$/, "");
  const githubAuthUrl = `${apiBaseUrl}/auth/oauth2/github`;

  return (
    <div className="flex min-h-screen items-center justify-center bg-[#f8fafc] p-4 font-sans">
      <div className="absolute inset-0 bg-[radial-gradient(#e2e8f0_1px,transparent_1px)] [background-size:16px_16px] [mask-image:radial-gradient(ellipse_50%_50%_at_50%_50%,#000_70%,transparent_100%)]"></div>
      
      <Card className="w-full max-w-md border-none shadow-2xl shadow-blue-100/50 relative bg-white/80 backdrop-blur-sm">
        <CardHeader className="space-y-2 pb-6">
          <div className="mx-auto bg-blue-600 p-3 rounded-2xl w-fit shadow-lg shadow-blue-200 mb-2">
            <ShieldCheck className="h-8 w-8 text-white" />
          </div>
          <CardTitle className="text-2xl font-bold text-center text-slate-900 tracking-tight">
            Goster IoT 管理系统
          </CardTitle>
          <CardDescription className="text-center text-slate-500">
            欢迎回来，请登录您的账户以管理设备
          </CardDescription>
        </CardHeader>
        <form onSubmit={handleSubmit(onSubmit)}>
          <CardContent className="space-y-5">
            {globalError && (
              <div className="bg-red-50 text-red-600 p-3 rounded-xl text-sm border border-red-100 flex items-center gap-2 animate-in fade-in slide-in-from-top-1">
                <span className="h-1.5 w-1.5 rounded-full bg-red-600"></span>
                {globalError}
              </div>
            )}
            <div className="space-y-2">
              <label className="text-sm font-semibold text-slate-700 ml-1">
                用户名
              </label>
              <div className="relative">
                <User className="absolute left-3 top-3 h-4 w-4 text-slate-400" />
                <Input
                  className={`pl-10 h-11 focus:ring-blue-500 rounded-xl transition-all ${errors.username ? 'border-red-300 bg-red-50/50' : 'border-slate-200'}`}
                  placeholder="请输入用户名"
                  {...register("username")}
                />
              </div>
              {errors.username && (
                <p className="text-xs text-red-500 mt-1 font-medium flex items-center gap-1 animate-in fade-in">
                  <span className="h-1 w-1 rounded-full bg-red-500 inline-block"></span>
                  {errors.username.message}
                </p>
              )}
            </div>
            <div className="space-y-2">
              <label className="text-sm font-semibold text-slate-700 ml-1">
                密码
              </label>
              <div className="relative">
                <Lock className="absolute left-3 top-3 h-4 w-4 text-slate-400" />
                <Input
                  className={`pl-10 h-11 focus:ring-blue-500 rounded-xl transition-all ${errors.password ? 'border-red-300 bg-red-50/50' : 'border-slate-200'}`}
                  type="password"
                  placeholder="请输入密码"
                  {...register("password")}
                />
              </div>
              {errors.password && (
                <p className="text-xs text-red-500 mt-1 font-medium flex items-center gap-1 animate-in fade-in">
                  <span className="h-1 w-1 rounded-full bg-red-500 inline-block"></span>
                  {errors.password.message}
                </p>
              )}
            </div>
          </CardContent>
          <CardFooter className="flex flex-col gap-4 pt-4">
            <Button className="w-full h-11 rounded-xl bg-blue-600 hover:bg-blue-700 text-white font-semibold transition-all shadow-lg shadow-blue-100" type="submit" disabled={loading}>
              {loading ? "正在登录..." : "立即登录"}
            </Button>
            
            <div className="relative w-full my-2">
              <div className="absolute inset-0 flex items-center">
                <span className="w-full border-t border-slate-200" />
              </div>
              <div className="relative flex justify-center text-xs uppercase">
                <span className="bg-white/80 px-2 text-slate-500 font-medium">或者使用以下方式</span>
              </div>
            </div>

            {/* 1:1 复刻：GitHub OAuth 登录 */}
            <a href={githubAuthUrl} className="w-full">
              <Button type="button" variant="outline" className="w-full h-11 rounded-xl border-slate-200 text-slate-700 hover:bg-slate-50 transition-all font-semibold">
                <Github className="h-5 w-5 mr-2" />
                使用 GitHub 账号登录
              </Button>
            </a>

            <p className="text-sm text-center text-slate-500 mt-2">
              还没有账户？{" "}
              <a href="/register" className="text-blue-600 font-semibold hover:text-blue-700 hover:underline underline-offset-4">
                立即注册
              </a>
            </p>
          </CardFooter>
        </form>
      </Card>
    </div>
  );
}
