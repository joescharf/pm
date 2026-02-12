import { Progress } from "@/components/ui/progress";
import type { HealthScore } from "@/lib/types";

interface HealthChartProps {
  health: HealthScore;
}

const dimensions: { key: keyof Omit<HealthScore, "Total">; label: string; max: number }[] = [
  { key: "GitCleanliness", label: "Git Cleanliness", max: 15 },
  { key: "ActivityRecency", label: "Activity Recency", max: 25 },
  { key: "IssueHealth", label: "Issue Health", max: 20 },
  { key: "ReleaseFreshness", label: "Release Freshness", max: 20 },
  { key: "BranchHygiene", label: "Branch Hygiene", max: 20 },
];

export function HealthChart({ health }: HealthChartProps) {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">Total Score</span>
        <span className="text-2xl font-bold">{health.Total}/100</span>
      </div>
      <div className="space-y-3">
        {dimensions.map(({ key, label, max }) => {
          const score = health[key];
          const pct = max > 0 ? Math.round((score / max) * 100) : 0;
          return (
            <div key={key} className="space-y-1">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">{label}</span>
                <span className="font-mono text-xs">
                  {score}/{max}
                </span>
              </div>
              <Progress value={pct} />
            </div>
          );
        })}
      </div>
    </div>
  );
}
