"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api, getApiErrorMessage } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";
import { useAuth } from "@/hooks/use-auth";
import { useUx } from "@/components/providers/ux-provider";
import { PageHeader } from "@/components/dashboard/page-header";
import { EmptyState } from "@/components/dashboard/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Bell, Building2, Check, Clock, RefreshCw, Shield, X } from "lucide-react";

type TenantRole = "tenant_admin" | "tenant_rw" | "tenant_ro";
type TenantInvitation = {
  id: string;
  tenant_id: string;
  username: string;
  role: TenantRole;
  invited_by: string;
  status: string;
  expires_at: string;
  created_at: string;
  updated_at: string;
};
type InvitationListData = {
  items: TenantInvitation[];
  total: number;
};

const roleLabels: Record<TenantRole, string> = {
  tenant_admin: "租户管理员",
  tenant_rw: "读写权限",
  tenant_ro: "只读权限",
};

function formatDate(value?: string) {
  if (!value) return "-";
  return new Date(value).toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function formatRelativeTime(value: string) {
  const now = new Date();
  const target = new Date(value);
  const diffMs = target.getTime() - now.getTime();
  const diffDays = Math.ceil(diffMs / (1000 * 60 * 60 * 24));

  if (diffDays < 0) return "已过期";
  if (diffDays === 0) return "今天到期";
  if (diffDays === 1) return "明天到期";
  return `${diffDays} 天后到期`;
}

export default function InvitationsPage() {
  const queryClient = useQueryClient();
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const { toast, confirm } = useUx();

  const { data: invitationData, isLoading } = useQuery({
    queryKey: queryKeys.invitations,
    queryFn: () => api.get<InvitationListData>("/api/v1/invitations"),
    enabled: isAuthenticated,
  });

  const acceptMutation = useMutation({
    mutationFn: (invitationId: string) =>
      api.post(`/api/v1/invitations/${encodeURIComponent(invitationId)}/accept`),
    onSuccess: () => {
      toast.success("已接受邀请");
      queryClient.invalidateQueries({ queryKey: queryKeys.invitations });
      queryClient.invalidateQueries({ queryKey: queryKeys.tenants });
      queryClient.invalidateQueries({ queryKey: queryKeys.authMe });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "接受邀请失败"));
    },
  });

  const rejectMutation = useMutation({
    mutationFn: (invitationId: string) =>
      api.post(`/api/v1/invitations/${encodeURIComponent(invitationId)}/reject`),
    onSuccess: () => {
      toast.success("已拒绝邀请");
      queryClient.invalidateQueries({ queryKey: queryKeys.invitations });
    },
    onError: (error: unknown) => {
      toast.error(getApiErrorMessage(error, "拒绝邀请失败"));
    },
  });

  if (authLoading) {
    return <EmptyState icon={RefreshCw} title="加载中" description="请稍候..." className="py-24" />;
  }

  if (!isAuthenticated) {
    return <EmptyState icon={Shield} title="需要登录" description="请先登录后再访问" className="py-24" />;
  }

  const invitations = invitationData?.items || [];

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Bell}
        title="租户邀请"
        description="查看并处理您收到的租户邀请"
      />

      {isLoading ? (
        <EmptyState icon={RefreshCw} title="加载中" description="请稍候..." className="py-16" />
      ) : invitations.length === 0 ? (
        <EmptyState
          icon={Bell}
          title="暂无邀请"
          description="您目前没有待处理的租户邀请"
          className="py-16"
        />
      ) : (
        <div className="grid gap-4">
          {invitations.map((invitation) => {
            const isExpired = new Date(invitation.expires_at) < new Date();

            return (
              <Card key={invitation.id} className={isExpired ? "opacity-60" : ""}>
                <CardContent className="p-6">
                  <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
                    <div className="flex items-start gap-4">
                      <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                        <Building2 className="h-6 w-6" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <h3 className="text-lg font-semibold text-slate-900">
                            {invitation.tenant_id}
                          </h3>
                          <Badge variant="outline" className="border-cyan-200 bg-cyan-50 text-cyan-700">
                            {roleLabels[invitation.role]}
                          </Badge>
                        </div>
                        <div className="mt-2 space-y-1 text-sm text-slate-600">
                          <div className="flex items-center gap-2">
                            <Shield className="h-4 w-4 text-slate-400" />
                            <span>邀请人：{invitation.invited_by}</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <Clock className="h-4 w-4 text-slate-400" />
                            <span>{formatDate(invitation.created_at)}</span>
                            <span className="text-slate-400">·</span>
                            <span className={isExpired ? "text-rose-600" : "text-slate-600"}>
                              {formatRelativeTime(invitation.expires_at)}
                            </span>
                          </div>
                        </div>
                      </div>
                    </div>

                    {!isExpired && (
                      <div className="flex gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={async () => {
                            const ok = await confirm({
                              title: "拒绝邀请",
                              description: `确定要拒绝来自 ${invitation.tenant_id} 的邀请吗？`,
                              confirmText: "拒绝",
                              cancelText: "取消",
                              tone: "danger",
                            });
                            if (ok) {
                              rejectMutation.mutate(invitation.id);
                            }
                          }}
                          disabled={rejectMutation.isPending}
                        >
                          <X className="h-4 w-4" />
                          拒绝
                        </Button>
                        <Button
                          size="sm"
                          onClick={async () => {
                            const ok = await confirm({
                              title: "接受邀请",
                              description: `确定要加入 ${invitation.tenant_id} 吗？您将获得${roleLabels[invitation.role]}权限。`,
                              confirmText: "接受",
                              cancelText: "取消",
                            });
                            if (ok) {
                              acceptMutation.mutate(invitation.id);
                            }
                          }}
                          disabled={acceptMutation.isPending}
                        >
                          <Check className="h-4 w-4" />
                          接受邀请
                        </Button>
                      </div>
                    )}

                    {isExpired && (
                      <Badge variant="outline" className="border-slate-300 bg-slate-100 text-slate-600">
                        已过期
                      </Badge>
                    )}
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
