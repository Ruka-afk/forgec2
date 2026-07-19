import { NextResponse } from "next/server";
import { cookies } from "next/headers";

async function handleProxy(request: Request, method: string) {
  const GO_BACKEND = process.env.GO_BACKEND_URL || "http://127.0.0.1:8443";

  const cookieStore = await cookies();
  const sessionCookie = cookieStore.get("forgec2_session");

  const url = new URL(request.url);
  let targetPath = url.pathname.replace(/^\/api\/go\/?/, "") || "/";

  // Forward as-is. The Go backend registers routes at the root level
  // (/agents, /phishing/campaigns, /roles, /bof, /monitor/alerts, ...)
  // and some under /api/ or /api/v1/. Paths that already carry an
  // /api/ prefix (e.g. /api/v1/dashboard, /api/tokens) are passed
  // through unchanged.

  const forwardParams = new URLSearchParams();
  url.searchParams.forEach((value, key) => {
    forwardParams.append(key, value);
  });
  const qs = forwardParams.toString();
  const targetUrl = `${GO_BACKEND}/${targetPath}${qs ? "?" + qs : ""}`;

  const headers: Record<string, string> = {};
  headers["Accept"] = "application/json";
  if (sessionCookie) {
    headers["Cookie"] = `forgec2_session=${sessionCookie.value}`;
  }

  const reqContentType = request.headers.get("Content-Type") || "";
  if (reqContentType) {
    headers["Content-Type"] = reqContentType;
  }
  let body: BodyInit | undefined;
  if (method !== "GET") {
    if (reqContentType.includes("multipart/form-data")) {
      body = await request.blob();
    } else {
      body = await request.text();
    }
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 30000);
  let response;
  try {
    response = await fetch(targetUrl, {
      method,
      headers,
      body,
      redirect: "manual",
      signal: controller.signal,
    });
  } finally {
    clearTimeout(timeout);
  }

  const contentType = response.headers.get("Content-Type") || "";
  const isBinary = contentType.startsWith("application/octet-stream") ||
    contentType.includes("x-msdownload") ||
    contentType.includes("exe") ||
    contentType.includes("zip") ||
    contentType.includes("pdf") ||
    contentType.startsWith("image/");

  if (isBinary) {
    const buffer = await response.arrayBuffer();
    const resHeaders = new Headers();
    response.headers.forEach((value, key) => {
      if (key.toLowerCase() !== "content-encoding" && key.toLowerCase() !== "transfer-encoding") {
        resHeaders.set(key, value);
      }
    });
    return new NextResponse(buffer, {
      status: response.status,
      headers: resHeaders,
    });
  }

  const responseBody = await response.text();
  const resHeaders = new Headers({
    "Content-Type": contentType || "application/json",
  });
  const setCookie = response.headers.get("set-cookie");
  if (setCookie) {
    resHeaders.set("Set-Cookie", setCookie);
  }
  return new NextResponse(responseBody, {
    status: response.status,
    headers: resHeaders,
  });
}

export async function GET(request: Request) {
  return handleProxy(request, "GET");
}
export async function POST(request: Request) {
  return handleProxy(request, "POST");
}
export async function PUT(request: Request) {
  return handleProxy(request, "PUT");
}
export async function DELETE(request: Request) {
  return handleProxy(request, "DELETE");
}
