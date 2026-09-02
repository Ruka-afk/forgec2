"use client";

import { memo, useEffect, useRef, useState } from "react";
import { renderMarkdown } from "@/lib/markdown";
import { sanitizeHtml } from "@/lib/sanitize";
import { MdContent } from "@/components/ui/md-content";

const MAX_MARKDOWN_CHARS = 80_000;

function SanitizedMarkdownInner({ content, live = false }: { content: string; live?: boolean }) {
  const [safe, setSafe] = useState<string>("");
  const [renderFailed, setRenderFailed] = useState(false);
  const lastRenderedAtRef = useRef(0);
  const src = content.length > MAX_MARKDOWN_CHARS
    ? `${content.slice(0, MAX_MARKDOWN_CHARS)}\n\n…`
    : content;
  useEffect(() => {
    let cancelled = false;
    const run = async () => {
      try {
        const html = await sanitizeHtml(renderMarkdown(src));
        if (!cancelled) {
          lastRenderedAtRef.current = Date.now();
          setSafe(html);
          setRenderFailed(false);
        }
      } catch {
        // A lazy DOMPurify load or an unusual markdown payload must not take
        // down the whole AI workspace. Fall back to React-escaped plain text.
        if (!cancelled) setRenderFailed(true);
      }
    };
    if (!live) {
      void run();
      return () => {
        cancelled = true;
      };
    }
    // Throttle rather than debounce. Debouncing restarted the timer on every
    // token, leaving long continuously streamed answers blank until the end.
    const elapsed = Date.now() - lastRenderedAtRef.current;
    const timer = window.setTimeout(() => { void run(); }, Math.max(0, 120 - elapsed));
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [src, live]);
  if (renderFailed) {
    return <p className="whitespace-pre-wrap break-words text-sm leading-6">{src}</p>;
  }
  return <MdContent dangerouslySetInnerHTML={{ __html: safe }} />;
}

export const SanitizedMarkdown = memo(SanitizedMarkdownInner);
