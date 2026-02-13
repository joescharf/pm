import { useMemo, useState } from "react";
import { Link } from "react-router";
import { ChevronDown, ChevronRight, Plus } from "lucide-react";
import { useIssues } from "@/hooks/use-issues";
import { useProjects } from "@/hooks/use-projects";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
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
  const [formOpen, setFormOpen] = useState(false);
  const [collapsedProjects, setCollapsedProjects] = useState<Set<string>>(
    new Set()
  );
  const { data: issuesData, isLoading: issuesLoading, error } = useIssues(filters);
  const { data: projectsData, isLoading: projectsLoading } = useProjects();
  const issues = issuesData ?? [];
  const projects = projectsData ?? [];
  const isLoading = issuesLoading || projectsLoading;

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

      <IssueFilters filters={filters} onChange={setFilters} />

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

      <IssueForm open={formOpen} onOpenChange={setFormOpen} />
    </div>
  );
}
