"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api, ApiError } from "@/lib/api-client";
import { Network, User, Lock, Github } from "lucide-react";

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
      queryClient.invalidateQueries({ queryKey: ["auth-me"] });
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

  const apiBaseUrl = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/api\/v1$/, "").replace(/\/$/, "");
  const githubAuthUrl = `${apiBaseUrl}/auth/oauth2/github`;

  return (
    <div className="flex min-h-screen bg-[#0f172a] font-sans">
      <div className="flex-1 flex items-center justify-center p-6 relative">
        {/* 背景光晕装饰 */}
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[500px] h-[500px] bg-blue-600/20 blur-[120px] rounded-full pointer-events-none"></div>

        <div className="w-full max-w-md bg-[#1e293b]/80 backdrop-blur-xl border border-slate-700/50 p-8 rounded-3xl shadow-2xl relative z-10">
          <div className="mb-8 text-center">
            <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-blue-600 shadow-lg shadow-blue-900/50 mb-4">
              <Network className="h-8 w-8 text-white" />
            </div>
            <h1 className="text-3xl font-black text-white tracking-tight">Goster IoT</h1>
            <p className="text-slate-400 mt-2 font-medium">设备管理控制台</p>
          </div>

          <form onSubmit={handleSubmit(onSubmit)} className="space-y-5">
            {globalError && (
              <div className="bg-rose-500/10 border border-rose-500/20 text-rose-400 p-3 rounded-xl text-sm flex items-center gap-2">
                <span className="h-1.5 w-1.5 rounded-full bg-rose-500 shrink-0"></span>
                {globalError}
              </div>
            )}
            
            <div className="space-y-2">
              <label className="text-xs font-bold text-slate-400 uppercase tracking-widest ml-1">用户名</label>
              <div className="relative">
                <User className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-slate-500" />
                <Input
                  className={`pl-10 h-12 bg-[#0f172a] border-slate-700 text-white placeholder:text-slate-600 rounded-xl focus-visible:ring-1 focus-visible:ring-blue-500 focus-visible:border-blue-500 ${errors.username ? 'border-rose-500/50 focus-visible:ring-rose-500' : ''}`}
                  placeholder="admin"
                  {...register("username")}
                />
              </div>
              {errors.username && <p className="text-xs text-rose-400 mt-1 ml-1">{errors.username.message}</p>}
            </div>

            <div className="space-y-2">
              <label className="text-xs font-bold text-slate-400 uppercase tracking-widest ml-1">密码</label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-slate-500" />
                <Input
                  type="password"
                  className={`pl-10 h-12 bg-[#0f172a] border-slate-700 text-white placeholder:text-slate-600 rounded-xl focus-visible:ring-1 focus-visible:ring-blue-500 focus-visible:border-blue-500 ${errors.password ? 'border-rose-500/50 focus-visible:ring-rose-500' : ''}`}
                  placeholder="••••••••"
                  {...register("password")}
                />
              </div>
              {errors.password && <p className="text-xs text-rose-400 mt-1 ml-1">{errors.password.message}</p>}
            </div>

            <label className="flex items-center gap-2 text-sm text-slate-400 cursor-pointer">
              <input
                type="checkbox"
                className="h-4 w-4 rounded border-slate-600 bg-[#0f172a] accent-blue-600"
                {...register("remember_me")}
              />
              记住我
            </label>

            <Button className="w-full h-12 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-bold text-base shadow-lg shadow-blue-900/20 transition-all mt-4" type="submit" disabled={loading}>
              {loading ? "正在验证..." : "登录系统"}
            </Button>

            <div className="relative flex items-center py-2">
              <div className="flex-grow border-t border-slate-700"></div>
              <span className="flex-shrink-0 mx-4 text-slate-500 text-xs font-medium uppercase tracking-widest">或者使用</span>
              <div className="flex-grow border-t border-slate-700"></div>
            </div>

            <a href={githubAuthUrl} className="block w-full">
              <Button type="button" variant="outline" className="w-full h-12 rounded-xl bg-[#0f172a] border-slate-700 text-slate-300 hover:bg-slate-800 hover:text-white transition-all font-bold">
                <Github className="h-5 w-5 mr-2" />
                GitHub 账号登录
              </Button>
            </a>

            <p className="text-sm text-center text-slate-400 pt-2">
              需要新账户？ <a href="/register" className="text-blue-400 font-bold hover:text-blue-300 transition-colors">申请接入</a>
            </p>
          </form>
        </div>
      </div>
    </div>
  );
}
