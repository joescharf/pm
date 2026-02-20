import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useSyncSession,
  useMergeSession,
  useDeleteWorktree,
} from "@/hooks/use-sessions";
import { toast } from "sonner";
import type { SessionDetail, AgentSession } from "@/lib/types";

type Session = SessionDetail | AgentSession;

function isLive(s: Session) {
  return s.Status === "active" || s.Status === "idle";
}

// --- Sync Dialog ---
export function SyncButton({ session }: { session: Session }) {
  const [open, setOpen] = useState(false);
  const [strategy, setStrategy] = useState<"merge" | "rebase">("merge");
  const sync = useSyncSession();

  const handleSync = () => {
    sync.mutate(
      { sessionId: session.ID, rebase: strategy === "rebase" },
      {
        onSuccess: (data) => {
          setOpen(false);
          if (data.Conflicts?.length) {
            toast.warning(`Sync completed with ${data.Conflicts.length} conflict(s)`);
          } else if (data.Synced) {
            toast.success("Synced with base branch");
          } else {
            toast.info("Already up to date");
          }
        },
        onError: (err) => toast.error(`Sync failed: ${(err as Error).message}`),
      },
    );
  };

  if (!isLive(session)) return null;

  return (
    <>
      <Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => setOpen(true)}>
        Sync
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Sync with base branch</DialogTitle>
            <DialogDescription>
              Pull latest changes from the base branch into{" "}
              <code className="text-xs font-mono">{session.Branch}</code>.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div>
              <Label>Strategy</Label>
              <Select value={strategy} onValueChange={(v) => setStrategy(v as "merge" | "rebase")}>
                <SelectTrigger className="mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="merge">Merge</SelectItem>
                  <SelectItem value="rebase">Rebase</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSync} disabled={sync.isPending}>
              {sync.isPending ? "Syncing..." : "Sync"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

// --- Merge Dialog ---
export function MergeButton({ session }: { session: Session }) {
  const [open, setOpen] = useState(false);
  const [method, setMethod] = useState<"pr" | "merge" | "rebase">("pr");
  const [force, setForce] = useState(false);
  const merge = useMergeSession();

  const methodLabels = { pr: "Pull Request", merge: "Merge", rebase: "Rebase" };
  const buttonLabels = { pr: "Create PR", merge: "Merge", rebase: "Rebase & Merge" };

  const handleMerge = () => {
    merge.mutate(
      {
        sessionId: session.ID,
        rebase: method === "rebase",
        create_pr: method === "pr",
        force,
      },
      {
        onSuccess: (data) => {
          setOpen(false);
          if (data.Conflicts?.length) {
            toast.warning(`Merge has ${data.Conflicts.length} conflict(s)`);
          } else if (data.PRCreated && data.PRURL) {
            toast.success("PR created", {
              description: data.PRURL,
              action: {
                label: "Open",
                onClick: () => window.open(data.PRURL, "_blank"),
              },
            });
          } else if (data.Success) {
            toast.success(`${methodLabels[method]} completed successfully`);
          }
        },
        onError: (err) => toast.error(`${methodLabels[method]} failed: ${(err as Error).message}`),
      },
    );
  };

  if (!isLive(session)) return null;

  return (
    <>
      <Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => setOpen(true)}>
        Merge
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Merge into base branch</DialogTitle>
            <DialogDescription>
              Integrate <code className="text-xs font-mono">{session.Branch}</code> into the base
              branch.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div>
              <Label>Method</Label>
              <Select value={method} onValueChange={(v) => setMethod(v as "pr" | "merge" | "rebase")}>
                <SelectTrigger className="mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="pr">Pull Request</SelectItem>
                  <SelectItem value="merge">Merge</SelectItem>
                  <SelectItem value="rebase">Rebase</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={force}
                onChange={(e) => setForce(e.target.checked)}
                className="rounded border-input"
              />
              Force (skip dirty worktree check)
            </label>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleMerge} disabled={merge.isPending}>
              {merge.isPending ? "Working..." : buttonLabels[method]}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

// --- Delete Worktree Dialog ---
export function DeleteWorktreeButton({ session }: { session: Session }) {
  const [open, setOpen] = useState(false);
  const del = useDeleteWorktree();

  const handleDelete = () => {
    del.mutate(
      { sessionId: session.ID },
      {
        onSuccess: () => {
          setOpen(false);
          toast.success("Worktree deleted");
        },
        onError: (err) => toast.error(`Delete failed: ${(err as Error).message}`),
      },
    );
  };

  if (!isLive(session)) return null;

  return (
    <>
      <Button
        variant="outline"
        size="sm"
        className="h-7 text-xs text-destructive"
        onClick={() => setOpen(true)}
      >
        Delete Worktree
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete worktree</DialogTitle>
            <DialogDescription>
              This will remove the worktree directory for{" "}
              <code className="text-xs font-mono">{session.Branch}</code> and mark the session as
              abandoned. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          {session.WorktreePath && (
            <p className="text-xs font-mono text-muted-foreground break-all">
              {session.WorktreePath}
            </p>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDelete} disabled={del.isPending}>
              {del.isPending ? "Deleting..." : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
