"use client";

import { useEffect, useState } from "react";
import { DEFAULT_SHORTCUTS, loadShortcuts, formatShortcut, matchShortcut } from "@/lib/shortcuts";

export default function ShortcutsHelp() {
  const [open, setOpen] = useState(false);
  const [shortcuts, setShortcuts] = useState(DEFAULT_SHORTCUTS);
  const isMac = typeof navigator !== "undefined" && /Mac/.test(navigator.platform);

  useEffect(() => {
    setShortcuts(loadShortcuts());
  }, [open]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const s = loadShortcuts();
      if (matchShortcut(e, s.show_shortcuts)) {
        e.preventDefault();
        setOpen((v) => !v);
      }
      if (e.key === "Escape" && open) {
        setOpen(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  if (!open) return null;

  const groups = [
    { title: "Navigation", keys: ["global_search", "refresh"] },
    { title: "Actions", keys: ["new_item", "save", "toggle_lock"] },
    { title: "General", keys: ["show_shortcuts", "close_modal"] },
  ];

  return (
    <div
      className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-[70]"
      onClick={() => setOpen(false)}
      role="presentation"
    >
      <div
        className="bg-white dark:bg-slate-800 rounded-3xl shadow-2xl w-full max-w-md mx-4 overflow-hidden"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
      >
        <div className="bg-gradient-to-r from-indigo-600 to-purple-600 px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <i className="fa-solid fa-keyboard text-white text-lg"></i>
            <h2 className="text-lg font-semibold text-white">Keyboard Shortcuts</h2>
          </div>
          <button onClick={() => setOpen(false)} className="text-white/70 hover:text-white">
            <i className="fa-solid fa-xmark"></i>
          </button>
        </div>
        <div className="p-6 space-y-5 max-h-[60vh] overflow-y-auto">
          {groups.map((g) => (
            <div key={g.title}>
              <h3 className="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-2">{g.title}</h3>
              <div className="space-y-2">
                {g.keys.map((key) => {
                  const s = shortcuts[key];
                  if (!s) return null;
                  return (
                    <div key={key} className="flex items-center justify-between gap-3 text-sm">
                      <span className="text-slate-600 dark:text-slate-300">{s.description}</span>
                      <kbd className="px-2 py-1 bg-slate-100 dark:bg-slate-700 rounded-lg text-xs font-mono text-slate-600 dark:text-slate-300 border border-slate-200 dark:border-slate-600 shrink-0">
                        {formatShortcut(s, isMac)}
                      </kbd>
                    </div>
                  );
                })}
              </div>
            </div>
          ))}
          <p className="text-xs text-slate-400 pt-2 border-t border-slate-100 dark:border-slate-700">
            <a href="/settings#section-shortcuts" className="text-indigo-600 dark:text-indigo-400 hover:underline">
              Customize shortcuts
            </a>{" "}
            in Settings
          </p>
        </div>
      </div>
    </div>
  );
}

export function ShortcutsHelpButton() {
  const [open, setOpen] = useState(false);

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        className="hidden md:flex text-xs px-3 py-1.5 hover:bg-slate-100 dark:hover:bg-slate-700 rounded-lg items-center gap-1.5 text-slate-500 dark:text-slate-400"
        title="Keyboard shortcuts (Ctrl+/)"
      >
        <i className="fa-solid fa-keyboard"></i>
        <span>Shortcuts</span>
      </button>
      {open && <ShortcutsHelpPortal onClose={() => setOpen(false)} />}
    </>
  );
}

function ShortcutsHelpPortal({ onClose }: { onClose: () => void }) {
  const [shortcuts, setShortcuts] = useState(DEFAULT_SHORTCUTS);
  const isMac = typeof navigator !== "undefined" && /Mac/.test(navigator.platform);

  useEffect(() => {
    setShortcuts(loadShortcuts());
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const groups = [
    { title: "Navigation", keys: ["global_search", "refresh"] },
    { title: "Actions", keys: ["new_item", "save", "toggle_lock"] },
    { title: "General", keys: ["show_shortcuts", "close_modal"] },
  ];

  return (
    <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-[70]" onClick={onClose}>
      <div className="bg-white dark:bg-slate-800 rounded-3xl shadow-2xl w-full max-w-md mx-4 overflow-hidden" onClick={(e) => e.stopPropagation()}>
        <div className="bg-gradient-to-r from-indigo-600 to-purple-600 px-6 py-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-white"><i className="fa-solid fa-keyboard mr-2"></i>Shortcuts</h2>
          <button onClick={onClose} className="text-white/70 hover:text-white"><i className="fa-solid fa-xmark"></i></button>
        </div>
        <div className="p-6 space-y-4">
          {groups.map((g) => (
            <div key={g.title}>
              <h3 className="text-xs font-semibold text-slate-400 uppercase mb-2">{g.title}</h3>
              {g.keys.map((key) => {
                const s = shortcuts[key];
                if (!s) return null;
                return (
                  <div key={key} className="flex justify-between text-sm py-1">
                    <span className="text-slate-600 dark:text-slate-300">{s.label}</span>
                    <kbd className="text-xs font-mono bg-slate-100 dark:bg-slate-700 px-2 py-0.5 rounded">{formatShortcut(s, isMac)}</kbd>
                  </div>
                );
              })}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}