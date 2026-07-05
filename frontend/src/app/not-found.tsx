"use client";

import Link from "next/link";

export default function NotFound() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-white dark:bg-slate-900">
      <div className="text-center space-y-4">
        <div className="text-6xl font-bold text-slate-200 dark:text-slate-700">404</div>
        <h1 className="text-xl font-semibold text-slate-900 dark:text-slate-100">Page not found</h1>
        <p className="text-sm text-slate-500 dark:text-slate-400">The page you&apos;re looking for doesn&apos;t exist.</p>
        <Link href="/dashboard" className="inline-block px-5 py-2 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium rounded-xl transition-colors">
          Back to Dashboard
        </Link>
      </div>
    </div>
  );
}
