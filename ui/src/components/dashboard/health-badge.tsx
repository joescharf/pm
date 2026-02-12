import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

function healthColor(score: number): string {
  if (score >= 80) return "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300";
  if (score >= 60) return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300";
  if (score >= 40) return "bg-orange-100 text-orange-800 dark:bg-orange-900/40 dark:text-orange-300";
  return "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300";
}

interface HealthBadgeProps {
  score: number;
  className?: string;
}

export function HealthBadge({ score, className }: HealthBadgeProps) {
  return (
    <Badge variant="outline" className={cn(healthColor(score), "font-mono", className)}>
      {score}
    </Badge>
  );
}
