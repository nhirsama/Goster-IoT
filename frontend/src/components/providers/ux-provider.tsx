"use client";

import { createContext, useCallback, useContext, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

type ToastTone = "success" | "error" | "info";
type ConfirmTone = "default" | "danger";

type ToastItem = {
  id: number;
  message: string;
  tone: ToastTone;
};

type ConfirmOptions = {
  title: string;
  description?: string;
  confirmText?: string;
  cancelText?: string;
  tone?: ConfirmTone;
};

type ToastApi = {
  show: (message: string, tone?: ToastTone) => void;
  success: (message: string) => void;
  error: (message: string) => void;
  info: (message: string) => void;
};

type UxContextValue = {
  toast: ToastApi;
  confirm: (options: ConfirmOptions) => Promise<boolean>;
};

type ConfirmState = {
  options: ConfirmOptions;
  resolve: (result: boolean) => void;
};

const UxContext = createContext<UxContextValue | null>(null);

export function UxProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [confirmState, setConfirmState] = useState<ConfirmState | null>(null);
  const nextToastId = useRef(1);

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((item) => item.id !== id));
  }, []);

  const showToast = useCallback(
    (message: string, tone: ToastTone = "info") => {
      const id = nextToastId.current++;
      setToasts((prev) => [...prev, { id, message, tone }]);
      setTimeout(() => removeToast(id), 2600);
    },
    [removeToast]
  );

  const confirm = useCallback((options: ConfirmOptions) => {
    return new Promise<boolean>((resolve) => {
      setConfirmState({ options, resolve });
    });
  }, []);

  const closeConfirm = useCallback(
    (result: boolean) => {
      if (!confirmState) return;
      confirmState.resolve(result);
      setConfirmState(null);
    },
    [confirmState]
  );

  const value = useMemo<UxContextValue>(
    () => ({
      toast: {
        show: showToast,
        success: (message: string) => showToast(message, "success"),
        error: (message: string) => showToast(message, "error"),
        info: (message: string) => showToast(message, "info"),
      },
      confirm,
    }),
    [confirm, showToast]
  );

  return (
    <UxContext.Provider value={value}>
      {children}

      <div className="fixed top-4 right-4 z-[80] space-y-2 w-[min(360px,calc(100vw-2rem))]">
        {toasts.map((toast) => (
          <div
            key={toast.id}
            className={`rounded-xl border px-4 py-3 text-sm shadow-lg backdrop-blur bg-white/95 ${
              toast.tone === "error"
                ? "border-rose-200 text-rose-700"
                : toast.tone === "success"
                  ? "border-emerald-200 text-emerald-700"
                  : "border-slate-200 text-slate-700"
            }`}
          >
            {toast.message}
          </div>
        ))}
      </div>

      <Dialog open={!!confirmState} onOpenChange={(open) => !open && closeConfirm(false)}>
        <DialogContent showCloseButton={false} className="max-w-md rounded-2xl">
          <DialogHeader>
            <DialogTitle>{confirmState?.options.title || "请确认操作"}</DialogTitle>
            {confirmState?.options.description && (
              <DialogDescription>{confirmState.options.description}</DialogDescription>
            )}
          </DialogHeader>
          <DialogFooter className="pt-2">
            <Button variant="outline" onClick={() => closeConfirm(false)}>
              {confirmState?.options.cancelText || "取消"}
            </Button>
            <Button
              variant={confirmState?.options.tone === "danger" ? "destructive" : "default"}
              onClick={() => closeConfirm(true)}
            >
              {confirmState?.options.confirmText || "确认"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </UxContext.Provider>
  );
}

export function useUx() {
  const context = useContext(UxContext);
  if (!context) {
    throw new Error("useUx must be used within UxProvider");
  }
  return context;
}
