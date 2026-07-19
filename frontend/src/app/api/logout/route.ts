import { NextResponse } from "next/server";

export async function POST() {
  const GO_BACKEND = process.env.GO_BACKEND_URL || "http://127.0.0.1:8443";

  await fetch(`${GO_BACKEND}/logout`, {
    method: "POST",
    redirect: "manual",
  });

  const res = NextResponse.json({ success: true }, { status: 200 });
  res.cookies.delete("forgec2_session");
  return res;
}
