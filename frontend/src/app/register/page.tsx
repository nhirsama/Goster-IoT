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
import { api } from "@/lib/api-client";
import { UserPlus, User, Mail, Lock } from "lucide-react";

const registerSchema = z.object({
  username: z.string().min(3, "用户名至少需要 3 个字符"),
  password: z.string().min(8, "密码至少需要 8 个字符"),
  email: z.string().email("请输入有效的电子邮箱地址").optional().or(z.literal("")),
});

type RegisterFormValues = z.infer<typeof registerSchema>;

export default function RegisterPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
  });

  const onSubmit = async (data: RegisterFormValues) => {
    setLoading(true);
    setError(null);
    try {
      await api.post("/api/v1/auth/register", data);
      queryClient.invalidateQueries({ queryKey: ["auth-me"] });
      router.push("/");
    } catch (err: any) {
      setError(err.message || "注册失败，用户名可能已存在");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-[#f8fafc] p-4 font-sans">
      <div className="absolute inset-0 bg-[radial-gradient(#e2e8f0_1px,transparent_1px)] [background-size:16px_16px] [mask-image:radial-gradient(ellipse_50%_50%_at_50%_50%,#000_70%,transparent_100%)]"></div>
      
      <Card className="w-full max-w-md border-none shadow-2xl shadow-blue-100/50 relative bg-white/80 backdrop-blur-sm">
        <CardHeader className="space-y-2 pb-8">
          <div className="mx-auto bg-blue-600 p-3 rounded-2xl w-fit shadow-lg shadow-blue-200 mb-2">
            <UserPlus className="h-8 w-8 text-white" />
          </div>
          <CardTitle className="text-2xl font-bold text-center text-slate-900 tracking-tight">
            创建新账户
          </CardTitle>
          <CardDescription className="text-center text-slate-500">
            注册以开始管理您的 Goster IoT 设备
          </CardDescription>
        </CardHeader>
        <form onSubmit={handleSubmit(onSubmit)}>
          <CardContent className="space-y-5">
            {error && (
              <div className="bg-red-50 text-red-600 p-3 rounded-xl text-sm border border-red-100 flex items-center gap-2 animate-in fade-in slide-in-from-top-1">
                <span className="h-1.5 w-1.5 rounded-full bg-red-600"></span>
                {error}
              </div>
            )}
            <div className="space-y-2">
              <label className="text-sm font-semibold text-slate-700 ml-1">
                用户名
              </label>
              <div className="relative">
                <User className="absolute left-3 top-3 h-4 w-4 text-slate-400" />
                <Input
                  className="pl-10 h-11 border-slate-200 focus:ring-blue-500 rounded-xl transition-all"
                  placeholder="请输入您的用户名"
                  {...register("username")}
                />
              </div>
              {errors.username && (
                <p className="text-xs text-red-500 mt-1">{errors.username.message}</p>
              )}
            </div>
            <div className="space-y-2">
              <label className="text-sm font-semibold text-slate-700 ml-1">
                电子邮箱 (可选)
              </label>
              <div className="relative">
                <Mail className="absolute left-3 top-3 h-4 w-4 text-slate-400" />
                <Input
                  className="pl-10 h-11 border-slate-200 focus:ring-blue-500 rounded-xl transition-all"
                  type="email"
                  placeholder="john@example.com"
                  {...register("email")}
                />
              </div>
              {errors.email && (
                <p className="text-xs text-red-500 mt-1">{errors.email.message}</p>
              )}
            </div>
            <div className="space-y-2">
              <label className="text-sm font-semibold text-slate-700 ml-1">
                密码
              </label>
              <div className="relative">
                <Lock className="absolute left-3 top-3 h-4 w-4 text-slate-400" />
                <Input
                  className="pl-10 h-11 border-slate-200 focus:ring-blue-500 rounded-xl transition-all"
                  type="password"
                  placeholder="请输入至少 8 位密码"
                  {...register("password")}
                />
              </div>
              {errors.password && (
                <p className="text-xs text-red-500 mt-1">{errors.password.message}</p>
              )}
            </div>
          </CardContent>
          <CardFooter className="flex flex-col gap-4 pt-4">
            <Button className="w-full h-11 rounded-xl bg-blue-600 hover:bg-blue-700 text-white font-semibold transition-all shadow-lg shadow-blue-100" type="submit" disabled={loading}>
              {loading ? "正在创建账户..." : "立即注册"}
            </Button>
            <p className="text-sm text-center text-slate-500">
              已经有账户了？{" "}
              <a href="/login" className="text-blue-600 font-semibold hover:text-blue-700 hover:underline underline-offset-4">
                立即登录
              </a>
            </p>
          </CardFooter>
        </form>
      </Card>
    </div>
  );
}
