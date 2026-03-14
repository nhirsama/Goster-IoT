import type { LucideIcon } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

type StatCardTone = "primary" | "success" | "warning" | "neutral";

type StatCardProps = {
  title: string;
  value: string | number;
  hint?: string;
  icon: LucideIcon;
  tone?: StatCardTone;
  className?: string;
};

const toneClassMap: Record<StatCardTone, string> = {
  primary: "bg-primary/10 text-primary border-primary/20",
  success: "bg-emerald-500/10 text-emerald-600 border-emerald-500/20",
  warning: "bg-amber-500/10 text-amber-600 border-amber-500/20",
  neutral: "bg-slate-500/10 text-slate-600 border-slate-500/20",
};

export function StatCard({ title, value, hint, icon: Icon, tone = "neutral", className }: StatCardProps) {
  return (
    <Card className={cn("elevate-hover", className)}>
      <CardHeader className="border-b border-slate-200/70 pb-3">
        <CardTitle className="text-sm font-medium text-slate-500">{title}</CardTitle>
      </CardHeader>
      <CardContent className="flex items-center justify-between gap-3 pt-4">
        <div>
          <div className="text-2xl font-semibold text-slate-900">{value}</div>
          {hint ? <p className="mt-1 text-xs text-slate-500">{hint}</p> : null}
        </div>
        <div className={cn("rounded-xl border p-2.5", toneClassMap[tone])}>
          <Icon className="h-4 w-4" />
        </div>
      </CardContent>
    </Card>
  );
}
