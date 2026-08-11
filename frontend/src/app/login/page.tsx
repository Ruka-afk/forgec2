"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Spinner } from "@/components/UI";
import { AlertCircle, Lock, Shield, User } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useForm } from "@/lib/hooks/useForm";
import { isLoginSuccessResponse, parseLoginErrorBody, safeNextPath } from "@/lib/login";
import { z } from "zod";

type LoginFormValues = {
  username: string;
  password: string;
  totpCode: string;
  rememberMe: boolean;
};

function LoginForm() {
  const { t } = useI18n();
  const [error, setError] = useState("");
  const [version, setVersion] = useState("");
  const router = useRouter();
  const searchParams = useSearchParams();

  const loginSchema = useMemo(
    () =>
      z.object({
        username: z.string().min(1, t("login.username_required")),
        password: z.string().min(1, t("login.password_required")),
        totpCode: z.string().regex(/^[0-9]{0,6}$/, t("login.totp_invalid")),
        rememberMe: z.boolean(),
      }),
    [t],
  );

  const {
    values,
    errors,
    touched,
    isSubmitting,
    handleChange,
    handleBlur,
    handleSubmit,
    setFieldValue,
  } = useForm<LoginFormValues>({
    initialValues: { username: "", password: "", totpCode: "", rememberMe: false },
    schema: loginSchema,
    onSubmit: async (vals) => {
      setError("");
      const params = new URLSearchParams();
      params.append("username", vals.username);
      params.append("password", vals.password);
      if (vals.totpCode) params.append("totp_code", vals.totpCode);
      if (vals.rememberMe) params.append("remember_me", "on");

      try {
        const response = await fetch("/login", {
          method: "POST",
          headers: {
            "Content-Type": "application/x-www-form-urlencoded",
            Accept: "application/json, text/html, */*",
          },
          body: params.toString(),
          credentials: "include",
          redirect: "manual",
        });

        if (isLoginSuccessResponse(response)) {
          const dest = safeNextPath(searchParams.get("next"));
          router.push(dest);
          router.refresh();
          return;
        }

        let msg = t("login.error_failed");
        try {
          const ct = response.headers.get("content-type") || "";
          if (ct.includes("application/json")) {
            const data = await response.json();
            const parsed = parseLoginErrorBody(data);
            if (parsed) msg = parsed;
          }
        } catch { /* keep default */ }
        setError(msg);
      } catch {
        setError(t("login.error_network"));
      }
    },
  });

  useEffect(() => {
    if (searchParams.get("expired") === "1") {
      setError(t("login.session_expired"));
    }
  }, [searchParams, t]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch("/health", { credentials: "include" });
        if (!res.ok) return;
        const data = await res.json() as { version?: string };
        if (!cancelled && data.version) setVersion(String(data.version));
      } catch { /* ignore */ }
    })();
    return () => { cancelled = true; };
  }, []);

  return (
    <div className="min-h-screen flex items-center justify-center relative overflow-hidden bg-gradient-to-br from-background via-muted/30 to-primary/5 dark:from-background dark:via-primary/[0.03] dark:to-primary/8">
      <div className="absolute rounded-full blur-(--blur-orb) opacity-15 dark:opacity-10 w-72 h-72 bg-primary/30 top-[-10%] left-[-5%] animate-orb-float" style={{ animationDelay: "0s" }} />
      <div className="absolute rounded-full blur-(--blur-orb) opacity-15 dark:opacity-10 w-56 h-56 bg-primary/20 bottom-[-5%] right-[-3%] animate-orb-float" style={{ animationDelay: "2s" }} />
      <div className="absolute rounded-full blur-(--blur-orb) opacity-15 dark:opacity-10 w-40 h-40 bg-violet-400/15 top-[40%] right-[20%] animate-orb-float" style={{ animationDelay: "4s" }} />

      <div className="absolute inset-0 opacity-[0.03] dark:opacity-[0.04] [background-image:linear-gradient(var(--border)_1px,transparent_1px),linear-gradient(90deg,var(--border)_1px,transparent_1px)] [background-size:40px_40px]" />

      <div className="relative z-10 w-full max-w-[22rem] mx-4 animate-fade-slide-up">
        <div className="text-center mb-8">
          <div className="w-16 h-16 mx-auto mb-4 rounded-xl bg-gradient-to-br from-primary to-primary/70 flex items-center justify-center shadow-xl shadow-primary/25 ring-1 ring-primary/20 animate-float">
            <Shield className="w-7 h-7 text-primary-foreground" aria-hidden="true" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">
            Forge<span className="text-primary">C2</span>
          </h1>
          <p className="text-sm mt-1.5 text-muted-foreground/70">{t("login.subtitle")}</p>
        </div>

        <Card className="bg-card/80 backdrop-blur-xl border-border/60 shadow-2xl shadow-black/5 dark:shadow-black/20">
          <CardContent className="p-7">
          {error && (
            <Alert variant="destructive" className="mb-5 animate-scale-in" role="alert">
              <AlertCircle className="w-4 h-4" aria-hidden="true" />
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <form onSubmit={handleSubmit} className="space-y-4" aria-label={t("login.form_aria")} noValidate>
            <div>
              <Label htmlFor="login-username" className="text-xs font-medium text-muted-foreground mb-1.5 block">{t("login.username")}</Label>
              <div className="relative group">
                <User className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none z-10 transition-colors group-focus-within:text-primary" aria-hidden="true" />
                <Input
                  id="login-username"
                  type="text"
                  autoFocus
                  value={values.username}
                  onChange={handleChange("username")}
                  onBlur={handleBlur("username")}
                  className="h-11 pl-9 transition-colors"
                  aria-invalid={!!(touched.username && errors.username)}
                  aria-describedby={errors.username ? "login-username-error" : undefined}
                  autoComplete="username"
                />
              </div>
              {touched.username && errors.username && (
                <p id="login-username-error" role="alert" className="text-xs text-destructive mt-1">{errors.username}</p>
              )}
            </div>

            <div>
              <Label htmlFor="login-password" className="text-xs font-medium text-muted-foreground mb-1.5 block">{t("login.password")}</Label>
              <div className="relative group">
                <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none z-10 transition-colors group-focus-within:text-primary" aria-hidden="true" />
                <Input
                  id="login-password"
                  type="password"
                  value={values.password}
                  onChange={handleChange("password")}
                  onBlur={handleBlur("password")}
                  className="h-11 pl-9 transition-colors"
                  aria-invalid={!!(touched.password && errors.password)}
                  aria-describedby={errors.password ? "login-password-error" : undefined}
                  autoComplete="current-password"
                />
              </div>
              {touched.password && errors.password && (
                <p id="login-password-error" role="alert" className="text-xs text-destructive mt-1">{errors.password}</p>
              )}
            </div>

            <div>
              <Label htmlFor="login-totp" className="text-xs font-medium text-muted-foreground mb-1.5 block">
                {t("login.totp")} <span className="text-muted-foreground/70">({t("login.totp_optional")})</span>
              </Label>
              <div className="relative group">
                <Shield className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none z-10 transition-colors group-focus-within:text-primary" aria-hidden="true" />
                <Input
                  id="login-totp"
                  type="text"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  maxLength={6}
                  pattern="[0-9]{6}"
                  value={values.totpCode}
                  onChange={handleChange("totpCode")}
                  onBlur={handleBlur("totpCode")}
                  placeholder="000000"
                  className="h-11 pl-9 font-mono tracking-(--tracking-wide) text-center transition-colors"
                  aria-invalid={!!(touched.totpCode && errors.totpCode)}
                  aria-describedby={errors.totpCode ? "login-totp-error" : undefined}
                />
              </div>
              {touched.totpCode && errors.totpCode && (
                <p id="login-totp-error" role="alert" className="text-xs text-destructive mt-1">{errors.totpCode}</p>
              )}
            </div>

            <label className="flex items-center gap-2 cursor-pointer select-none">
              <Checkbox
                checked={values.rememberMe}
                onCheckedChange={(v) => setFieldValue("rememberMe", v === true)}
                aria-label={t("login.remember_me")}
              />
              <span className="text-xs text-muted-foreground">{t("login.remember_me")}</span>
            </label>

            <Button
              type="submit"
              disabled={isSubmitting}
              className="w-full h-11 font-semibold transition-all duration-200 shadow-lg shadow-primary/20 hover:shadow-xl hover:shadow-primary/30 active:scale-[0.98]"
            >
              {isSubmitting ? (
                <span className="flex items-center justify-center gap-2">
                  <Spinner size="sm" />
                  {t("login.signing_in")}
                </span>
              ) : t("login.sign_in")}
            </Button>
          </form>
          </CardContent>
        </Card>

        <p className="text-center text-xs mt-6 text-muted-foreground/70">
          {version
            ? t("login.footer_version", { version })
            : t("login.footer")}
        </p>
      </div>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={<div className="min-h-screen flex items-center justify-center"><Spinner /></div>}>
      <LoginForm />
    </Suspense>
  );
}
