import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { IssueStatus, IssuePriority } from "@/lib/types";

const statusConfig: Record<IssueStatus, { label: string; className: string }> = {
  open: {
    label: "Open",
    className: "bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300",
  },
  in_progress: {
    label: "In Progress",
    className: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300",
  },
  done: {
    label: "Done",
    className: "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300",
  },
  closed: {
    label: "Closed",
    className: "bg-gray-100 text-gray-800 dark:bg-gray-900/40 dark:text-gray-300",
  },
};

const priorityConfig: Record<IssuePriority, { label: string; variant: "secondary" | "default" | "destructive"; className?: string }> = {
  low: {
    label: "Low",
    variant: "secondary",
  },
  medium: {
    label: "Medium",
    variant: "default",
    className: "bg-blue-600 text-white dark:bg-blue-700",
  },
  high: {
    label: "High",
    variant: "destructive",
  },
};

interface StatusBadgeProps {
  status: IssueStatus;
  className?: string;
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const config = statusConfig[status];
  if (!config) {
    return <Badge variant="outline" className={className}>{status || "—"}</Badge>;
  }
  return (
    <Badge variant="outline" className={cn("border-transparent", config.className, className)}>
      {config.label}
    </Badge>
  );
}

interface PriorityBadgeProps {
  priority: IssuePriority;
  className?: string;
}

export function PriorityBadge({ priority, className }: PriorityBadgeProps) {
  const config = priorityConfig[priority];
  if (!config) {
    return <Badge variant="secondary" className={className}>{priority || "—"}</Badge>;
  }
  return (
    <Badge variant={config.variant} className={cn(config.className, className)}>
      {config.label}
    </Badge>
  );
}
