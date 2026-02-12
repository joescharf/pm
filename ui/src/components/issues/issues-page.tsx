import { useState } from "react";
import { Link } from "react-router";
import { Plus } from "lucide-react";
import { useIssues } from "@/hooks/use-issues";
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
import type { IssueStatus, IssuePriority } from "@/lib/types";

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

export function IssuesPage() {
  const [filters, setFilters] = useState<FilterValues>({});
  const [formOpen, setFormOpen] = useState(false);
  const { data, isLoading, error } = useIssues(filters);
  const issues = data ?? [];

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
            {issues.map((issue) => (
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
                  <Badge variant="outline">{typeLabels[issue.Type] ?? issue.Type}</Badge>
                </TableCell>
                <TableCell>
                  <TimeAgo date={issue.UpdatedAt} className="text-muted-foreground text-sm" />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <IssueForm open={formOpen} onOpenChange={setFormOpen} />
    </div>
  );
}
