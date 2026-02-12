import { useMemo, useState } from "react";
import { Link } from "react-router";
import { MoreHorizontal, Plus } from "lucide-react";
import { toast } from "sonner";
import { useProjects, useDeleteProject } from "@/hooks/use-projects";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/shared/empty-state";
import { TimeAgo } from "@/components/shared/time-ago";
import { ProjectForm } from "./project-form";

export function ProjectsPage() {
  const [groupFilter, setGroupFilter] = useState<string>("");
  const [formOpen, setFormOpen] = useState(false);

  const { data, isLoading, error } = useProjects(groupFilter || undefined);
  const projects = data ?? [];
  const deleteProject = useDeleteProject();

  // Fetch all projects (unfiltered) to extract unique groups
  const { data: allProjects } = useProjects();
  const groups = useMemo(() => {
    const all = allProjects ?? [];
    const unique = [...new Set(all.map((p) => p.GroupName).filter(Boolean))];
    return unique.sort();
  }, [allProjects]);

  function handleDelete(id: string, name: string) {
    if (!window.confirm(`Delete project "${name}"? This cannot be undone.`)) {
      return;
    }
    deleteProject.mutate(id, {
      onSuccess: () => toast("Project deleted"),
      onError: (err) =>
        toast.error(`Failed to delete project: ${(err as Error).message}`),
    });
  }

  if (error) {
    return (
      <div className="text-destructive text-sm">
        Failed to load projects: {(error as Error).message}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold tracking-tight">Projects</h2>
        <div className="flex items-center gap-3">
          {groups.length > 0 && (
            <Select
              value={groupFilter}
              onValueChange={(val) =>
                setGroupFilter(val === "__all__" ? "" : val)
              }
            >
              <SelectTrigger className="w-[160px]">
                <SelectValue placeholder="All groups" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">All groups</SelectItem>
                {groups.map((g) => (
                  <SelectItem key={g} value={g}>
                    {g}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
          <Button onClick={() => setFormOpen(true)}>
            <Plus />
            Add Project
          </Button>
        </div>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 rounded-lg" />
          ))}
        </div>
      ) : projects.length === 0 ? (
        <EmptyState message="No projects found" />
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Path</TableHead>
              <TableHead>Language</TableHead>
              <TableHead>Group</TableHead>
              <TableHead>Updated</TableHead>
              <TableHead className="w-[50px]" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {projects.map((p) => (
              <TableRow key={p.ID}>
                <TableCell>
                  <Link
                    to={`/projects/${p.ID}`}
                    className="font-medium hover:underline"
                  >
                    {p.Name}
                  </Link>
                </TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground max-w-[200px] truncate">
                  {p.Path || "—"}
                </TableCell>
                <TableCell>{p.Language || "—"}</TableCell>
                <TableCell>{p.GroupName || "—"}</TableCell>
                <TableCell>
                  <TimeAgo
                    date={p.UpdatedAt}
                    className="text-xs text-muted-foreground"
                  />
                </TableCell>
                <TableCell>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon-sm">
                        <MoreHorizontal className="size-4" />
                        <span className="sr-only">Actions</span>
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem asChild>
                        <Link to={`/projects/${p.ID}`}>View Details</Link>
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        variant="destructive"
                        onClick={() => handleDelete(p.ID, p.Name)}
                      >
                        Delete
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <ProjectForm open={formOpen} onOpenChange={setFormOpen} />
    </div>
  );
}
