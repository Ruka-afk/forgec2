"use client";

import React, { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { ChevronUp } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default React.memo(function ScrollToTop() {
  const { t } = useI18n();
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const main = document.querySelector("main");
    if (!main) return;
    const onScroll = () => setVisible(main.scrollTop > 300);
    main.addEventListener("scroll", onScroll, { passive: true });
    return () => main.removeEventListener("scroll", onScroll);
  }, []);

  if (!visible) return null;

  return (
    <Button
      variant="outline"
      size="icon"
      onClick={() => document.querySelector("main")?.scrollTo({ top: 0, behavior: "smooth" })}
      className="fixed bottom-6 right-6 z-(--z-scroll-to-top) w-11 h-11 rounded-full shadow-lg backdrop-blur-sm bg-background/80 hover:text-foreground hover:shadow-xl transition-all animate-scale-in"
      aria-label={t("common.scroll_to_top")}
    >
      <ChevronUp className="w-4 h-4" />
    </Button>
  );
});
