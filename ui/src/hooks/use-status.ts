import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { StatusEntry, HealthScore } from "@/lib/types";

export function useStatusOverview() {
  return useQuery({
    queryKey: ["status"],
    queryFn: () => apiFetch<StatusEntry[]>("/api/v1/status"),
  });
}

export function useProjectStatus(id: string) {
  return useQuery({
    queryKey: ["status", id],
    queryFn: () => apiFetch<StatusEntry>(`/api/v1/status/${id}`),
    enabled: !!id,
  });
}

export function useProjectHealth(id: string) {
  return useQuery({
    queryKey: ["health", id],
    queryFn: () => apiFetch<HealthScore>(`/api/v1/health/${id}`),
    enabled: !!id,
  });
}
