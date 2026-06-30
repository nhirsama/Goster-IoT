"use client";

import Link from "next/link";
import { useState, useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";
import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { Turnstile } from "@marsidev/react-turnstile";

import { AuthAlert, AuthField, AuthShell } from "@/components/auth-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api, ApiError } from "@/lib/api-client";
import { Check, Lock, Mail, User, UserPlus } from "lucide-react";
import { components } from "@/lib/api-types";
import { queryKeys } from "@/lib/query-keys";

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
  const [captchaConfigError, setCaptchaConfigError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors },
  } = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
  });

  useEffect(() => {
    let alive = true;

    api
      .get<CaptchaConfig>("/api/v1/auth/captcha/config")
      .then((res) => {
        if (!alive) return;
        setCaptchaConfig(res);
        setCaptchaConfigError(null);
      })
      .catch(() => {
        if (!alive) return;
        setCaptchaConfigError("验证码配置加载失败，提交时将由服务端进行最终校验。");
      });

    return () => {
      alive = false;
    };
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
      queryClient.invalidateQueries({ queryKey: queryKeys.authMe });
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
    <AuthShell
      icon={UserPlus}
      title="创建账户"
      description="申请接入 Goster IoT 设备网络"
      footer={
        <>
          已有系统账户？{" "}
          <Link href="/login" className="font-semibold text-primary transition hover:text-primary/80">
            直接登录
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
            placeholder="请输入您的用户名"
            {...register("username")}
          />
        </AuthField>

        <AuthField label="电子邮箱（可选）" icon={Mail} error={errors.email?.message}>
          <Input
            className={`h-12 bg-white pl-10 text-slate-900 placeholder:text-slate-400 ${
              errors.email ? "border-rose-300 focus-visible:ring-rose-100" : ""
            }`}
            type="email"
            placeholder="john@example.com"
            {...register("email")}
          />
        </AuthField>

        <div className="grid gap-4 sm:grid-cols-2">
          <AuthField label="密码" icon={Lock} error={errors.password?.message}>
            <Input
              type="password"
              className={`h-12 bg-white pl-10 text-slate-900 placeholder:text-slate-400 ${
                errors.password ? "border-rose-300 focus-visible:ring-rose-100" : ""
              }`}
              placeholder="至少 8 位密码"
              {...register("password")}
            />
          </AuthField>

          <AuthField label="确认密码" icon={Check} error={errors.confirm_password?.message}>
            <Input
              type="password"
              className={`h-12 bg-white pl-10 text-slate-900 placeholder:text-slate-400 ${
                errors.confirm_password ? "border-rose-300 focus-visible:ring-rose-100" : ""
              }`}
              placeholder="再次输入"
              {...register("confirm_password")}
            />
          </AuthField>
        </div>

        {captchaConfig?.enabled && captchaConfig.provider === "turnstile" && captchaConfig.site_key ? (
          <div className="flex justify-center rounded-xl border border-slate-200 bg-slate-50/70 p-3">
            <Turnstile
              siteKey={captchaConfig.site_key}
              onSuccess={(token) => {
                setCaptchaToken(token);
                if (globalError === "请先完成人机验证") setGlobalError(null);
              }}
              options={{ theme: "light" }}
            />
          </div>
        ) : null}

        {captchaConfigError ? <AuthAlert tone="warning">{captchaConfigError}</AuthAlert> : null}

        <Button className="h-12 w-full text-base" type="submit" disabled={loading}>
          {loading ? "正在创建账户..." : "立即注册"}
        </Button>
      </form>
    </AuthShell>
  );
}
