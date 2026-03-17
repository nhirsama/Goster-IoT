import { paths } from "./api-types";

export type ApiRequestConfig = Omit<RequestInit, "method" | "body">;
type ApiPrimitive = string | number | boolean | null | undefined;
type ApiParams = Record<string, ApiPrimitive>;
const TENANT_STORAGE_KEY = "goster_tenant_id";
let tenantMemoryFallback: string | undefined;
type ApiErrorDetail = {
  type: string;
  field?: string;
  reason?: string;
  details?: Record<string, unknown>;
};
type ApiEnvelope<T> = {
  code?: number;
  message?: string;
  data: T;
  error?: ApiErrorDetail;
  request_id?: string;
};

function normalizeTenantId(value?: string | null): string | undefined {
  const normalized = value?.trim();
  return normalized ? normalized : undefined;
}

function getTenantStorage(): Storage | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }
  const storage = window.localStorage as Partial<Storage> | undefined;
  if (!storage) {
    return undefined;
  }
  if (
    typeof storage.getItem !== "function" ||
    typeof storage.setItem !== "function" ||
    typeof storage.removeItem !== "function"
  ) {
    return undefined;
  }
  return storage as Storage;
}

export function getActiveTenantId(): string | undefined {
  const storage = getTenantStorage();
  if (storage) {
    return normalizeTenantId(storage.getItem(TENANT_STORAGE_KEY)) || tenantMemoryFallback;
  }
  return tenantMemoryFallback || normalizeTenantId(process.env.NEXT_PUBLIC_DEFAULT_TENANT_ID);
}

export function setActiveTenantId(tenantId?: string | null): void {
  const normalized = normalizeTenantId(tenantId);
  tenantMemoryFallback = normalized;
  const storage = getTenantStorage();
  if (!storage) {
    return;
  }
  if (normalized) {
    storage.setItem(TENANT_STORAGE_KEY, normalized);
    return;
  }
  storage.removeItem(TENANT_STORAGE_KEY);
}

export class ApiError extends Error {
  public code?: number;
  public errorDetail?: ApiErrorDetail;
  public requestId?: string;

  constructor(message: string, code?: number, errorDetail?: ApiErrorDetail, requestId?: string) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.errorDetail = errorDetail;
    this.requestId = requestId;
  }
}

export function getApiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const reason = error.errorDetail?.reason;
    return reason || error.message || fallback;
  }
  if (error instanceof Error) {
    return error.message || fallback;
  }
  return fallback;
}

async function request<T>(
  path: string,
  method: string,
  body?: unknown,
  config?: ApiRequestConfig
): Promise<T> {
  // 智能拼接：如果 path 已经包含 /api/v1 且 baseUrl 也包含，则进行去重
  const baseUrl = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/$/, "");
  let cleanPath = path.startsWith("/") ? path : `/${path}`;
  
  // 如果 baseUrl 以 /api/v1 结尾，且 cleanPath 以 /api/v1 开头
  if (baseUrl.endsWith("/api/v1") && cleanPath.startsWith("/api/v1")) {
    cleanPath = cleanPath.substring(7); // 移除开头的 /api/v1
  }

  const url = `${baseUrl}${cleanPath}`;
  const requestId = `req_${Math.floor(Date.now() / 1000)}_${Math.random().toString(36).substring(2, 10)}`;
  const tenantId = getActiveTenantId();

  const response = await fetch(url, {
    ...config,
    method,
    headers: {
      "Content-Type": "application/json",
      "X-Request-Id": requestId,
      ...(tenantId ? { "X-Tenant-Id": tenantId } : {}),
      ...config?.headers,
    },
    credentials: "include",
    body: body ? JSON.stringify(body) : undefined,
  });

  if (response.status === 204) {
    return {} as T;
  }

  const result = (await response.json()) as ApiEnvelope<T>;

  if (!response.ok || (result.code !== undefined && result.code !== 0)) {
    throw new ApiError(
      result.message || response.statusText || "Request failed",
      result.code,
      result.error,
      result.request_id
    );
  }

  return result.data as T;
}

export const api = {
  get: <T = unknown, P extends keyof paths | (string & {}) = keyof paths | (string & {})>(
    path: P,
    params?: ApiParams,
    config?: ApiRequestConfig
  ) => {
    let url = path as string;
    if (params) {
      const searchParams = new URLSearchParams();
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined && value !== null) {
          searchParams.append(key, String(value));
        }
      });
      const query = searchParams.toString();
      if (query) url += `?${query}`;
    }
    return request<T>(url, "GET", undefined, config);
  },

  post: <T = unknown, P extends keyof paths | (string & {}) = keyof paths | (string & {})>(
    path: P,
    body?: unknown,
    config?: ApiRequestConfig
  ) => request<T>(path as string, "POST", body, config),

  delete: <T = unknown, P extends keyof paths | (string & {}) = keyof paths | (string & {})>(
    path: P,
    config?: ApiRequestConfig
  ) => request<T>(path as string, "DELETE", undefined, config),
};
