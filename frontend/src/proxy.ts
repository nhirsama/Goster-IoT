import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

type AuthSessionEnvelope = {
  code?: number;
  data?: {
    authenticated?: boolean;
  };
};

function buildApiUrl(path: string): string {
  const baseUrl = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/$/, "");
  let cleanPath = path.startsWith("/") ? path : `/${path}`;

  if (baseUrl.endsWith("/api/v1") && cleanPath.startsWith("/api/v1")) {
    cleanPath = cleanPath.substring(7);
  }

  return `${baseUrl}${cleanPath}`;
}

function isPublicPath(pathname: string): boolean {
  return (
    pathname.startsWith("/login") ||
    pathname.startsWith("/register") ||
    pathname.startsWith("/api") ||
    pathname.includes(".")
  );
}

async function isSessionValid(request: NextRequest): Promise<boolean> {
  const session = request.cookies.get("goster_session");
  if (!session?.value) {
    return false;
  }

  const authUrl = buildApiUrl("/api/v1/auth/me");

  try {
    const response = await fetch(authUrl, {
      method: "GET",
      headers: {
        cookie: request.headers.get("cookie") || "",
        "X-Request-Id": `proxy_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
      },
      cache: "no-store",
    });

    if (!response.ok) {
      return false;
    }

    const contentType = response.headers.get("content-type") || "";
    if (!contentType.includes("application/json")) {
      return false;
    }

    const result = (await response.json()) as AuthSessionEnvelope;
    if (result.code !== undefined && result.code !== 0) {
      return false;
    }

    return result.data?.authenticated !== false;
  } catch {
    return false;
  }
}

export async function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  if (isPublicPath(pathname)) {
    return NextResponse.next();
  }

  const valid = await isSessionValid(request);
  if (!valid) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico).*)"],
};
