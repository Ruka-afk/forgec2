import { Skeleton } from "@/components/ui/skeleton";

export default function DnsLoading() {
  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 space-y-6 p-4 sm:p-6">
      <Skeleton className="h-8 w-48" />
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-32 rounded-xl" />
        ))}
      </div>
      <Skeleton className="h-64 rounded-xl" />
    </div>
  );
}
