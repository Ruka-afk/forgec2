import { NextResponse } from "next/server";

export async function POST(request: Request) {
  const GO_BACKEND = process.env.GO_BACKEND_URL || "http://127.0.0.1:8000";

  const body = await request.text();

  const response = await fetch(`${GO_BACKEND}/login`, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
    },
    body: body,
    redirect: "manual",
  });

  if (response.ok || response.status === 302) {
    const res = NextResponse.json({ success: true }, { status: 200 });

    // Parse all Set-Cookie headers (Node.js join-joined string)
    const rawCookies = response.headers.get("set-cookie");
    if (rawCookies) {
      // Split on ", " before a cookie name pattern (name=) to get individual cookies
      const cookieEntries = rawCookies.split(/,(?=\s*[A-Za-z0-9_-]+=)/);
      for (const entry of cookieEntries) {
        const parts = entry.trim().split(";");
        const nameValue = parts[0];
        const eqIdx = nameValue.indexOf("=");
        if (eqIdx < 0) continue;
        const name = nameValue.substring(0, eqIdx).trim();
        const value = nameValue.substring(eqIdx + 1).trim();
        res.cookies.set(name, value, {
          path: "/",
          httpOnly: true,
          secure: process.env.NODE_ENV === "production",
          sameSite: "lax",
        });
      }
    }

    return res;
  }

  return NextResponse.json(
    { success: false, error: "Invalid username or password" },
    { status: 401 }
  );
}
