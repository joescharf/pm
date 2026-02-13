import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";

interface LaunchAgentRequest {
  issue_ids: string[];
  project_id: string;
}

interface LaunchAgentResponse {
  session_id: string;
  branch: string;
  worktree_path: string;
  command: string;
}

export function useLaunchAgent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: LaunchAgentRequest) =>
      apiFetch<LaunchAgentResponse>("/api/v1/agent/launch", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
      qc.invalidateQueries({ queryKey: ["issues"] });
    },
  });
}
