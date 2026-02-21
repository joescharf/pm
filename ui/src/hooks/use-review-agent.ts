import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { LaunchReviewAgentResponse } from "@/lib/types";

export function useLaunchReviewAgent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ issueId }: { issueId: string }) =>
      apiFetch<LaunchReviewAgentResponse>(`/api/v1/issues/${issueId}/review-agent`, {
        method: "POST",
        body: JSON.stringify({}),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
      qc.invalidateQueries({ queryKey: ["issues"] });
      qc.invalidateQueries({ queryKey: ["issue"] });
    },
  });
}
