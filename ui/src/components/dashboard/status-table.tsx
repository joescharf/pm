import { useMemo, useState } from "react";
import { Link } from "react-router";
import { ArrowDown, ArrowUp, ArrowUpDown, ExternalLink } from "lucide-react";
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

type SortKey = "name" | "openIssues" | "inProgress" | "health" | "lastActivity";
type SortDir = "asc" | "desc";

interface StatusTableProps {
  entries: StatusEntry[];
}

export function StatusTable({ entries }: StatusTableProps) {
  const [sortKey, setSortKey] = useState<SortKey>("lastActivity");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  const sorted = useMemo(() => {
    const copy = [...entries];
    copy.sort((a, b) => {
      let cmp = 0;
      switch (sortKey) {
        case "name":
          cmp = a.project.Name.localeCompare(b.project.Name);
          break;
        case "openIssues":
          cmp = a.openIssues - b.openIssues;
          break;
        case "inProgress":
          cmp = a.inProgressIssues - b.inProgressIssues;
          break;
        case "health":
          cmp = a.health - b.health;
          break;
        case "lastActivity": {
          const da = a.lastActivity ? new Date(a.lastActivity).getTime() : 0;
          const db = b.lastActivity ? new Date(b.lastActivity).getTime() : 0;
          cmp = da - db;
          break;
        }
      }
      return sortDir === "asc" ? cmp : -cmp;
    });
    return copy;
  }, [entries, sortKey, sortDir]);

  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir(key === "name" ? "asc" : "desc");
    }
  }

  function SortIcon({ column }: { column: SortKey }) {
    if (sortKey !== column) return <ArrowUpDown className="size-3 ml-1 opacity-40" />;
    return sortDir === "asc"
      ? <ArrowUp className="size-3 ml-1" />
      : <ArrowDown className="size-3 ml-1" />;
  }

  if (entries.length === 0) {
    return <EmptyState message="No projects tracked yet" />;
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>
            <button onClick={() => toggleSort("name")} className="inline-flex items-center hover:text-foreground transition-colors">
              Project <SortIcon column="name" />
            </button>
          </TableHead>
          <TableHead>Version</TableHead>
          <TableHead>Branch</TableHead>
          <TableHead>Status</TableHead>
          <TableHead className="text-center">
            <button onClick={() => toggleSort("openIssues")} className="inline-flex items-center hover:text-foreground transition-colors">
              Open <SortIcon column="openIssues" />
            </button>
          </TableHead>
          <TableHead className="text-center">
            <button onClick={() => toggleSort("inProgress")} className="inline-flex items-center hover:text-foreground transition-colors">
              In Progress <SortIcon column="inProgress" />
            </button>
          </TableHead>
          <TableHead className="text-center">
            <button onClick={() => toggleSort("health")} className="inline-flex items-center hover:text-foreground transition-colors">
              Health <SortIcon column="health" />
            </button>
          </TableHead>
          <TableHead>
            <button onClick={() => toggleSort("lastActivity")} className="inline-flex items-center hover:text-foreground transition-colors">
              Last Activity <SortIcon column="lastActivity" />
            </button>
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {sorted.map((e) => (
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
