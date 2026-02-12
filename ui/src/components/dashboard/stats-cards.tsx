import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { StatusEntry } from "@/lib/types";
import { FolderGit2, CircleDot, AlertCircle, Activity } from "lucide-react";

interface StatsCardsProps {
  entries: StatusEntry[];
}

export function StatsCards({ entries }: StatsCardsProps) {
  const totalProjects = entries.length;
  const totalOpen = entries.reduce((n, e) => n + e.openIssues, 0);
  const totalInProgress = entries.reduce((n, e) => n + e.inProgressIssues, 0);
  const avgHealth =
    totalProjects > 0
      ? Math.round(entries.reduce((n, e) => n + e.health, 0) / totalProjects)
      : 0;

  const stats = [
    { label: "Projects", value: totalProjects, icon: FolderGit2 },
    { label: "Open Issues", value: totalOpen, icon: CircleDot },
    { label: "In Progress", value: totalInProgress, icon: AlertCircle },
    { label: "Avg Health", value: avgHealth, icon: Activity },
  ];

  return (
    <div className="grid gap-4 grid-cols-2 lg:grid-cols-4">
      {stats.map((s) => (
        <Card key={s.label}>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">{s.label}</CardTitle>
            <s.icon className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{s.value}</div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
