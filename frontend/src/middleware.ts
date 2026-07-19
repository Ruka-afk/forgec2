import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const PUBLIC_ROUTES = ["/login", "/api/login"];
const STATIC_PREFIXES = ["/_next/", "/favicon", "/js/", "/css/"];

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  const isPublic = PUBLIC_ROUTES.some((r) => pathname === r || pathname.startsWith(r));
  if (isPublic) return NextResponse.next();

  const isStatic = STATIC_PREFIXES.some((p) => pathname.startsWith(p));
  if (isStatic) return NextResponse.next();

  const sessionCookie = request.cookies.get("forgec2_session");
  if (!sessionCookie?.value) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("redirect", pathname);
    return NextResponse.redirect(loginUrl);
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api/).*)"],
};
