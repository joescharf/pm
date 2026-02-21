import { useState } from "react";
import { useParams, useNavigate, Link } from "react-router";
import { Pencil, Trash2, ArrowLeft, Sparkles, Bot } from "lucide-react";
import { useIssue, useDeleteIssue, useEnrichIssue } from "@/hooks/use-issues";
import { useIssueReviews } from "@/hooks/use-reviews";
import { useLaunchReviewAgent } from "@/hooks/use-review-agent";
import { ReviewForm } from "@/components/issues/review-form";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { TimeAgo } from "@/components/shared/time-ago";
import { StatusBadge, PriorityBadge } from "@/components/issues/status-badge";
import { IssueForm } from "@/components/issues/issue-form";
import { toast } from "sonner";
import type { ReviewCategory } from "@/lib/types";

const typeLabels: Record<string, string> = {
  feature: "Feature",
  bug: "Bug",
  chore: "Chore",
};

const categoryLabels: Record<ReviewCategory, { label: string; className: string }> = {
  pass: {
    label: "Pass",
    className: "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300",
  },
  fail: {
    label: "Fail",
    className: "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300",
  },
  skip: {
    label: "Skip",
    className: "bg-gray-100 text-gray-800 dark:bg-gray-900/40 dark:text-gray-300",
  },
  na: {
    label: "N/A",
    className: "bg-gray-100 text-gray-800 dark:bg-gray-900/40 dark:text-gray-300",
  },
};

function CategoryBadge({ category, label }: { category: ReviewCategory; label: string }) {
  const config = categoryLabels[category];
  if (!config) return null;
  return (
    <span className="inline-flex items-center gap-1 text-xs">
      <span className="text-muted-foreground">{label}:</span>
      <Badge variant="outline" className={`border-transparent text-xs py-0 ${config.className}`}>
        {config.label}
      </Badge>
    </span>
  );
}

function ReviewHistory({ issueId }: { issueId: string }) {
  const { data: reviews, isLoading } = useIssueReviews(issueId);

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Reviews</CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-16 rounded-lg" />
        </CardContent>
      </Card>
    );
  }

  const items = reviews ?? [];

  return (
    <Card>
      <CardHeader>
        <CardTitle>Reviews</CardTitle>
      </CardHeader>
      <CardContent>
        {items.length === 0 ? (
          <p className="text-sm text-muted-foreground">No reviews yet</p>
        ) : (
          <div className="space-y-4">
            {items.map((review) => (
              <div key={review.ID} className="border rounded-lg p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Badge
                      variant="outline"
                      className={
                        review.Verdict === "pass"
                          ? "border-transparent bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300"
                          : "border-transparent bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300"
                      }
                    >
                      {review.Verdict === "pass" ? "Pass" : "Fail"}
                    </Badge>
                    {review.DiffStats && (
                      <span className="text-xs text-muted-foreground font-mono">
                        {review.DiffStats}
                      </span>
                    )}
                  </div>
                  <TimeAgo date={review.ReviewedAt} className="text-sm text-muted-foreground" />
                </div>

                <p className="text-sm">{review.Summary}</p>

                <div className="flex flex-wrap gap-3">
                  <CategoryBadge category={review.CodeQuality} label="Code" />
                  <CategoryBadge category={review.RequirementsMatch} label="Requirements" />
                  <CategoryBadge category={review.TestCoverage} label="Tests" />
                  <CategoryBadge category={review.UIUX} label="UI/UX" />
                </div>

                {review.Verdict === "fail" && review.FailureReasons && review.FailureReasons.length > 0 && (
                  <div className="space-y-1">
                    <p className="text-xs font-medium text-muted-foreground">Failure Reasons:</p>
                    <ul className="list-disc list-inside text-sm space-y-0.5">
                      {review.FailureReasons.map((reason, i) => (
                        <li key={i}>{reason}</li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export function IssueDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { data: issue, isLoading, error } = useIssue(id!);
  const deleteIssue = useDeleteIssue();
  const enrichIssue = useEnrichIssue();
  const launchReview = useLaunchReviewAgent();
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
          {issue.Status === "done" && (
            <Button
              variant="default"
              size="sm"
              onClick={() => {
                launchReview.mutate(
                  { issueId: issue.ID },
                  {
                    onSuccess: () => toast.success("AI review agent launched"),
                    onError: (err) => toast.error(`Failed to launch review: ${(err as Error).message}`),
                  }
                );
              }}
              disabled={launchReview.isPending}
            >
              <Bot className="size-4" />
              {launchReview.isPending ? "Launching..." : "Start AI Review"}
            </Button>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              enrichIssue.mutate(issue.ID, {
                onSuccess: () => toast.success("Issue enriched"),
                onError: (err) => toast.error(`Enrichment failed: ${(err as Error).message}`),
              });
            }}
            disabled={enrichIssue.isPending}
          >
            <Sparkles />
            {enrichIssue.isPending ? "Enriching..." : "Enrich"}
          </Button>
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

      {/* AI Prompt */}
      {issue.AIPrompt && (
        <Card>
          <CardHeader>
            <CardTitle>AI Prompt</CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="text-sm whitespace-pre-wrap font-mono text-muted-foreground bg-muted rounded-md p-4">{issue.AIPrompt}</pre>
          </CardContent>
        </Card>
      )}

      {/* Submit Review (shown for non-closed issues) */}
      {issue.Status !== "closed" && (
        <ReviewForm issueId={issue.ID} defaultExpanded={issue.Status === "done"} />
      )}

      {/* Reviews */}
      <ReviewHistory issueId={issue.ID} />

      {/* Edit Dialog */}
      <IssueForm open={editOpen} onOpenChange={setEditOpen} issue={issue} />
    </div>
  );
}
