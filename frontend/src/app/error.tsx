"use client";

import Link from "next/link";

export default function ErrorPage({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <div className="min-h-screen flex items-center justify-center bg-white dark:bg-slate-900">
      <div className="text-center space-y-4 max-w-md px-4">
        <div className="text-6xl font-bold text-red-200 dark:text-red-800">!</div>
        <h1 className="text-xl font-semibold text-slate-900 dark:text-slate-100">Something went wrong</h1>
        <p className="text-sm text-slate-500 dark:text-slate-400">{error.message || "An unexpected error occurred."}</p>
        <div className="flex items-center justify-center gap-3">
          <button onClick={reset} className="px-5 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium rounded-xl transition-colors">
            Try again
          </button>
          <Link href="/dashboard" className="px-5 py-2 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 text-slate-700 dark:text-slate-300 text-sm font-medium rounded-xl transition-colors">
            Dashboard
          </Link>
        </div>
      </div>
    </div>
  );
}
