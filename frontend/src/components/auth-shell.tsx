import Link from "next/link";
import type { LucideIcon } from "lucide-react";
import { Activity, BellRing, Network, ShieldCheck, Wifi } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export function AuthShell({
  icon: Icon,
  title,
  description,
  children,
  footer,
  className,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
  children: React.ReactNode;
  footer: React.ReactNode;
  className?: string;
}) {
  return (
    <main className="relative flex min-h-screen items-center justify-center overflow-hidden px-4 py-10 text-slate-900 sm:px-6 lg:px-8">
      <div className="pointer-events-none absolute inset-0 -z-10">
        <div className="absolute left-[8%] top-[12%] h-72 w-72 rounded-full bg-blue-500/10 blur-3xl" />
        <div className="absolute bottom-[8%] right-[8%] h-80 w-80 rounded-full bg-teal-400/10 blur-3xl" />
      </div>

      <div className="grid w-full max-w-6xl gap-8 lg:grid-cols-[minmax(0,1fr)_minmax(420px,0.82fr)] lg:items-center">
        <section className="hidden space-y-6 lg:block">
          <Link href="/" className="inline-flex items-center gap-3">
            <div className="rounded-2xl bg-primary p-3 text-primary-foreground shadow-sm">
              <Network className="h-6 w-6" />
            </div>
            <div>
              <p className="text-xl font-semibold tracking-tight text-slate-950">Goster IoT</p>
              <p className="text-sm text-slate-500">Management Dashboard</p>
            </div>
          </Link>

          <div className="max-w-xl space-y-4">
            <Badge className="rounded-full bg-white/80 px-3 py-1 text-xs text-slate-600 ring-1 ring-slate-200">
              <Activity className="mr-1 h-3.5 w-3.5 text-emerald-500" />
              设备与租户统一管理
            </Badge>
            <h1 className="text-4xl font-semibold tracking-tight text-slate-950 xl:text-5xl">
              用同一套控制台管理设备、认证和租户边界
            </h1>
            <p className="text-base leading-7 text-slate-600">
              登录后可查看实时设备状态、处理接入审批、维护黑名单，并在多租户上下文中管理成员和权限。
            </p>
          </div>

          <div className="grid max-w-2xl gap-3 sm:grid-cols-3">
            {[
              { label: "设备状态", hint: "自动同步", icon: Wifi },
              { label: "安全审批", hint: "权限隔离", icon: ShieldCheck },
              { label: "租户邀请", hint: "成员协作", icon: BellRing },
            ].map((item) => (
              <div key={item.label} className="glass-card rounded-2xl p-4">
                <div className="mb-3 w-fit rounded-xl bg-primary/10 p-2 text-primary">
                  <item.icon className="h-4 w-4" />
                </div>
                <p className="text-sm font-semibold text-slate-900">{item.label}</p>
                <p className="mt-1 text-xs text-slate-500">{item.hint}</p>
              </div>
            ))}
          </div>
        </section>

        <section className={cn("glass-card relative w-full rounded-3xl p-6 sm:p-8", className)}>
          <div className="mb-8 text-center">
            <div className="mx-auto mb-4 inline-flex h-16 w-16 items-center justify-center rounded-2xl border border-primary/15 bg-primary/10 text-primary shadow-sm">
              <Icon className="h-8 w-8" />
            </div>
            <h2 className="text-3xl font-semibold tracking-tight text-slate-950">{title}</h2>
            <p className="mt-2 text-sm font-medium text-slate-500">{description}</p>
          </div>

          {children}

          <div className="mt-6 text-center text-sm text-slate-500">{footer}</div>
        </section>
      </div>
    </main>
  );
}

export function AuthField({
  label,
  icon: Icon,
  error,
  children,
}: {
  label: string;
  icon: LucideIcon;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-2">
      <label className="ml-1 text-xs font-semibold uppercase tracking-widest text-slate-500">{label}</label>
      <div className="relative">
        <Icon className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        {children}
      </div>
      {error ? <p className="ml-1 mt-1 text-xs text-rose-600">{error}</p> : null}
    </div>
  );
}

export function AuthAlert({ children, tone = "danger" }: { children: React.ReactNode; tone?: "danger" | "warning" }) {
  const danger = tone === "danger";
  return (
    <div
      className={cn(
        "flex items-center gap-2 rounded-xl border p-3 text-sm",
        danger ? "border-rose-200 bg-rose-50 text-rose-700" : "border-amber-200 bg-amber-50 text-amber-700"
      )}
    >
      <span className={cn("h-1.5 w-1.5 shrink-0 rounded-full", danger ? "bg-rose-500" : "bg-amber-500")} />
      {children}
    </div>
  );
}
