import { Link } from "react-router";
import { ExternalLink } from "lucide-react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { HealthBadge } from "./health-badge";
import { TimeAgo } from "@/components/shared/time-ago";
import { EmptyState } from "@/components/shared/empty-state";
import type { StatusEntry } from "@/lib/types";

function repoURL(url: string): string {
  return url.replace(/\.git$/, "");
}

interface StatusTableProps {
  entries: StatusEntry[];
}

export function StatusTable({ entries }: StatusTableProps) {
  if (entries.length === 0) {
    return <EmptyState message="No projects tracked yet" />;
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Project</TableHead>
          <TableHead>Version</TableHead>
          <TableHead>Branch</TableHead>
          <TableHead>Status</TableHead>
          <TableHead className="text-center">Open</TableHead>
          <TableHead className="text-center">In Progress</TableHead>
          <TableHead className="text-center">Health</TableHead>
          <TableHead>Last Activity</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {entries.map((e) => (
          <TableRow key={e.project.ID}>
            <TableCell>
              <Link
                to={`/projects/${e.project.ID}`}
                className="font-medium hover:underline"
              >
                {e.project.Name}
              </Link>
              {e.project.GroupName && (
                <span className="ml-2 text-xs text-muted-foreground">
                  {e.project.GroupName}
                </span>
              )}
            </TableCell>
            <TableCell className="font-mono text-xs">
              {e.latestVersion ? (
                e.project.RepoURL && e.versionSource === "github" ? (
                  <a
                    href={`${repoURL(e.project.RepoURL)}/releases/tag/${e.latestVersion}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 hover:underline"
                    title="GitHub release"
                  >
                    {e.latestVersion}
                    <ExternalLink className="size-3" />
                  </a>
                ) : (
                  <span title="git tag">{e.latestVersion}</span>
                )
              ) : (
                "—"
              )}
            </TableCell>
            <TableCell className="font-mono text-xs">
              {e.branch ? (
                e.project.RepoURL ? (
                  <a
                    href={`${repoURL(e.project.RepoURL)}/tree/${e.branch}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 hover:underline"
                  >
                    {e.branch}
                    <ExternalLink className="size-3" />
                  </a>
                ) : (
                  e.branch
                )
              ) : (
                "—"
              )}
            </TableCell>
            <TableCell>
              {e.isDirty ? (
                <Badge variant="destructive" className="text-xs">dirty</Badge>
              ) : (
                <Badge variant="secondary" className="text-xs">clean</Badge>
              )}
            </TableCell>
            <TableCell className="text-center">{e.openIssues}</TableCell>
            <TableCell className="text-center">{e.inProgressIssues}</TableCell>
            <TableCell className="text-center">
              <HealthBadge score={e.health} />
            </TableCell>
            <TableCell>
              <TimeAgo date={e.lastActivity} className="text-xs text-muted-foreground" />
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
