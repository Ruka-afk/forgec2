import { NextResponse } from "next/server";

export async function POST(request: Request) {
  const GO_BACKEND = process.env.GO_BACKEND_URL || "http://127.0.0.1:8080";

  const body = await request.text();

  const response = await fetch(`${GO_BACKEND}/login`, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
    },
    body: body,
    redirect: "manual",
  });

  const setCookie = response.headers.get("set-cookie");

  if ((response.ok || response.status === 302) && setCookie) {
    const res = NextResponse.json({ success: true }, { status: 200 });
    const cookieParts = setCookie.split(";")[0];
    const eqIdx = cookieParts.indexOf("=");
    const name = cookieParts.substring(0, eqIdx);
    const value = cookieParts.substring(eqIdx + 1);
    res.cookies.set(name, value, {
      path: "/",
      httpOnly: true,
      maxAge: 86400,
      sameSite: "lax",
    });
    return res;
  }

  return NextResponse.json(
    { success: false, error: "Invalid username or password" },
    { status: 401 }
  );
}
