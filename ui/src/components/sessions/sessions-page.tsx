import { useState } from "react";
import { useNavigate } from "react-router";
import { useSessions, useDiscoverWorktrees } from "@/hooks/use-sessions";
import { useProjects } from "@/hooks/use-projects";
import { useCloseAgent, useResumeAgent } from "@/hooks/use-agent";
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
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { EmptyState } from "@/components/shared/empty-state";
import { TimeAgo } from "@/components/shared/time-ago";
import { SyncButton, MergeButton, DeleteWorktreeButton } from "./session-actions";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import type { SessionStatus } from "@/lib/types";

const STATUS_TABS: { label: string; value: string; statuses?: SessionStatus[] }[] = [
  { label: "All", value: "all" },
  { label: "Active", value: "active", statuses: ["active"] },
  { label: "Idle", value: "idle", statuses: ["idle"] },
  { label: "Completed", value: "completed", statuses: ["completed"] },
  { label: "Abandoned", value: "abandoned", statuses: ["abandoned"] },
];

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

function conflictColor(state: string): string {
  switch (state) {
    case "sync_conflict":
      return "bg-orange-100 text-orange-800 dark:bg-orange-900/40 dark:text-orange-300";
    case "merge_conflict":
      return "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300";
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
  const navigate = useNavigate();
  const [statusTab, setStatusTab] = useState("all");
  const [projectFilter, setProjectFilter] = useState<string>("");

  const activeTab = STATUS_TABS.find((t) => t.value === statusTab);
  const { data, isLoading, error } = useSessions(
    projectFilter || undefined,
    activeTab?.statuses,
  );
  const sessions = data ?? [];

  const { data: projects } = useProjects();
  const closeAgent = useCloseAgent();
  const resumeAgent = useResumeAgent();
  const discover = useDiscoverWorktrees();

  const handleClose = (sessionId: string, status: "idle" | "completed" | "abandoned") => {
    closeAgent.mutate(
      { session_id: sessionId, status },
      {
        onSuccess: () => toast.success(`Session ${status}`),
        onError: (err) => toast.error(`Failed: ${(err as Error).message}`),
      },
    );
  };

  const handleResume = (sessionId: string) => {
    resumeAgent.mutate(
      { session_id: sessionId },
      {
        onSuccess: (data) => {
          toast.success("Session resumed");
          if (data.command) {
            navigator.clipboard.writeText(data.command).then(
              () => toast.info("Command copied to clipboard"),
              () => {},
            );
          }
        },
        onError: (err) => toast.error(`Failed: ${(err as Error).message}`),
      },
    );
  };

  const handleDiscover = () => {
    discover.mutate(projectFilter || undefined, {
      onSuccess: (data) => {
        if (data.count > 0) {
          toast.success(`Discovered ${data.count} worktree(s)`);
        } else {
          toast.info("No new worktrees found");
        }
      },
      onError: (err) => toast.error(`Discovery failed: ${(err as Error).message}`),
    });
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
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold tracking-tight">Sessions</h2>
        <Button
          variant="outline"
          size="sm"
          onClick={handleDiscover}
          disabled={discover.isPending}
        >
          {discover.isPending ? "Discovering..." : "Discover Worktrees"}
        </Button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-4">
        <Tabs value={statusTab} onValueChange={setStatusTab}>
          <TabsList>
            {STATUS_TABS.map((tab) => (
              <TabsTrigger key={tab.value} value={tab.value}>
                {tab.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        {projects && projects.length > 0 && (
          <Select value={projectFilter} onValueChange={setProjectFilter}>
            <SelectTrigger size="sm" className="w-[180px]">
              <SelectValue placeholder="All projects" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">All projects</SelectItem>
              {projects.map((p) => (
                <SelectItem key={p.ID} value={p.ID}>
                  {p.Name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </div>

      {/* Table */}
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
              <TableHead>Project</TableHead>
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
              <TableRow
                key={s.ID}
                className="cursor-pointer"
                onClick={() => navigate(`/sessions/${s.ID}`)}
              >
                <TableCell className="font-mono text-xs">
                  <span>{s.Branch || "\u2014"}</span>
                  {s.Discovered && (
                    <Badge
                      variant="outline"
                      className="ml-1.5 text-[10px] bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300"
                    >
                      discovered
                    </Badge>
                  )}
                  {s.ConflictState !== "none" && (
                    <Badge
                      variant="outline"
                      className={cn("ml-1.5 text-[10px]", conflictColor(s.ConflictState))}
                    >
                      {s.ConflictState === "sync_conflict" ? "sync conflict" : "merge conflict"}
                    </Badge>
                  )}
                </TableCell>
                <TableCell className="text-sm">{s.ProjectName || "\u2014"}</TableCell>
                <TableCell>
                  <Badge
                    variant="outline"
                    className={cn(sessionColor(s.Status), "text-xs")}
                  >
                    {s.Status}
                  </Badge>
                </TableCell>
                <TableCell className="text-sm max-w-[200px] truncate">
                  {s.Outcome || "\u2014"}
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
                    <div
                      className="flex items-center gap-1 flex-wrap"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {s.Status === "idle" && (
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-7 text-xs"
                          onClick={() => handleResume(s.ID)}
                          disabled={resumeAgent.isPending}
                        >
                          Resume
                        </Button>
                      )}
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
                      <SyncButton session={s} />
                      <MergeButton session={s} />
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
