"use client";

import { useEffect, useState } from "react";
import { renderMarkdown } from "@/lib/markdown";
import { sanitizeHtml } from "@/lib/sanitize";
import { MdContent } from "@/components/ui/md-content";
export function SanitizedMarkdown({ content }: { content: string }) {
  const [safe, setSafe] = useState<string>("");
  useEffect(() => {
    let cancelled = false;
    sanitizeHtml(renderMarkdown(content)).then((h) => {
      if (!cancelled) setSafe(h);
    });
    return () => {
      cancelled = true;
    };
  }, [content]);
  return <MdContent dangerouslySetInnerHTML={{ __html: safe }} />;
}
