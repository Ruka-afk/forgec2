import { Skeleton } from "@/components/ui/skeleton";

export default function DnsLoading() {
  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up space-y-6 p-4 sm:p-6">
      <Skeleton className="h-8 w-48" />
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 sm:gap-5">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-32 rounded-lg" />
        ))}
      </div>
      <Skeleton className="h-20 rounded-lg" />
      <Skeleton className="h-48 rounded-lg" />
    </div>
  );
}
