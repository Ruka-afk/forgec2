import { memo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { Image, Send } from "lucide-react";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import CollectCard from "./CollectCard";
import { useCollectTask } from "./useCollectTask";

interface WallpaperSectionProps {
  agentId: string;
  online: boolean;
}

const STYLES = ["fill", "fit", "stretch", "tile", "center", "span"] as const;

export default memo(function WallpaperSection({ agentId, online }: WallpaperSectionProps) {
  const { t } = useI18n();
  const { busy, result, collect } = useCollectTask(agentId);
  const [image, setImage] = useState("");
  const [style, setStyle] = useState<string>("fill");

  const handleSet = async () => {
    const src = image.trim();
    if (!src) return;
    await collect("wallpaper", paths.agents.wallpaper(agentId), {
      body: { image: src, style },
      emptyText: t("agents.wallpaper_done"),
      successText: t("agents.wallpaper_done"),
      errorText: t("agents.wallpaper_failed"),
    });
  };

  return (
    <CollectCard
      title={t("agents.wallpaper_title")}
      icon={<Image className="size-3.5" />}
      emptyIcon={Image}
      emptyTitle={t("agents.wallpaper_empty")}
      emptyHint={t("agents.wallpaper_empty_hint")}
      result={result}
    >
      <div className="flex flex-wrap gap-2">
        <Input
          value={image}
          onChange={(e) => setImage(e.target.value)}
          placeholder={t("agents.wallpaper_image_ph")}
          aria-label={t("agents.wallpaper_image_ph")}
          className="min-w-0 flex-1 font-mono text-xs"
        />
        <Select value={style} onValueChange={(v) => v !== null && setStyle(v)}>
          <SelectTrigger className="h-9 w-28" aria-label={t("agents.wallpaper_style")}> 
            <SelectValue placeholder={t("agents.wallpaper_style")} />
          </SelectTrigger>
          <SelectContent>
            {STYLES.map((s) => (
              <SelectItem key={s} value={s}>
                {s}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button size="sm" disabled={!online || busy !== null || !image.trim()} onClick={() => void handleSet()}>
          {busy !== null ? (
            <>
              <Spinner size="xs" /> {t("agents.wallpaper_setting")}
            </>
          ) : (
            <>
              <Send className="size-4" /> {t("agents.wallpaper_set")}
            </>
          )}
        </Button>
      </div>
    </CollectCard>
  );
});
