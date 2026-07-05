"use client";

export default function DocsPage() {
  return (
    <div className="max-w-7xl mx-auto mb-20 md:mb-0 h-[calc(100vh-8rem)]">
      <div className="flex items-center gap-3 mb-4">
        <div className="w-10 h-10 bg-gradient-to-br from-sky-500 to-indigo-600 rounded-xl flex items-center justify-center shadow-lg shadow-sky-500/20">
          <i className="fa-solid fa-book text-white text-sm"></i>
        </div>
        <div>
          <h1 className="text-xl font-bold text-slate-900 dark:text-slate-100">API Documentation</h1>
          <p className="text-slate-500 dark:text-slate-400 text-xs">ForgeC2 REST API "OpenAPI / Swagger UI</p>
        </div>
        <a
          href="/api/go?p=/api/docs/openapi.yaml"
          target="_blank"
          rel="noopener noreferrer"
          className="ml-auto px-3 py-1.5 text-xs bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 dark:hover:bg-slate-600 rounded-lg text-slate-600 dark:text-slate-300"
        >
          <i className="fa-solid fa-download mr-1"></i>OpenAPI YAML
        </a>
      </div>
      <div className="ui-card overflow-hidden shadow-sm h-[calc(100%-4rem)]">
        <iframe
          src="/api/go?p=/api/docs/"
          title="ForgeC2 API Docs"
          className="w-full h-full border-0"
        />
      </div>
    </div>
  );
}