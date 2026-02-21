import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { IssueReview, ReviewVerdict, ReviewCategory } from "@/lib/types";

export function useIssueReviews(issueId: string) {
  return useQuery({
    queryKey: ["issue-reviews", issueId],
    queryFn: () => apiFetch<IssueReview[]>(`/api/v1/issues/${issueId}/reviews`),
    enabled: !!issueId,
  });
}

export interface CreateReviewInput {
  verdict: ReviewVerdict;
  summary: string;
  code_quality?: ReviewCategory;
  requirements_match?: ReviewCategory;
  test_coverage?: ReviewCategory;
  ui_ux?: ReviewCategory;
  failure_reasons?: string[];
  diff_stats?: string;
}

export function useCreateReview(issueId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateReviewInput) =>
      apiFetch<IssueReview>(`/api/v1/issues/${issueId}/reviews`, {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["issue-reviews", issueId] });
      qc.invalidateQueries({ queryKey: ["issue", issueId] });
    },
  });
}
