import { Inbox } from "lucide-react";

interface EmptyStateProps {
  message?: string;
  icon?: React.ReactNode;
}

export function EmptyState({
  message = "No data found",
  icon,
}: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
      {icon ?? <Inbox className="h-10 w-10 mb-3" />}
      <p className="text-sm">{message}</p>
    </div>
  );
}
