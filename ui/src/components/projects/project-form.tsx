import { useEffect, useState } from "react";
import { toast } from "sonner";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { useCreateProject, useUpdateProject } from "@/hooks/use-projects";
import type { Project } from "@/lib/types";

interface ProjectFormProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  project?: Project;
}

export function ProjectForm({ open, onOpenChange, project }: ProjectFormProps) {
  const isEdit = !!project;

  const [name, setName] = useState("");
  const [path, setPath] = useState("");
  const [description, setDescription] = useState("");
  const [language, setLanguage] = useState("");
  const [repoURL, setRepoURL] = useState("");
  const [groupName, setGroupName] = useState("");
  const [buildCmd, setBuildCmd] = useState("");
  const [serveCmd, setServeCmd] = useState("");
  const [servePort, setServePort] = useState<number | "">(0);

  const createProject = useCreateProject();
  const updateProject = useUpdateProject();

  useEffect(() => {
    if (open) {
      setName(project?.Name ?? "");
      setPath(project?.Path ?? "");
      setDescription(project?.Description ?? "");
      setLanguage(project?.Language ?? "");
      setRepoURL(project?.RepoURL ?? "");
      setGroupName(project?.GroupName ?? "");
      setBuildCmd(project?.BuildCmd ?? "");
      setServeCmd(project?.ServeCmd ?? "");
      setServePort(project?.ServePort ?? 0);
    }
  }, [open, project]);

  const isPending = createProject.isPending || updateProject.isPending;

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;

    const data = {
      Name: name.trim(),
      Path: path.trim(),
      Description: description.trim(),
      Language: language.trim(),
      RepoURL: repoURL.trim(),
      GroupName: groupName.trim(),
      BuildCmd: buildCmd.trim(),
      ServeCmd: serveCmd.trim(),
      ServePort: typeof servePort === "number" ? servePort : 0,
    };

    if (isEdit) {
      updateProject.mutate(
        { id: project.ID, ...data },
        {
          onSuccess: () => {
            toast("Project updated");
            onOpenChange(false);
          },
          onError: (err) => {
            toast.error(`Failed to update project: ${(err as Error).message}`);
          },
        },
      );
    } else {
      createProject.mutate(data, {
        onSuccess: () => {
          toast("Project created");
          onOpenChange(false);
        },
        onError: (err) => {
          toast.error(`Failed to create project: ${(err as Error).message}`);
        },
      });
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit Project" : "New Project"}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Name *</Label>
            <Input
              id="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="my-project"
              required
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="path">Path</Label>
            <Input
              id="path"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder="/home/user/projects/my-project"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="description">Description</Label>
            <Textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="A brief description of the project"
              rows={3}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="language">Language</Label>
              <Input
                id="language"
                value={language}
                onChange={(e) => setLanguage(e.target.value)}
                placeholder="Go"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="groupName">Group</Label>
              <Input
                id="groupName"
                value={groupName}
                onChange={(e) => setGroupName(e.target.value)}
                placeholder="backend"
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="repoURL">Repository URL</Label>
            <Input
              id="repoURL"
              value={repoURL}
              onChange={(e) => setRepoURL(e.target.value)}
              placeholder="https://github.com/org/repo"
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="buildCmd">Build Command</Label>
              <Input
                id="buildCmd"
                value={buildCmd}
                onChange={(e) => setBuildCmd(e.target.value)}
                placeholder="e.g. npm run build"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="serveCmd">Serve Command</Label>
              <Input
                id="serveCmd"
                value={serveCmd}
                onChange={(e) => setServeCmd(e.target.value)}
                placeholder="e.g. npm run dev"
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="servePort">Serve Port</Label>
            <Input
              id="servePort"
              type="number"
              value={servePort === 0 ? "" : servePort}
              onChange={(e) => setServePort(e.target.value ? parseInt(e.target.value, 10) : 0)}
              placeholder="e.g. 3000"
            />
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isPending}>
              {isPending
                ? isEdit
                  ? "Saving..."
                  : "Creating..."
                : isEdit
                  ? "Save Changes"
                  : "Create Project"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
