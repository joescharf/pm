import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTags } from "@/hooks/use-tags";
import type { IssueStatus, IssuePriority } from "@/lib/types";

interface IssueFilterValues {
  status?: IssueStatus;
  priority?: IssuePriority;
  tag?: string;
}

interface IssueFiltersProps {
  filters: IssueFilterValues;
  onChange: (filters: IssueFilterValues) => void;
}

export function IssueFilters({ filters, onChange }: IssueFiltersProps) {
  const { data: tagsData } = useTags();
  const tags = tagsData ?? [];

  return (
    <div className="flex items-center gap-3">
      <Select
        value={filters.status ?? "__all__"}
        onValueChange={(value) =>
          onChange({
            ...filters,
            status: value === "__all__" ? undefined : (value as IssueStatus),
          })
        }
      >
        <SelectTrigger className="w-[150px]">
          <SelectValue placeholder="Status" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__all__">All Statuses</SelectItem>
          <SelectItem value="open">Open</SelectItem>
          <SelectItem value="in_progress">In Progress</SelectItem>
          <SelectItem value="done">Done</SelectItem>
          <SelectItem value="closed">Closed</SelectItem>
        </SelectContent>
      </Select>

      <Select
        value={filters.priority ?? "__all__"}
        onValueChange={(value) =>
          onChange({
            ...filters,
            priority: value === "__all__" ? undefined : (value as IssuePriority),
          })
        }
      >
        <SelectTrigger className="w-[150px]">
          <SelectValue placeholder="Priority" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__all__">All Priorities</SelectItem>
          <SelectItem value="low">Low</SelectItem>
          <SelectItem value="medium">Medium</SelectItem>
          <SelectItem value="high">High</SelectItem>
        </SelectContent>
      </Select>

      <Select
        value={filters.tag ?? "__all__"}
        onValueChange={(value) =>
          onChange({
            ...filters,
            tag: value === "__all__" ? undefined : value,
          })
        }
      >
        <SelectTrigger className="w-[150px]">
          <SelectValue placeholder="Tag" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__all__">All Tags</SelectItem>
          {tags.map((tag) => (
            <SelectItem key={tag.ID} value={tag.Name}>
              {tag.Name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
