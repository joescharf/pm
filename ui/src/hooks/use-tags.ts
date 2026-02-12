import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { Tag } from "@/lib/types";

export function useTags() {
  return useQuery({
    queryKey: ["tags"],
    queryFn: () => apiFetch<Tag[]>("/api/v1/tags"),
  });
}
