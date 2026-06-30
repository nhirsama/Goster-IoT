import type { ReactNode } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

type DashboardPanelProps = {
  title: string;
  description?: string;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
};

export function DashboardPanel({
  title,
  description,
  action,
  children,
  className,
  contentClassName = "p-0",
}: DashboardPanelProps) {
  return (
    <Card className={cn("py-0", className)}>
      <div className="flex flex-col gap-3 border-b border-slate-200/70 bg-white/55 px-5 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6">
        <div className="min-w-0">
          <h2 className="text-base font-semibold text-slate-900">{title}</h2>
          {description ? <p className="mt-1 text-sm text-slate-500">{description}</p> : null}
        </div>
        {action ? <div className="shrink-0">{action}</div> : null}
      </div>
      <CardContent className={contentClassName}>{children}</CardContent>
    </Card>
  );
}

export function DashboardToolbar({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("rounded-xl border border-slate-200/70 bg-slate-50/70 p-3", className)}>
      {children}
    </div>
  );
}
