import { paths } from "./api-types";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export type ApiRequestConfig = Omit<RequestInit, "method" | "body">;

export class ApiError extends Error {
  public code?: number;
  public errorDetail?: { type: string; field?: string; reason?: string; details?: any };
  public requestId?: string;

  constructor(message: string, code?: number, errorDetail?: any, requestId?: string) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.errorDetail = errorDetail;
    this.requestId = requestId;
  }
}

async function request<T>(
  path: string,
  method: string,
  body?: any,
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

  const response = await fetch(url, {
    ...config,
    method,
    headers: {
      "Content-Type": "application/json",
      "X-Request-Id": requestId,
      ...config?.headers,
    },
    credentials: "include",
    body: body ? JSON.stringify(body) : undefined,
  });

  if (response.status === 204) {
    return {} as T;
  }

  const result = await response.json();

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
  get: <P extends keyof paths | (string & {})>(
    path: P,
    params?: any,
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
    return request<any>(url, "GET", undefined, config);
  },

  post: <P extends keyof paths | (string & {})>(
    path: P,
    body?: any,
    config?: ApiRequestConfig
  ) => request<any>(path as string, "POST", body, config),

  delete: <P extends keyof paths | (string & {})>(
    path: P,
    config?: ApiRequestConfig
  ) => request<any>(path as string, "DELETE", undefined, config),
};
