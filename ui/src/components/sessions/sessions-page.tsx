import { useSessions } from "@/hooks/use-sessions";
import { useCloseAgent } from "@/hooks/use-agent";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/empty-state";
import { TimeAgo } from "@/components/shared/time-ago";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import type { SessionStatus } from "@/lib/types";

function sessionColor(status: SessionStatus): string {
  switch (status) {
    case "active":
      return "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300";
    case "idle":
      return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300";
    case "completed":
      return "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300";
    case "abandoned":
      return "bg-gray-100 text-gray-800 dark:bg-gray-900/40 dark:text-gray-300";
    default:
      return "";
  }
}

function formatDuration(start: string, end: string | null): string {
  if (!end) return "active";
  const ms = new Date(end).getTime() - new Date(start).getTime();
  const sec = Math.floor(ms / 1000);
  if (sec < 60) return `${sec}s`;
  const min = Math.floor(sec / 60);
  if (min < 60) return `${min}m`;
  const hr = Math.floor(min / 60);
  return `${hr}h ${min % 60}m`;
}

export function SessionsPage() {
  const { data, isLoading, error } = useSessions();
  const sessions = data ?? [];
  const closeAgent = useCloseAgent();

  const handleClose = (sessionId: string, status: "idle" | "completed" | "abandoned") => {
    closeAgent.mutate(
      { session_id: sessionId, status },
      {
        onSuccess: () => toast.success(`Session ${status}`),
        onError: (err) => toast.error(`Failed: ${(err as Error).message}`),
      }
    );
  };

  if (error) {
    return (
      <div className="text-destructive text-sm">
        Failed to load sessions: {(error as Error).message}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold tracking-tight">Sessions</h2>

      {isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-10 rounded" />
          ))}
        </div>
      ) : sessions.length === 0 ? (
        <EmptyState message="No agent sessions recorded" />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Branch</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Outcome</TableHead>
              <TableHead className="text-center">Commits</TableHead>
              <TableHead>Duration</TableHead>
              <TableHead>Started</TableHead>
              <TableHead>Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sessions.map((s) => (
              <TableRow key={s.ID}>
                <TableCell className="font-mono text-xs">
                  {s.Branch || "—"}
                </TableCell>
                <TableCell>
                  <Badge
                    variant="outline"
                    className={cn(sessionColor(s.Status), "text-xs")}
                  >
                    {s.Status}
                  </Badge>
                </TableCell>
                <TableCell className="text-sm max-w-[200px] truncate">
                  {s.Outcome || "—"}
                </TableCell>
                <TableCell className="text-center">{s.CommitCount}</TableCell>
                <TableCell className="text-xs">
                  {formatDuration(s.StartedAt, s.EndedAt)}
                </TableCell>
                <TableCell>
                  <TimeAgo
                    date={s.StartedAt}
                    className="text-xs text-muted-foreground"
                  />
                </TableCell>
                <TableCell>
                  {(s.Status === "active" || s.Status === "idle") && (
                    <div className="flex items-center gap-1">
                      {s.Status === "active" && (
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-7 text-xs"
                          onClick={() => handleClose(s.ID, "idle")}
                          disabled={closeAgent.isPending}
                        >
                          Pause
                        </Button>
                      )}
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-xs"
                        onClick={() => handleClose(s.ID, "completed")}
                        disabled={closeAgent.isPending}
                      >
                        Done
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-7 text-xs text-destructive"
                        onClick={() => handleClose(s.ID, "abandoned")}
                        disabled={closeAgent.isPending}
                      >
                        Abandon
                      </Button>
                    </div>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
