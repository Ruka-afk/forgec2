import { Spinner } from "@/components/UI";

export default function RootLoading() {
  return (
    <div className="flex items-center justify-center h-screen w-screen bg-background">
      <div className="flex flex-col items-center gap-4">
        <Spinner size="lg" />
        <p className="text-sm text-muted-foreground">Loading ForgeC2...</p>
      </div>
    </div>
  );
}
