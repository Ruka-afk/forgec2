import { NextResponse } from "next/server";
import { cookies } from "next/headers";

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

async function handleProxy(request: Request, method: string) {
  const GO_BACKEND = process.env.GO_BACKEND_URL || "http://127.0.0.1:8080";

  const cookieStore = await cookies();
  const sessionCookie = cookieStore.get("forgec2_session");

  const url = new URL(request.url);
  const targetPath = url.searchParams.get("p") || "/";

  const forwardParams = new URLSearchParams();
  url.searchParams.forEach((value, key) => {
    if (key !== "p") forwardParams.append(key, value);
  });
  const queryString = forwardParams.toString();
  const targetUrl = `${GO_BACKEND}${targetPath}${queryString ? "?" + queryString : ""}`;

  const headers: Record<string, string> = {};
  if (method === "GET") {
    headers["Accept"] = "application/json";
  }

  if (sessionCookie) {
    headers["Cookie"] = `forgec2_session=${sessionCookie.value}`;
  }

  const reqContentType = request.headers.get("Content-Type") || "";
  let body: BodyInit | undefined;
  if (method !== "GET" && method !== "DELETE") {
    if (reqContentType.includes("multipart/form-data")) {
      body = await request.blob();
    } else {
      body = await request.text();
      if (body) {
        headers["Content-Type"] = reqContentType || "application/json";
      }
    }
  }

  const response = await fetch(targetUrl, {
    method,
    headers,
    body,
    redirect: "manual",
  });

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
  return new NextResponse(responseBody, {
    status: response.status,
    headers: {
      "Content-Type": contentType || "application/json",
    },
  });
}
