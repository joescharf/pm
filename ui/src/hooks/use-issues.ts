import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { Issue, IssueStatus, IssuePriority } from "@/lib/types";

interface IssueFilters {
  status?: IssueStatus;
  priority?: IssuePriority;
  tag?: string;
}

export function useIssues(filters?: IssueFilters) {
  const params = new URLSearchParams();
  if (filters?.status) params.set("status", filters.status);
  if (filters?.priority) params.set("priority", filters.priority);
  if (filters?.tag) params.set("tag", filters.tag);
  const qs = params.toString();
  return useQuery({
    queryKey: ["issues", filters ?? {}],
    queryFn: () => apiFetch<Issue[]>(`/api/v1/issues${qs ? `?${qs}` : ""}`),
  });
}

export function useProjectIssues(projectId: string) {
  return useQuery({
    queryKey: ["issues", "project", projectId],
    queryFn: () => apiFetch<Issue[]>(`/api/v1/projects/${projectId}/issues`),
    enabled: !!projectId,
  });
}

export function useIssue(id: string) {
  return useQuery({
    queryKey: ["issues", id],
    queryFn: () => apiFetch<Issue>(`/api/v1/issues/${id}`),
    enabled: !!id,
  });
}

export function useCreateIssue() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      projectId,
      ...data
    }: Partial<Issue> & { projectId: string }) =>
      apiFetch<Issue>(`/api/v1/projects/${projectId}/issues`, {
        method: "POST",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["issues"] });
      qc.invalidateQueries({ queryKey: ["status"] });
    },
  });
}

export function useUpdateIssue() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...data }: Partial<Issue> & { id: string }) =>
      apiFetch<Issue>(`/api/v1/issues/${id}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["issues"] });
      qc.invalidateQueries({ queryKey: ["status"] });
    },
  });
}

export function useDeleteIssue() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiFetch<void>(`/api/v1/issues/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["issues"] });
      qc.invalidateQueries({ queryKey: ["status"] });
    },
  });
}

export function useBulkUpdateIssueStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ ids, status }: { ids: string[]; status: string }) =>
      apiFetch<{ updated: number }>(`/api/v1/issues/bulk-update`, {
        method: "POST",
        body: JSON.stringify({ ids, status }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["issues"] });
      qc.invalidateQueries({ queryKey: ["status"] });
    },
  });
}

export function useBulkDeleteIssues() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (ids: string[]) =>
      apiFetch<{ deleted: number }>(`/api/v1/issues/bulk-delete`, {
        method: "POST",
        body: JSON.stringify({ ids }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["issues"] });
      qc.invalidateQueries({ queryKey: ["status"] });
    },
  });
}
