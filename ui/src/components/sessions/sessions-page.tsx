import { useSessions } from "@/hooks/use-sessions";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/empty-state";
import { TimeAgo } from "@/components/shared/time-ago";
import { cn } from "@/lib/utils";
import type { SessionStatus } from "@/lib/types";

function sessionColor(status: SessionStatus): string {
  switch (status) {
    case "running":
      return "bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300";
    case "completed":
      return "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300";
    case "failed":
      return "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300";
    case "abandoned":
      return "bg-gray-100 text-gray-800 dark:bg-gray-900/40 dark:text-gray-300";
    default:
      return "";
  }
}

function formatDuration(start: string, end: string | null): string {
  if (!end) return "running";
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
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
