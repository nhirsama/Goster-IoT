"use client";

import { useState, useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { Turnstile } from "@marsidev/react-turnstile";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api, ApiError } from "@/lib/api-client";
import { UserPlus, User, Mail, Lock, Check } from "lucide-react";
import { components } from "@/lib/api-types";

const registerSchema = z
  .object({
    username: z.string().min(3, "用户名至少需要 3 个字符"),
    password: z.string().min(8, "密码至少需要 8 个字符"),
    confirm_password: z.string().min(1, "请再次输入密码"),
    email: z.string().email("请输入有效的电子邮箱地址").optional().or(z.literal("")),
  })
  .refine((data) => data.password === data.confirm_password, {
    message: "两次输入的密码不一致",
    path: ["confirm_password"],
  });

type RegisterFormValues = z.infer<typeof registerSchema>;
type CaptchaConfig = components["schemas"]["CaptchaConfig"];

export default function RegisterPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [globalError, setGlobalError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [captchaConfig, setCaptchaConfig] = useState<CaptchaConfig | null>(null);
  const [captchaToken, setCaptchaToken] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors },
  } = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
  });

  useEffect(() => {
    api.get<CaptchaConfig>("/api/v1/auth/captcha/config").then((res) => {
      setCaptchaConfig(res);
    }).catch(console.error);
  }, []);

  const onSubmit = async (data: RegisterFormValues) => {
    if (captchaConfig?.enabled && !captchaToken) {
      setGlobalError("请先完成人机验证");
      return;
    }

    setLoading(true);
    setGlobalError(null);
    try {
      const payload = {
        username: data.username,
        password: data.password,
        email: data.email,
      };
      await api.post("/api/v1/auth/register", {
        ...payload,
        captcha_token: captchaToken || undefined,
      });
      queryClient.invalidateQueries({ queryKey: ["auth-me"] });
      router.push("/");
    } catch (err: unknown) {
      if (err instanceof ApiError && err.errorDetail) {
        const { field, reason } = err.errorDetail;
        if (field && (field === "username" || field === "password" || field === "email")) {
          setError(field, { type: "server", message: reason || "校验失败" });
        } else {
          setGlobalError(reason || err.message || "注册失败，请检查输入");
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

  return (
    <div className="flex min-h-screen bg-[#0f172a] font-sans">
      <div className="flex-1 flex items-center justify-center p-6 relative">
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[500px] h-[500px] bg-blue-600/20 blur-[120px] rounded-full pointer-events-none"></div>

        <div className="w-full max-w-md bg-[#1e293b]/80 backdrop-blur-xl border border-slate-700/50 p-8 rounded-3xl shadow-2xl relative z-10">
          <div className="mb-8 text-center">
            <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-blue-600 shadow-lg shadow-blue-900/50 mb-4">
              <UserPlus className="h-8 w-8 text-white" />
            </div>
            <h1 className="text-3xl font-black text-white tracking-tight">创建账户</h1>
            <p className="text-slate-400 mt-2 font-medium">接入 Goster IoT 设备网络</p>
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
                  placeholder="请输入您的用户名"
                  {...register("username")}
                />
              </div>
              {errors.username && <p className="text-xs text-rose-400 mt-1 ml-1">{errors.username.message}</p>}
            </div>

            <div className="space-y-2">
              <label className="text-xs font-bold text-slate-400 uppercase tracking-widest ml-1">电子邮箱 (可选)</label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-slate-500" />
                <Input
                  className={`pl-10 h-12 bg-[#0f172a] border-slate-700 text-white placeholder:text-slate-600 rounded-xl focus-visible:ring-1 focus-visible:ring-blue-500 focus-visible:border-blue-500 ${errors.email ? 'border-rose-500/50 focus-visible:ring-rose-500' : ''}`}
                  type="email"
                  placeholder="john@example.com"
                  {...register("email")}
                />
              </div>
              {errors.email && <p className="text-xs text-rose-400 mt-1 ml-1">{errors.email.message}</p>}
            </div>

            <div className="space-y-2">
              <label className="text-xs font-bold text-slate-400 uppercase tracking-widest ml-1">密码</label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-slate-500" />
                <Input
                  type="password"
                  className={`pl-10 h-12 bg-[#0f172a] border-slate-700 text-white placeholder:text-slate-600 rounded-xl focus-visible:ring-1 focus-visible:ring-blue-500 focus-visible:border-blue-500 ${errors.password ? 'border-rose-500/50 focus-visible:ring-rose-500' : ''}`}
                  placeholder="至少 8 位密码"
                  {...register("password")}
                />
              </div>
              {errors.password && <p className="text-xs text-rose-400 mt-1 ml-1">{errors.password.message}</p>}
            </div>

            <div className="space-y-2">
              <label className="text-xs font-bold text-slate-400 uppercase tracking-widest ml-1">确认密码</label>
              <div className="relative">
                <Check className="absolute left-3 top-1/2 -translate-y-1/2 h-5 w-5 text-slate-500" />
                <Input
                  type="password"
                  className={`pl-10 h-12 bg-[#0f172a] border-slate-700 text-white placeholder:text-slate-600 rounded-xl focus-visible:ring-1 focus-visible:ring-blue-500 focus-visible:border-blue-500 ${errors.confirm_password ? 'border-rose-500/50 focus-visible:ring-rose-500' : ''}`}
                  placeholder="请再次输入密码"
                  {...register("confirm_password")}
                />
              </div>
              {errors.confirm_password && <p className="text-xs text-rose-400 mt-1 ml-1">{errors.confirm_password.message}</p>}
            </div>
            
            {captchaConfig?.enabled && captchaConfig.provider === "turnstile" && captchaConfig.site_key && (
              <div className="flex justify-center mt-4">
                <Turnstile 
                  siteKey={captchaConfig.site_key} 
                  onSuccess={(token) => {
                    setCaptchaToken(token);
                    if (globalError === "请先完成人机验证") setGlobalError(null);
                  }}
                  options={{ theme: "dark" }}
                />
              </div>
            )}

            <Button className="w-full h-12 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-bold text-base shadow-lg shadow-blue-900/20 transition-all mt-6" type="submit" disabled={loading}>
              {loading ? "正在创建账户..." : "立即注册"}
            </Button>

            <p className="text-sm text-center text-slate-400 pt-4">
              已有系统账户？ <a href="/login" className="text-blue-400 font-bold hover:text-blue-300 transition-colors">直接登录</a>
            </p>
          </form>
        </div>
      </div>
    </div>
  );
}
