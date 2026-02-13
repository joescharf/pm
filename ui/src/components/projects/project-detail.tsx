import { useState } from "react";
import { Link, useParams, useNavigate } from "react-router";
import { Pencil, Trash2, ExternalLink, Plus } from "lucide-react";
import { toast } from "sonner";
import { useProject, useDeleteProject } from "@/hooks/use-projects";
import { useProjectIssues } from "@/hooks/use-issues";
import { useSessions } from "@/hooks/use-sessions";
import { useCloseAgent } from "@/hooks/use-agent";
import { useProjectHealth } from "@/hooks/use-status";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
import { TimeAgo } from "@/components/shared/time-ago";
import { EmptyState } from "@/components/shared/empty-state";
import { cn } from "@/lib/utils";
import { HealthChart } from "./health-chart";
import { ProjectForm } from "./project-form";
import { IssueForm } from "@/components/issues/issue-form";
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

export function ProjectDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [editOpen, setEditOpen] = useState(false);
  const [createIssueOpen, setCreateIssueOpen] = useState(false);

  const { data: project, isLoading, error } = useProject(id!);
  const { data: healthData, isLoading: healthLoading } = useProjectHealth(id!);
  const { data: issuesData } = useProjectIssues(id!);
  const { data: sessionsData } = useSessions(id!);
  const deleteProject = useDeleteProject();
  const closeAgent = useCloseAgent();

  const issues = issuesData ?? [];
  const sessions = sessionsData ?? [];
  const worktrees = sessions.filter(
    (s) => s.Status === "active" || s.Status === "idle"
  );

  const handleClose = (sessionId: string, status: "idle" | "completed" | "abandoned") => {
    closeAgent.mutate(
      { session_id: sessionId, status },
      {
        onSuccess: () => toast.success(`Session ${status}`),
        onError: (err) => toast.error(`Failed: ${(err as Error).message}`),
      }
    );
  };

  function handleDelete() {
    if (
      !window.confirm(
        `Delete project "${project?.Name}"? This cannot be undone.`,
      )
    ) {
      return;
    }
    deleteProject.mutate(id!, {
      onSuccess: () => {
        toast("Project deleted");
        navigate("/projects");
      },
      onError: (err) =>
        toast.error(`Failed to delete project: ${(err as Error).message}`),
    });
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-64 rounded-lg" />
        <Skeleton className="h-48 rounded-xl" />
        <Skeleton className="h-64 rounded-xl" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-destructive text-sm">
        Failed to load project: {(error as Error).message}
      </div>
    );
  }

  if (!project) {
    return (
      <div className="text-muted-foreground text-sm">Project not found.</div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">
            {project.Name}
          </h2>
          {project.Description && (
            <p className="text-muted-foreground text-sm mt-1">
              {project.Description}
            </p>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => setEditOpen(true)}>
            <Pencil />
            Edit
          </Button>
          <Button
            variant="destructive"
            onClick={handleDelete}
            disabled={deleteProject.isPending}
          >
            <Trash2 />
            Delete
          </Button>
        </div>
      </div>

      {/* Metadata Card */}
      <Card>
        <CardHeader>
          <CardTitle>Project Info</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-6 gap-y-3 text-sm">
            <div>
              <dt className="text-muted-foreground">Path</dt>
              <dd className="font-mono text-xs mt-0.5">
                {project.Path || "—"}
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Language</dt>
              <dd className="mt-0.5">{project.Language || "—"}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Repository</dt>
              <dd className="mt-0.5">
                {project.RepoURL ? (
                  <a
                    href={project.RepoURL}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 hover:underline text-primary"
                  >
                    {project.RepoURL}
                    <ExternalLink className="size-3" />
                  </a>
                ) : (
                  "—"
                )}
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Group</dt>
              <dd className="mt-0.5">{project.GroupName || "—"}</dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Created</dt>
              <dd className="mt-0.5">
                <TimeAgo date={project.CreatedAt} />
              </dd>
            </div>
            <div>
              <dt className="text-muted-foreground">Updated</dt>
              <dd className="mt-0.5">
                <TimeAgo date={project.UpdatedAt} />
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      {/* Issues, Worktrees & Sessions Tabs */}
      <Tabs defaultValue="issues">
        <div className="flex items-center justify-between">
          <TabsList>
            <TabsTrigger value="issues">
              Issues{issues.length > 0 && ` (${issues.length})`}
            </TabsTrigger>
            <TabsTrigger value="worktrees">
              Worktrees{worktrees.length > 0 && ` (${worktrees.length})`}
            </TabsTrigger>
            <TabsTrigger value="sessions">
              Sessions{sessions.length > 0 && ` (${sessions.length})`}
            </TabsTrigger>
          </TabsList>
          <Button variant="outline" size="sm" onClick={() => setCreateIssueOpen(true)}>
            <Plus />
            New Issue
          </Button>
        </div>

        <TabsContent value="issues">
          {issues.length === 0 ? (
            <EmptyState message="No issues for this project" />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Title</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Priority</TableHead>
                  <TableHead>Type</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {issues.map((issue) => (
                  <TableRow key={issue.ID}>
                    <TableCell>
                      <Link
                        to={`/issues/${issue.ID}`}
                        className="font-medium hover:underline"
                      >
                        {issue.Title}
                      </Link>
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary" className="text-xs">
                        {issue.Status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-xs">
                        {issue.Priority}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline" className="text-xs">
                        {issue.Type}
                      </Badge>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </TabsContent>

        <TabsContent value="worktrees">
          {worktrees.length === 0 ? (
            <EmptyState message="No active worktrees for this project" />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Branch</TableHead>
                  <TableHead>Issue</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Started</TableHead>
                  <TableHead>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {worktrees.map((wt) => (
                  <TableRow key={wt.ID}>
                    <TableCell className="font-mono text-xs">
                      {wt.Branch || "—"}
                    </TableCell>
                    <TableCell>
                      {wt.IssueID ? (
                        <Link
                          to={`/issues/${wt.IssueID}`}
                          className="font-mono text-xs hover:underline"
                        >
                          {wt.IssueID.slice(0, 12)}
                        </Link>
                      ) : (
                        <span className="text-muted-foreground text-xs">—</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant="outline"
                        className={cn(sessionColor(wt.Status), "text-xs")}
                      >
                        {wt.Status}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <TimeAgo
                        date={wt.StartedAt}
                        className="text-xs text-muted-foreground"
                      />
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1">
                        {wt.Status === "active" && (
                          <Button
                            variant="outline"
                            size="sm"
                            className="h-7 text-xs"
                            onClick={() => handleClose(wt.ID, "idle")}
                            disabled={closeAgent.isPending}
                          >
                            Pause
                          </Button>
                        )}
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-7 text-xs"
                          onClick={() => handleClose(wt.ID, "completed")}
                          disabled={closeAgent.isPending}
                        >
                          Done
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          className="h-7 text-xs text-destructive"
                          onClick={() => handleClose(wt.ID, "abandoned")}
                          disabled={closeAgent.isPending}
                        >
                          Abandon
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </TabsContent>

        <TabsContent value="sessions">
          {sessions.length === 0 ? (
            <EmptyState message="No agent sessions for this project" />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Branch</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-center">Commits</TableHead>
                  <TableHead>Started</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sessions.map((session) => (
                  <TableRow key={session.ID}>
                    <TableCell className="font-mono text-xs">
                      {session.Branch || "—"}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant="outline"
                        className={cn(sessionColor(session.Status), "text-xs")}
                      >
                        {session.Status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-center">
                      {session.CommitCount}
                    </TableCell>
                    <TableCell>
                      <TimeAgo
                        date={session.StartedAt}
                        className="text-xs text-muted-foreground"
                      />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </TabsContent>
      </Tabs>

      {/* Health Score (lower prominence) */}
      <Card>
        <CardHeader>
          <CardTitle>Health Score</CardTitle>
        </CardHeader>
        <CardContent>
          {healthLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-6 rounded" />
              ))}
            </div>
          ) : healthData ? (
            <HealthChart health={healthData} />
          ) : (
            <p className="text-muted-foreground text-sm">
              No health data available.
            </p>
          )}
        </CardContent>
      </Card>

      {/* Edit Dialog */}
      <ProjectForm
        open={editOpen}
        onOpenChange={setEditOpen}
        project={project}
      />

      {/* Create Issue Dialog */}
      <IssueForm
        open={createIssueOpen}
        onOpenChange={setCreateIssueOpen}
        defaultProjectId={id}
      />
    </div>
  );
}
