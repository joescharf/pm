import { useState, useEffect } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useProjects } from "@/hooks/use-projects";
import { useCreateIssue, useUpdateIssue } from "@/hooks/use-issues";
import { toast } from "sonner";
import type { Issue, IssueStatus, IssuePriority, IssueType } from "@/lib/types";

interface IssueFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  issue?: Issue;
  defaultProjectId?: string;
}

export function IssueForm({ open, onOpenChange, issue, defaultProjectId }: IssueFormProps) {
  const isEdit = !!issue;
  const { data: projectsData } = useProjects();
  const projects = projectsData ?? [];
  const createIssue = useCreateIssue();
  const updateIssue = useUpdateIssue();

  const [projectId, setProjectId] = useState(defaultProjectId ?? "");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [body, setBody] = useState("");
  const [aiPrompt, setAiPrompt] = useState("");
  const [type, setType] = useState<IssueType>("feature");
  const [priority, setPriority] = useState<IssuePriority>("medium");
  const [status, setStatus] = useState<IssueStatus>("open");

  useEffect(() => {
    if (open) {
      if (issue) {
        setProjectId(issue.ProjectID);
        setTitle(issue.Title);
        setDescription(issue.Description);
        setBody(issue.Body ?? "");
        setAiPrompt(issue.AIPrompt ?? "");
        setType(issue.Type);
        setPriority(issue.Priority);
        setStatus(issue.Status);
      } else {
        setProjectId(defaultProjectId ?? "");
        setTitle("");
        setDescription("");
        setBody("");
        setAiPrompt("");
        setType("feature");
        setPriority("medium");
        setStatus("open");
      }
    }
  }, [open, issue, defaultProjectId]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    if (!projectId) {
      toast.error("Please select a project");
      return;
    }
    if (!title.trim()) {
      toast.error("Title is required");
      return;
    }

    if (isEdit) {
      updateIssue.mutate(
        {
          id: issue.ID,
          Title: title.trim(),
          Description: description,
          Body: body,
          AIPrompt: aiPrompt,
          Type: type,
          Priority: priority,
          Status: status,
        },
        {
          onSuccess: () => {
            toast.success("Issue updated");
            onOpenChange(false);
          },
          onError: (err) => {
            toast.error(`Failed to update issue: ${(err as Error).message}`);
          },
        },
      );
    } else {
      createIssue.mutate(
        {
          projectId,
          Title: title.trim(),
          Description: description,
          Body: body,
          AIPrompt: aiPrompt,
          Type: type,
          Priority: priority,
          Status: status,
        },
        {
          onSuccess: () => {
            toast.success("Issue created");
            onOpenChange(false);
          },
          onError: (err) => {
            toast.error(`Failed to create issue: ${(err as Error).message}`);
          },
        },
      );
    }
  };

  const isPending = createIssue.isPending || updateIssue.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit Issue" : "Create Issue"}</DialogTitle>
          <DialogDescription>
            {isEdit ? "Update the issue details below." : "Fill in the details to create a new issue."}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="project">Project</Label>
            <Select value={projectId} onValueChange={setProjectId} disabled={isEdit}>
              <SelectTrigger id="project" className="w-full">
                <SelectValue placeholder="Select a project" />
              </SelectTrigger>
              <SelectContent>
                {projects.map((p) => (
                  <SelectItem key={p.ID} value={p.ID}>
                    {p.Name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="title">Title</Label>
            <Input
              id="title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Issue title"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="description">Description</Label>
            <Textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Describe the issue..."
              rows={3}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="body">Raw Body</Label>
            <Textarea
              id="body"
              value={body}
              onChange={(e) => setBody(e.target.value)}
              placeholder="Original issue text or context..."
              rows={3}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="aiPrompt">AI Prompt</Label>
            <Textarea
              id="aiPrompt"
              value={aiPrompt}
              onChange={(e) => setAiPrompt(e.target.value)}
              placeholder="Guidance for AI agents working on this issue..."
              rows={3}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="type">Type</Label>
              <Select value={type} onValueChange={(v) => setType(v as IssueType)}>
                <SelectTrigger id="type" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="feature">Feature</SelectItem>
                  <SelectItem value="bug">Bug</SelectItem>
                  <SelectItem value="chore">Chore</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="priority">Priority</Label>
              <Select value={priority} onValueChange={(v) => setPriority(v as IssuePriority)}>
                <SelectTrigger id="priority" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="low">Low</SelectItem>
                  <SelectItem value="medium">Medium</SelectItem>
                  <SelectItem value="high">High</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          {isEdit && (
            <div className="space-y-2">
              <Label htmlFor="status">Status</Label>
              <Select value={status} onValueChange={(v) => setStatus(v as IssueStatus)}>
                <SelectTrigger id="status" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="open">Open</SelectItem>
                  <SelectItem value="in_progress">In Progress</SelectItem>
                  <SelectItem value="done">Done</SelectItem>
                  <SelectItem value="closed">Closed</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={isPending}>
              {isPending ? (isEdit ? "Updating..." : "Creating...") : isEdit ? "Update Issue" : "Create Issue"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
