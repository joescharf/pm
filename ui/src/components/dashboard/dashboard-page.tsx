import { RefreshCw } from "lucide-react";
import { toast } from "sonner";
import { useStatusOverview } from "@/hooks/use-status";
import { useRefreshAllProjects } from "@/hooks/use-projects";
import { Button } from "@/components/ui/button";
import { StatsCards } from "./stats-cards";
import { StatusTable } from "./status-table";
import { Skeleton } from "@/components/ui/skeleton";

export function DashboardPage() {
  const { data, isLoading, error } = useStatusOverview();
  const refreshAll = useRefreshAllProjects();
  const entries = data ?? [];

  function handleRefresh() {
    refreshAll.mutate(undefined, {
      onSuccess: (result) => {
        if (result.failed > 0) {
          toast.warning(
            `Refreshed ${result.refreshed} of ${result.total} projects, ${result.failed} failed`
          );
        } else {
          toast.success(
            `Refreshed ${result.refreshed} of ${result.total} projects`
          );
        }
      },
      onError: (err) => {
        toast.error(`Refresh failed: ${(err as Error).message}`);
      },
    });
  }

  if (error) {
    return (
      <div className="text-destructive text-sm">
        Failed to load status: {(error as Error).message}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold tracking-tight">Dashboard</h2>
        <Button
          variant="outline"
          size="sm"
          onClick={handleRefresh}
          disabled={refreshAll.isPending}
        >
          <RefreshCw
            className={`size-4 mr-2 ${refreshAll.isPending ? "animate-spin" : ""}`}
          />
          {refreshAll.isPending ? "Refreshing..." : "Refresh All"}
        </Button>
      </div>
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
