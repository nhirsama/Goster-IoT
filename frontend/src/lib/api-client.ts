import { paths } from "./api-types";

export type ApiRequestConfig = Omit<RequestInit, "method" | "body">;
type ApiPrimitive = string | number | boolean | null | undefined;
type ApiParams = Record<string, ApiPrimitive>;
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
  const hasJsonBody = body !== undefined && body !== null;
  const requestHeaders = new Headers(config?.headers);

  if (hasJsonBody && !requestHeaders.has("Content-Type")) {
    requestHeaders.set("Content-Type", "application/json");
  }
  requestHeaders.set("X-Request-Id", requestId);

  let response: Response;
  try {
    response = await fetch(url, {
      ...config,
      method,
      headers: requestHeaders,
      credentials: "include",
      body: hasJsonBody ? JSON.stringify(body) : undefined,
    });
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : "Network request failed";
    throw new ApiError(message, -1, { type: "network_error", reason: message }, requestId);
  }

  if (response.status === 204) {
    return {} as T;
  }

  const contentType = response.headers?.get?.("content-type") || "";
  const isJsonResponse = contentType.includes("application/json");
  let result: ApiEnvelope<T> | null = null;
  let fallbackMessage = "";

  if (isJsonResponse) {
    try {
      result = (await response.json()) as ApiEnvelope<T>;
    } catch {
      if (!response.ok) {
        throw new ApiError(response.statusText || "Request failed", response.status, undefined, requestId);
      }
      throw new ApiError("Invalid JSON response", response.status, undefined, requestId);
    }
  } else if (!contentType) {
    try {
      result = (await response.json()) as ApiEnvelope<T>;
    } catch {
      fallbackMessage = await response.text().catch(() => "");
    }
  } else {
    fallbackMessage = await response.text().catch(() => "");
  }

  if (!response.ok) {
    throw new ApiError(
      result?.message || fallbackMessage || response.statusText || "Request failed",
      result?.code ?? response.status,
      result?.error,
      result?.request_id || requestId
    );
  }

  if (!result) {
    return {} as T;
  }

  if (result.code !== undefined && result.code !== 0) {
    throw new ApiError(
      result.message || response.statusText || "Request failed",
      result.code,
      result.error,
      result.request_id || requestId
    );
  }

  if (!("data" in result)) {
    return result as T;
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
