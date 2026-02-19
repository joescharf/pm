import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { IssueReview } from "@/lib/types";

export function useIssueReviews(issueId: string) {
  return useQuery({
    queryKey: ["issue-reviews", issueId],
    queryFn: () => apiFetch<IssueReview[]>(`/api/v1/issues/${issueId}/reviews`),
    enabled: !!issueId,
  });
}
