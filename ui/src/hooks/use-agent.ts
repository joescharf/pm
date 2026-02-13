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

interface CloseAgentRequest {
  session_id: string;
  status?: "idle" | "completed" | "abandoned";
}

interface CloseAgentResponse {
  session_id: string;
  status: string;
  ended_at?: string;
}

export function useCloseAgent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: CloseAgentRequest) =>
      apiFetch<CloseAgentResponse>("/api/v1/agent/close", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
      qc.invalidateQueries({ queryKey: ["issues"] });
    },
  });
}
