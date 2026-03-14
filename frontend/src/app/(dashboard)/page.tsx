"use client";

import { useAuth } from "@/hooks/use-auth";
import { 
  LineChart, 
  Wifi, 
  ShieldAlert, 
  Database 
} from "lucide-react";

export default function DashboardHome() {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading || !isAuthenticated) return null;

  return (
    <div className="container-fluid h-full flex flex-col items-center justify-center min-h-[80vh]">
      <div className="text-center text-slate-500 max-w-2xl mx-auto">
        <div className="mb-6 flex justify-center">
          <div className="bg-white p-6 rounded-3xl shadow-sm border border-slate-100 d-inline-block">
            <LineChart className="h-20 w-20 text-blue-600" />
          </div>
        </div>
        <h2 className="text-3xl font-light mb-4 text-slate-900">欢迎使用 Goster-IoT</h2>
        <p className="text-lg text-slate-500 mb-12">请从左侧列表选择一个设备以查看实时监控数据、日志及配置信息。</p>
        
        {/* Desktop Cards */}
        <div className="hidden lg:flex gap-6 justify-center">
          <div className="bg-white p-6 rounded-2xl shadow-sm border border-slate-100 w-40 flex flex-col items-center hover:-translate-y-1 transition-transform cursor-default">
            <Wifi className="h-10 w-10 text-emerald-500 mb-4" />
            <div className="text-sm font-bold text-slate-700">实时监控</div>
          </div>
          <div className="bg-white p-6 rounded-2xl shadow-sm border border-slate-100 w-40 flex flex-col items-center hover:-translate-y-1 transition-transform cursor-default">
            <ShieldAlert className="h-10 w-10 text-blue-500 mb-4" />
            <div className="text-sm font-bold text-slate-700">安全认证</div>
          </div>
          <div className="bg-white p-6 rounded-2xl shadow-sm border border-slate-100 w-40 flex flex-col items-center hover:-translate-y-1 transition-transform cursor-default">
            <Database className="h-10 w-10 text-cyan-500 mb-4" />
            <div className="text-sm font-bold text-slate-700">数据存储</div>
          </div>
        </div>

        {/* Mobile Hint */}
        <div className="lg:hidden mt-8">
          <p className="text-slate-400 text-sm">请使用底部导航切换首页、设备和管理</p>
        </div>
      </div>
    </div>
  );
}
