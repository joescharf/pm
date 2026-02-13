import { useParams, Link } from "react-router";
import { useSession } from "@/hooks/use-sessions";
import { useCloseAgent, useResumeAgent } from "@/hooks/use-agent";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
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

export function SessionDetail() {
  const { id } = useParams<{ id: string }>();
  const { data: session, isLoading, error } = useSession(id!);
  const closeAgent = useCloseAgent();
  const resumeAgent = useResumeAgent();

  const handleClose = (status: "idle" | "completed" | "abandoned") => {
    closeAgent.mutate(
      { session_id: id!, status },
      {
        onSuccess: () => toast.success(`Session ${status}`),
        onError: (err) => toast.error(`Failed: ${(err as Error).message}`),
      }
    );
  };

  const handleResume = () => {
    resumeAgent.mutate(
      { session_id: id! },
      {
        onSuccess: (data) => {
          toast.success("Session resumed");
          if (data.command) {
            navigator.clipboard.writeText(data.command).then(
              () => toast.info("Command copied to clipboard"),
              () => {}
            );
          }
        },
        onError: (err) => toast.error(`Failed: ${(err as Error).message}`),
      }
    );
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-64 rounded-lg" />
        <Skeleton className="h-48 rounded-xl" />
        <Skeleton className="h-48 rounded-xl" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-destructive text-sm">
        Failed to load session: {(error as Error).message}
      </div>
    );
  }

  if (!session) {
    return (
      <div className="text-muted-foreground text-sm">Session not found.</div>
    );
  }

  const isLive = session.Status === "active" || session.Status === "idle";

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">
            Session: {session.Branch || session.ID.slice(0, 12)}
          </h2>
          <p className="text-muted-foreground text-sm mt-1">
            {session.ProjectName && (
              <>
                Project:{" "}
                <Link
                  to={`/projects/${session.ProjectID}`}
                  className="hover:underline text-primary"
                >
                  {session.ProjectName}
                </Link>
                {" \u00B7 "}
              </>
            )}
            ID: {session.ID.slice(0, 12)}
          </p>
        </div>
        {isLive && (
          <div className="flex items-center gap-2">
            {session.Status === "idle" && (
              <Button
                onClick={handleResume}
                disabled={resumeAgent.isPending}
              >
                Resume
              </Button>
            )}
            {session.Status === "active" && (
              <Button
                variant="outline"
                onClick={() => handleClose("idle")}
                disabled={closeAgent.isPending}
              >
                Pause
              </Button>
            )}
            <Button
              variant="outline"
              onClick={() => handleClose("completed")}
              disabled={closeAgent.isPending}
            >
              Done
            </Button>
            <Button
              variant="destructive"
              onClick={() => handleClose("abandoned")}
              disabled={closeAgent.isPending}
            >
              Abandon
            </Button>
          </div>
        )}
      </div>

      {/* Session Metadata */}
      <Card>
        <CardHeader>
          <CardTitle>Session Info</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3 text-sm">
            <div>
              <dt className="text-muted-foreground">Status</dt>
              <dd className="mt-0.5">
                <Badge
                  variant="outline"
                  className={cn(sessionColor(session.Status), "text-xs")}
                >
                  {session.Status}
                </Badge>
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Branch</dt>
              <dd className="font-mono text-xs mt-0.5">
                {session.Branch || "\u2014"}
              </dd>
            </div>
            {session.Outcome && (
              <div>
                <dt className="text-muted-foreground">Outcome</dt>
                <dd className="mt-0.5">{session.Outcome}</dd>
              </div>
            )}
            <div>
              <dt className="text-muted-foreground">Commits</dt>
              <dd className="mt-0.5">{session.CommitCount}</dd>
            </div>
            {session.IssueID && (
              <div>
                <dt className="text-muted-foreground">Issue</dt>
                <dd className="mt-0.5">
                  <Link
                    to={`/issues/${session.IssueID}`}
                    className="font-mono text-xs hover:underline text-primary"
                  >
                    {session.IssueID.slice(0, 12)}
                  </Link>
                </dd>
              </div>
            )}
            <div>
              <dt className="text-muted-foreground">Worktree</dt>
              <dd className="font-mono text-xs mt-0.5">
                {session.WorktreePath || "\u2014"}
                {session.WorktreeExists !== undefined && (
                  <Badge
                    variant="outline"
                    className={cn(
                      "ml-2 text-xs",
                      session.WorktreeExists
                        ? "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300"
                        : "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300"
                    )}
                  >
                    {session.WorktreeExists ? "exists" : "missing"}
                  </Badge>
                )}
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      {/* Git State */}
      <Card>
        <CardHeader>
          <CardTitle>Git State</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3 text-sm">
            {session.LastCommitHash && (
              <div>
                <dt className="text-muted-foreground">Last Commit</dt>
                <dd className="font-mono text-xs mt-0.5">
                  {session.LastCommitHash}{" "}
                  <span className="text-muted-foreground">
                    {session.LastCommitMessage}
                  </span>
                </dd>
              </div>
            )}
            {session.CurrentBranch && (
              <div>
                <dt className="text-muted-foreground">Current Branch</dt>
                <dd className="font-mono text-xs mt-0.5">
                  {session.CurrentBranch}
                </dd>
              </div>
            )}
            {session.IsDirty !== undefined && session.WorktreeExists && (
              <div>
                <dt className="text-muted-foreground">Working Tree</dt>
                <dd className="mt-0.5">
                  <Badge
                    variant="outline"
                    className={cn(
                      "text-xs",
                      session.IsDirty
                        ? "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300"
                        : "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300"
                    )}
                  >
                    {session.IsDirty ? "dirty" : "clean"}
                  </Badge>
                </dd>
              </div>
            )}
            {(session.AheadCount !== undefined ||
              session.BehindCount !== undefined) &&
              session.WorktreeExists && (
                <div>
                  <dt className="text-muted-foreground">Ahead / Behind</dt>
                  <dd className="mt-0.5">
                    <span className="text-green-600 dark:text-green-400">
                      +{session.AheadCount ?? 0}
                    </span>
                    {" / "}
                    <span className="text-red-600 dark:text-red-400">
                      -{session.BehindCount ?? 0}
                    </span>
                  </dd>
                </div>
              )}
          </dl>
        </CardContent>
      </Card>

      {/* Timeline */}
      <Card>
        <CardHeader>
          <CardTitle>Timeline</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-1 sm:grid-cols-3 gap-x-6 gap-y-3 text-sm">
            <div>
              <dt className="text-muted-foreground">Started</dt>
              <dd className="mt-0.5">
                <TimeAgo date={session.StartedAt} />
              </dd>
            </div>
            {session.LastActiveAt && (
              <div>
                <dt className="text-muted-foreground">Last Active</dt>
                <dd className="mt-0.5">
                  <TimeAgo date={session.LastActiveAt} />
                </dd>
              </div>
            )}
            {session.EndedAt && (
              <div>
                <dt className="text-muted-foreground">Ended</dt>
                <dd className="mt-0.5">
                  <TimeAgo date={session.EndedAt} />
                </dd>
              </div>
            )}
          </dl>
        </CardContent>
      </Card>
    </div>
  );
}
