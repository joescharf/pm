import { useState } from "react";
import { Copy, Check, Rocket } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useLaunchAgent } from "@/hooks/use-agent";
import { toast } from "sonner";
import type { Issue, Project } from "@/lib/types";

interface AgentLaunchDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  issues: Issue[];
  project: Project;
  onSuccess?: () => void;
}

export function AgentLaunchDialog({
  open,
  onOpenChange,
  issues,
  project,
  onSuccess,
}: AgentLaunchDialogProps) {
  const launchAgent = useLaunchAgent();
  const [result, setResult] = useState<{ command: string; branch: string; session_id: string } | null>(null);
  const [copied, setCopied] = useState(false);

  const handleLaunch = () => {
    launchAgent.mutate(
      {
        issue_ids: issues.map((i) => i.ID),
        project_id: project.ID,
      },
      {
        onSuccess: (data) => {
          setResult({ command: data.command, branch: data.branch, session_id: data.session_id });
          toast.success("Agent session created");
          onSuccess?.();
        },
        onError: (err) => {
          toast.error(`Launch failed: ${(err as Error).message}`);
        },
      }
    );
  };

  const handleCopy = async () => {
    if (result?.command) {
      await navigator.clipboard.writeText(result.command);
      setCopied(true);
      toast.success("Command copied to clipboard");
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleClose = (open: boolean) => {
    if (!open) {
      setResult(null);
      setCopied(false);
    }
    onOpenChange(open);
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {result ? "Agent Session Ready" : "Launch Agent"}
          </DialogTitle>
          <DialogDescription>
            {result
              ? "Run the command below to start the Claude Code session."
              : `Create a Claude Code agent session for ${project.Name} with ${issues.length} issue${issues.length > 1 ? "s" : ""}.`}
          </DialogDescription>
        </DialogHeader>

        {result ? (
          <div className="space-y-4">
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground">Branch: <span className="font-mono text-foreground">{result.branch}</span></p>
              <div className="relative">
                <pre className="bg-muted rounded-md p-3 text-sm font-mono overflow-x-auto whitespace-pre-wrap break-all">
                  {result.command}
                </pre>
                <Button
                  size="sm"
                  variant="ghost"
                  className="absolute top-1 right-1"
                  onClick={handleCopy}
                >
                  {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => handleClose(false)}>
                Done
              </Button>
            </DialogFooter>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="space-y-2">
              <p className="text-sm font-medium">Selected Issues:</p>
              <div className="space-y-1">
                {issues.map((issue) => (
                  <div key={issue.ID} className="flex items-center gap-2 text-sm">
                    <Badge variant="outline" className="font-mono text-xs">
                      {issue.ID.slice(0, 12)}
                    </Badge>
                    <span>{issue.Title}</span>
                  </div>
                ))}
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => handleClose(false)}>
                Cancel
              </Button>
              <Button onClick={handleLaunch} disabled={launchAgent.isPending}>
                <Rocket className="h-4 w-4 mr-1" />
                {launchAgent.isPending ? "Launching..." : "Launch Agent"}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
