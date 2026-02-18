import { useState } from "react";
import { useParams, useNavigate, Link } from "react-router";
import { Pencil, Trash2, ArrowLeft } from "lucide-react";
import { useIssue, useDeleteIssue } from "@/hooks/use-issues";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { TimeAgo } from "@/components/shared/time-ago";
import { StatusBadge, PriorityBadge } from "@/components/issues/status-badge";
import { IssueForm } from "@/components/issues/issue-form";
import { toast } from "sonner";

const typeLabels: Record<string, string> = {
  feature: "Feature",
  bug: "Bug",
  chore: "Chore",
};

export function IssueDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data: issue, isLoading, error } = useIssue(id!);
  const deleteIssue = useDeleteIssue();
  const [editOpen, setEditOpen] = useState(false);

  const handleDelete = () => {
    if (!issue) return;
    if (!window.confirm(`Delete issue "${issue.Title}"?`)) return;

    deleteIssue.mutate(issue.ID, {
      onSuccess: () => {
        toast.success("Issue deleted");
        navigate("/issues");
      },
      onError: (err) => {
        toast.error(`Failed to delete issue: ${(err as Error).message}`);
      },
    });
  };

  if (error) {
    return (
      <div className="text-destructive text-sm">
        Failed to load issue: {(error as Error).message}
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-48 rounded-xl" />
        <Skeleton className="h-32 rounded-xl" />
      </div>
    );
  }

  if (!issue) {
    return <div className="text-muted-foreground text-sm">Issue not found.</div>;
  }

  const tags = issue.Tags ?? [];

  return (
    <div className="space-y-6">
      {/* Back link */}
      <Link
        to="/issues"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="size-4" />
        Back to Issues
      </Link>

      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-2">
          <div className="flex items-center gap-3">
            <h2 className="text-2xl font-bold tracking-tight">{issue.Title}</h2>
            <StatusBadge status={issue.Status} />
            <PriorityBadge priority={issue.Priority} />
          </div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <Button variant="outline" size="sm" onClick={() => setEditOpen(true)}>
            <Pencil />
            Edit
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={handleDelete}
            disabled={deleteIssue.isPending}
          >
            <Trash2 />
            {deleteIssue.isPending ? "Deleting..." : "Delete"}
          </Button>
        </div>
      </div>

      {/* Metadata Card */}
      <Card>
        <CardHeader>
          <CardTitle>Details</CardTitle>
        </CardHeader>
        <CardContent>
          <dl className="grid grid-cols-2 gap-x-6 gap-y-4 text-sm">
            <div>
              <dt className="text-muted-foreground">Type</dt>
              <dd className="mt-1">
                <Badge variant="outline">{typeLabels[issue.Type] ?? issue.Type}</Badge>
              </dd>
            </div>

            <div>
              <dt className="text-muted-foreground">Project</dt>
              <dd className="mt-1">
                <Link
                  to={`/projects/${issue.ProjectID}`}
                  className="text-foreground hover:underline"
                >
                  {issue.ProjectID}
                </Link>
              </dd>
            </div>

            <div>
              <dt className="text-muted-foreground">Tags</dt>
              <dd className="mt-1 flex flex-wrap gap-1">
                {tags.length > 0 ? (
                  tags.map((tag) => (
                    <Badge key={tag} variant="secondary">
                      {tag}
                    </Badge>
                  ))
                ) : (
                  <span className="text-muted-foreground">None</span>
                )}
              </dd>
            </div>

            <div>
              <dt className="text-muted-foreground">GitHub Issue</dt>
              <dd className="mt-1">
                {issue.GitHubIssue > 0 ? `#${issue.GitHubIssue}` : <span className="text-muted-foreground">None</span>}
              </dd>
            </div>

            <div>
              <dt className="text-muted-foreground">Created</dt>
              <dd className="mt-1">
                <TimeAgo date={issue.CreatedAt} />
              </dd>
            </div>

            <div>
              <dt className="text-muted-foreground">Updated</dt>
              <dd className="mt-1">
                <TimeAgo date={issue.UpdatedAt} />
              </dd>
            </div>

            {issue.ClosedAt && (
              <div>
                <dt className="text-muted-foreground">Closed</dt>
                <dd className="mt-1">
                  <TimeAgo date={issue.ClosedAt} />
                </dd>
              </div>
            )}
          </dl>
        </CardContent>
      </Card>

      {/* Description */}
      {issue.Description && (
        <Card>
          <CardHeader>
            <CardTitle>Description</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm whitespace-pre-wrap">{issue.Description}</p>
          </CardContent>
        </Card>
      )}

      {/* Body (raw import text) */}
      {issue.Body && (
        <Card>
          <CardHeader>
            <CardTitle>Raw</CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="text-sm whitespace-pre-wrap font-mono text-muted-foreground bg-muted rounded-md p-4">{issue.Body}</pre>
          </CardContent>
        </Card>
      )}

      {/* Edit Dialog */}
      <IssueForm open={editOpen} onOpenChange={setEditOpen} issue={issue} />
    </div>
  );
}
