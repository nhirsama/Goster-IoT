import { cookies } from "next/headers";
import { components } from "@/lib/api-types";

type AuthSession = components["schemas"]["AuthSession"];

type ApiEnvelope<T> = {
  code?: number;
  data?: T;
};

function buildApiUrl(path: string): string {
  const baseUrl = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/$/, "");
  let cleanPath = path.startsWith("/") ? path : `/${path}`;

  if (baseUrl.endsWith("/api/v1") && cleanPath.startsWith("/api/v1")) {
    cleanPath = cleanPath.substring(7);
  }

  return `${baseUrl}${cleanPath}`;
}

export async function getServerAuthSession(): Promise<AuthSession | null> {
  const cookieStore = await cookies();
  const sessionCookie = cookieStore.get("goster_session");

  if (!sessionCookie?.value) {
    return null;
  }

  const authUrl = buildApiUrl("/api/v1/auth/me");
  const cookieHeader = cookieStore
    .getAll()
    .map((item) => `${item.name}=${item.value}`)
    .join("; ");

  try {
    const response = await fetch(authUrl, {
      method: "GET",
      headers: {
        cookie: cookieHeader,
      },
      cache: "no-store",
    });

    if (!response.ok) {
      return null;
    }

    const contentType = response.headers.get("content-type") || "";
    if (!contentType.includes("application/json")) {
      return null;
    }

    const result = (await response.json()) as ApiEnvelope<AuthSession>;
    if (result.code !== undefined && result.code !== 0) {
      return null;
    }

    if (!result.data || result.data.authenticated === false) {
      return null;
    }

    return result.data;
  } catch {
    return null;
  }
}
