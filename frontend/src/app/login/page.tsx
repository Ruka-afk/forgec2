"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

export default function LoginPage() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const router = useRouter();

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);

    const params = new URLSearchParams();
    params.append("username", username);
    params.append("password", password);

    const response = await fetch("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: params.toString(),
    });

    if (response.ok) {
      router.push("/dashboard");
      router.refresh();
    } else {
      const data = await response.json().catch(() => ({}));
      setError(data.error || "Login failed");
    }

    setLoading(false);
  }

  return (
    <div className="min-h-screen flex items-center justify-center relative overflow-hidden"
      style={{ background: "linear-gradient(135deg, #f8fafc 0%, #e2e8f0 100%)" }}>
      {/* Grid decoration */}
      <div className="absolute inset-0"
        style={{
          backgroundImage: "linear-gradient(rgba(99,102,241,0.04) 1px, transparent 1px), linear-gradient(90deg, rgba(99,102,241,0.04) 1px, transparent 1px)",
          backgroundSize: "40px 40px",
        }} />

      <div className="relative z-10 w-full max-w-sm mx-4">
        {/* Brand */}
        <div className="text-center mb-8">
          <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-gradient-to-br from-indigo-500 to-indigo-700 flex items-center justify-center shadow-lg shadow-indigo-500/20">
            <i className="fa-solid fa-shield-halved text-white text-2xl"></i>
          </div>
          <h1 className="text-2xl font-bold tracking-tight" style={{ color: "var(--text-primary)" }}>
            Forge<span className="text-indigo-500">C2</span>
          </h1>
          <p className="text-sm mt-1" style={{ color: "var(--text-tertiary)" }}>Professional Red Team C2 Framework</p>
        </div>

        {/* Login card */}
        <div className="bg-[var(--card-bg)] border border-[var(--border)] rounded-2xl p-8 shadow-lg">
          {error && (
            <div className="mb-5 p-3 rounded-xl text-xs font-medium flex items-center gap-2"
              style={{ background: "var(--danger-bg)", color: "var(--danger-text)" }}>
              <i className="fa-solid fa-circle-exclamation"></i>
              {error}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label htmlFor="login-username" className="block text-xs font-medium mb-1.5" style={{ color: "var(--text-secondary)" }}>Username</label>
              <div className="relative">
                <i className="fa-solid fa-user absolute left-3 top-1/2 -translate-y-1/2 text-xs" style={{ color: "var(--text-tertiary)" }}></i>
                <input
                  id="login-username"
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full pl-9 pr-4 h-11 rounded-xl text-sm outline-none transition-all duration-150"
                  style={{
                    background: "var(--background)",
                    border: "1px solid var(--border)",
                    color: "var(--text-primary)",
                  }}
                  onFocus={(e) => { e.target.style.borderColor = "#6366f1"; e.target.style.boxShadow = "0 0 0 3px rgba(99,102,241,0.1)"; }}
                  onBlur={(e) => { e.target.style.borderColor = "var(--border)"; e.target.style.boxShadow = "none"; }}
                  required
                />
              </div>
            </div>

            <div>
              <label htmlFor="login-password" className="block text-xs font-medium mb-1.5" style={{ color: "var(--text-secondary)" }}>Password</label>
              <div className="relative">
                <i className="fa-solid fa-lock absolute left-3 top-1/2 -translate-y-1/2 text-xs" style={{ color: "var(--text-tertiary)" }}></i>
                <input
                  id="login-password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full pl-9 pr-4 h-11 rounded-xl text-sm outline-none transition-all duration-150"
                  style={{
                    background: "var(--background)",
                    border: "1px solid var(--border)",
                    color: "var(--text-primary)",
                  }}
                  onFocus={(e) => { e.target.style.borderColor = "#6366f1"; e.target.style.boxShadow = "0 0 0 3px rgba(99,102,241,0.1)"; }}
                  onBlur={(e) => { e.target.style.borderColor = "var(--border)"; e.target.style.boxShadow = "none"; }}
                  required
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full h-11 bg-indigo-600 hover:bg-indigo-700 active:bg-indigo-800 text-white font-medium rounded-xl text-sm transition-all duration-150 disabled:opacity-50 disabled:cursor-not-allowed mt-2"
            >
              {loading ? (
                <span className="flex items-center justify-center gap-2">
                  <i className="fa-solid fa-circle-notch fa-spin"></i>
                  Signing in...
                </span>
              ) : "Sign in"}
            </button>
          </form>
        </div>

        <p className="text-center text-xs mt-6" style={{ color: "var(--text-tertiary)" }}>
          ForgeC2 v2.0 — Authorized access only
        </p>
      </div>
    </div>
  );
}
