import { useStatusOverview } from "@/hooks/use-status";
import { StatsCards } from "./stats-cards";
import { StatusTable } from "./status-table";
import { Skeleton } from "@/components/ui/skeleton";

export function DashboardPage() {
  const { data, isLoading, error } = useStatusOverview();
  const entries = data ?? [];

  if (error) {
    return (
      <div className="text-destructive text-sm">
        Failed to load status: {(error as Error).message}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold tracking-tight">Dashboard</h2>
      {isLoading ? (
        <div className="space-y-4">
          <div className="grid gap-4 grid-cols-2 lg:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-24 rounded-xl" />
            ))}
          </div>
          <Skeleton className="h-64 rounded-xl" />
        </div>
      ) : (
        <>
          <StatsCards entries={entries} />
          <StatusTable entries={entries} />
        </>
      )}
    </div>
  );
}
