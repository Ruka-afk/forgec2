import { ToastMessage } from "./types";

export default function Toast({ toasts }: { toasts: ToastMessage[] }) {
  if (toasts.length === 0) return null;
  return (
    <div className="fixed top-4 right-4 z-50 space-y-2">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={`px-4 py-3 rounded-xl shadow-lg text-sm font-medium flex items-center gap-2 animate-pulse ${
            t.type === "success" ? "bg-emerald-600 text-white" :
            t.type === "error" ? "bg-red-600 text-white" :
            "bg-indigo-600 text-white"
          }`}
        >
          <i className={`fa-solid ${t.type === "success" ? "fa-check-circle" : t.type === "error" ? "fa-exclamation-circle" : "fa-info-circle"}`}></i>
          {t.text}
        </div>
      ))}
    </div>
  );
}
