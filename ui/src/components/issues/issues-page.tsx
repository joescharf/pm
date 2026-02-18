import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router";
import { ChevronDown, ChevronRight, Plus, Rocket, Trash2, X } from "lucide-react";
import { toast } from "sonner";
import { useIssues, useUpdateIssue, useDeleteIssue, useBulkUpdateIssueStatus, useBulkDeleteIssues } from "@/hooks/use-issues";
import { useProjects } from "@/hooks/use-projects";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { EmptyState } from "@/components/shared/empty-state";
import { TimeAgo } from "@/components/shared/time-ago";
import { StatusBadge, PriorityBadge } from "@/components/issues/status-badge";
import { IssueFilters } from "@/components/issues/issue-filters";
import { IssueForm } from "@/components/issues/issue-form";
import { AgentLaunchDialog } from "@/components/issues/agent-launch-dialog";
import type { Issue, IssueStatus, IssuePriority, Project } from "@/lib/types";

interface FilterValues {
  status?: IssueStatus;
  priority?: IssuePriority;
  tag?: string;
}

const typeLabels: Record<string, string> = {
  feature: "Feature",
  bug: "Bug",
  chore: "Chore",
};

interface ProjectGroup {
  project: Project;
  issues: Issue[];
}

export function IssuesPage() {
  const [filters, setFilters] = useState<FilterValues>({});
  const [showClosed, setShowClosed] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [collapsedProjects, setCollapsedProjects] = useState<Set<string>>(
    new Set()
  );
  const [selectedIssues, setSelectedIssues] = useState<Set<string>>(new Set());
  const [launchDialogOpen, setLaunchDialogOpen] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const { data: issuesData, isLoading: issuesLoading, error } = useIssues(filters);
  const { data: projectsData, isLoading: projectsLoading } = useProjects();
  const updateIssue = useUpdateIssue();
  const deleteIssue = useDeleteIssue();
  const bulkUpdateStatus = useBulkUpdateIssueStatus();
  const bulkDelete = useBulkDeleteIssues();
  const allIssues = issuesData ?? [];
  const projects = projectsData ?? [];
  const isLoading = issuesLoading || projectsLoading;

  const issues = useMemo(() => {
    if (showClosed) return allIssues;
    return allIssues.filter((i) => i.Status !== "closed");
  }, [allIssues, showClosed]);

  const groups = useMemo(() => {
    const projectMap = new Map<string, Project>();
    for (const p of projects) {
      projectMap.set(p.ID, p);
    }

    const groupMap = new Map<string, ProjectGroup>();
    for (const issue of issues) {
      const existing = groupMap.get(issue.ProjectID);
      if (existing) {
        existing.issues.push(issue);
      } else {
        const project = projectMap.get(issue.ProjectID);
        if (project) {
          groupMap.set(issue.ProjectID, { project, issues: [issue] });
        }
      }
    }

    // Sort groups by project name
    return Array.from(groupMap.values()).sort((a, b) =>
      a.project.Name.localeCompare(b.project.Name)
    );
  }, [issues, projects]);

  const selectedProjectIds = useMemo(() => {
    const ids = new Set<string>();
    for (const issue of issues) {
      if (selectedIssues.has(issue.ID)) {
        ids.add(issue.ProjectID);
      }
    }
    return ids;
  }, [selectedIssues, issues]);

  const canLaunchAgent = selectedProjectIds.size === 1;

  const toggleCollapsed = (projectId: string) => {
    setCollapsedProjects((prev) => {
      const next = new Set(prev);
      if (next.has(projectId)) {
        next.delete(projectId);
      } else {
        next.add(projectId);
      }
      return next;
    });
  };

  const toggleIssueSelection = (issue: Issue) => {
    setSelectedIssues((prev) => {
      const next = new Set(prev);
      if (next.has(issue.ID)) {
        next.delete(issue.ID);
      } else {
        next.add(issue.ID);
      }
      return next;
    });
  };

  const clearSelection = () => {
    setSelectedIssues(new Set());
  };

  async function handleBulkStatusChange(status: IssueStatus) {
    const ids = Array.from(selectedIssues);
    try {
      await bulkUpdateStatus.mutateAsync({ ids, status });
      toast.success(`Updated ${ids.length} issue${ids.length > 1 ? "s" : ""} to ${status.replace("_", " ")}`);
      clearSelection();
    } catch (err) {
      toast.error(`Failed to update issues: ${(err as Error).message}`);
    }
  }

  async function handleBulkDelete() {
    const ids = Array.from(selectedIssues);
    try {
      await bulkDelete.mutateAsync(ids);
      toast.success(`Deleted ${ids.length} issue${ids.length > 1 ? "s" : ""}`);
      clearSelection();
    } catch (err) {
      toast.error(`Failed to delete issues: ${(err as Error).message}`);
    }
    setDeleteConfirmOpen(false);
  }

  useEffect(() => {
    clearSelection();
  }, [filters]);

  if (error) {
    return (
      <div className="text-destructive text-sm">
        Failed to load issues: {(error as Error).message}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold tracking-tight">Issues</h2>
        <Button onClick={() => setFormOpen(true)}>
          <Plus />
          Add Issue
        </Button>
      </div>

      <div className="flex items-center gap-3">
        <IssueFilters filters={filters} onChange={setFilters} />
        <label className="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer ml-auto">
          <input
            type="checkbox"
            checked={showClosed}
            onChange={(e) => setShowClosed(e.target.checked)}
            className="rounded border-gray-300"
          />
          Show Closed
        </label>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 rounded-lg" />
          ))}
        </div>
      ) : issues.length === 0 ? (
        <EmptyState message="No issues found" />
      ) : (
        <div className="space-y-6">
          {groups.map(({ project, issues: groupIssues }) => {
            const isCollapsed = collapsedProjects.has(project.ID);
            return (
              <div key={project.ID} className="space-y-2">
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => toggleCollapsed(project.ID)}
                    className="flex items-center gap-1 text-muted-foreground hover:text-foreground transition-colors"
                  >
                    {isCollapsed ? (
                      <ChevronRight className="h-4 w-4" />
                    ) : (
                      <ChevronDown className="h-4 w-4" />
                    )}
                  </button>
                  <Link
                    to={`/projects/${project.ID}`}
                    className="text-lg font-semibold hover:underline"
                  >
                    {project.Name}
                  </Link>
                  <Badge variant="secondary">{groupIssues.length}</Badge>
                </div>

                {!isCollapsed && (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-10">
                          <span className="sr-only">Select</span>
                        </TableHead>
                        <TableHead>Title</TableHead>
                        <TableHead>Status</TableHead>
                        <TableHead>Priority</TableHead>
                        <TableHead>Type</TableHead>
                        <TableHead>Updated</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {groupIssues.map((issue) => (
                        <TableRow key={issue.ID}>
                          <TableCell className="w-10">
                            <input
                              type="checkbox"
                              checked={selectedIssues.has(issue.ID)}
                              onChange={() => toggleIssueSelection(issue)}
                              className="rounded border-gray-300"
                            />
                          </TableCell>
                          <TableCell>
                            <Link
                              to={`/issues/${issue.ID}`}
                              className="font-medium text-foreground hover:underline"
                            >
                              {issue.Title}
                            </Link>
                          </TableCell>
                          <TableCell>
                            <StatusBadge status={issue.Status} />
                          </TableCell>
                          <TableCell>
                            <PriorityBadge priority={issue.Priority} />
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline">
                              {typeLabels[issue.Type] ?? issue.Type}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <TimeAgo
                              date={issue.UpdatedAt}
                              className="text-muted-foreground text-sm"
                            />
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </div>
            );
          })}
        </div>
      )}

      {selectedIssues.size > 0 && (
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 flex items-center gap-3 bg-background border rounded-lg shadow-lg px-4 py-3">
          <span className="text-sm font-medium">
            {selectedIssues.size} issue{selectedIssues.size > 1 ? "s" : ""} selected
          </span>
          <Select
            onValueChange={(value) => handleBulkStatusChange(value as IssueStatus)}
          >
            <SelectTrigger className="w-[140px] h-8">
              <SelectValue placeholder="Set Status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="open">Open</SelectItem>
              <SelectItem value="in_progress">In Progress</SelectItem>
              <SelectItem value="done">Done</SelectItem>
              <SelectItem value="closed">Closed</SelectItem>
            </SelectContent>
          </Select>
          <Button
            size="sm"
            onClick={() => setLaunchDialogOpen(true)}
            disabled={!canLaunchAgent}
            title={canLaunchAgent ? undefined : "Select issues from a single project to launch an agent"}
          >
            <Rocket className="h-4 w-4 mr-1" />
            Launch Agent
          </Button>
          <Button
            size="sm"
            variant="destructive"
            onClick={() => setDeleteConfirmOpen(true)}
          >
            <Trash2 className="h-4 w-4 mr-1" />
            Delete
          </Button>
          <Button size="sm" variant="ghost" onClick={clearSelection}>
            <X className="h-4 w-4" />
          </Button>
        </div>
      )}

      <IssueForm open={formOpen} onOpenChange={setFormOpen} />

      {canLaunchAgent && (
        <AgentLaunchDialog
          open={launchDialogOpen}
          onOpenChange={setLaunchDialogOpen}
          issues={issues.filter((i) => selectedIssues.has(i.ID))}
          project={projects.find((p) => p.ID === [...selectedProjectIds][0])!}
          onSuccess={clearSelection}
        />
      )}

      <Dialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen}>
        <DialogContent showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>Delete {selectedIssues.size} issue{selectedIssues.size > 1 ? "s" : ""}?</DialogTitle>
            <DialogDescription>
              This action cannot be undone. The selected issue{selectedIssues.size > 1 ? "s" : ""} will be permanently deleted.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteConfirmOpen(false)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleBulkDelete}>
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
