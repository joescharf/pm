import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { AgentSession } from "@/lib/types";

export function useSessions(projectId?: string) {
  const params = projectId ? `?project_id=${projectId}` : "";
  return useQuery({
    queryKey: ["sessions", projectId ?? ""],
    queryFn: () => apiFetch<AgentSession[]>(`/api/v1/sessions${params}`),
  });
}
