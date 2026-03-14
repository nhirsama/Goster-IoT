import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

type PageHeaderProps = {
  icon: LucideIcon;
  title: string;
  description: string;
  action?: ReactNode;
  className?: string;
};

export function PageHeader({ icon: Icon, title, description, action, className }: PageHeaderProps) {
  return (
    <div className={cn("flex flex-col gap-4 md:flex-row md:items-end md:justify-between", className)}>
      <div className="flex items-start gap-3">
        <div className="rounded-2xl border border-primary/15 bg-primary/10 p-3 text-primary shadow-sm">
          <Icon className="h-5 w-5" />
        </div>
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight text-slate-900 sm:text-3xl">{title}</h1>
          <p className="text-sm text-slate-500 sm:text-base">{description}</p>
        </div>
      </div>
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}
