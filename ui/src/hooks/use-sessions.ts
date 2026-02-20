import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type {
  AgentSession,
  SessionDetail,
  SessionStatus,
  SyncSessionRequest,
  SyncSessionResponse,
  MergeSessionRequest,
  MergeSessionResponse,
  DiscoverWorktreesResponse,
  CloseCheckResponse,
  ReactivateResponse,
} from "@/lib/types";

export function useSessions(projectId?: string, status?: SessionStatus[]) {
  const params = new URLSearchParams();
  if (projectId) params.set("project_id", projectId);
  if (status?.length) params.set("status", status.join(","));
  const qs = params.toString();
  return useQuery({
    queryKey: ["sessions", projectId ?? "", status?.join(",") ?? ""],
    queryFn: () => apiFetch<AgentSession[]>(`/api/v1/sessions${qs ? `?${qs}` : ""}`),
    refetchInterval: 30000,
  });
}

export function useSession(id: string) {
  return useQuery({
    queryKey: ["session", id],
    queryFn: () => apiFetch<SessionDetail>(`/api/v1/sessions/${id}`),
    enabled: !!id,
    refetchInterval: 30000,
  });
}

export function useSyncSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ sessionId, ...req }: SyncSessionRequest & { sessionId: string }) =>
      apiFetch<SyncSessionResponse>(`/api/v1/sessions/${sessionId}/sync`, {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
      qc.invalidateQueries({ queryKey: ["session"] });
    },
  });
}

export function useMergeSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ sessionId, ...req }: MergeSessionRequest & { sessionId: string }) =>
      apiFetch<MergeSessionResponse>(`/api/v1/sessions/${sessionId}/merge`, {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
      qc.invalidateQueries({ queryKey: ["session"] });
      qc.invalidateQueries({ queryKey: ["issues"] });
    },
  });
}

export function useDeleteWorktree() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ sessionId, force }: { sessionId: string; force?: boolean }) =>
      apiFetch<void>(`/api/v1/sessions/${sessionId}/worktree${force ? "?force=true" : ""}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
      qc.invalidateQueries({ queryKey: ["session"] });
      qc.invalidateQueries({ queryKey: ["issues"] });
    },
  });
}

export function useDiscoverWorktrees() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (projectId?: string) => {
      const params = projectId ? `?project_id=${projectId}` : "";
      return apiFetch<DiscoverWorktreesResponse>(`/api/v1/sessions/discover${params}`, {
        method: "POST",
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
    },
  });
}

export function useCleanupSessions() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiFetch<{ deleted: number }>("/api/v1/sessions/cleanup", {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
    },
  });
}

export function useCloseCheck(sessionId: string, enabled: boolean) {
  return useQuery({
    queryKey: ["close-check", sessionId],
    queryFn: () => apiFetch<CloseCheckResponse>(`/api/v1/sessions/${sessionId}/close-check`),
    enabled,
    refetchInterval: false,
  });
}

export function useReactivateSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (sessionId: string) =>
      apiFetch<ReactivateResponse>(`/api/v1/sessions/${sessionId}/reactivate`, {
        method: "POST",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
      qc.invalidateQueries({ queryKey: ["session"] });
      qc.invalidateQueries({ queryKey: ["issues"] });
    },
  });
}
