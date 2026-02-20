import { useState, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import {
  useCloseCheck,
  useSyncSession,
  useMergeSession,
  useDeleteWorktree,
} from "@/hooks/use-sessions";
import { useCloseAgent } from "@/hooks/use-agent";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import type { AgentSession } from "@/lib/types";

type Step = "check" | "cleanup";

interface CloseWizardDialogProps {
  session: AgentSession;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CloseWizardDialog({ session, open, onOpenChange }: CloseWizardDialogProps) {
  const [step, setStep] = useState<Step>("check");
  const [syncStrategy, setSyncStrategy] = useState<"merge" | "rebase">("merge");
  const [mergeMethod, setMergeMethod] = useState<"pr" | "local">("pr");
  const [showSyncOptions, setShowSyncOptions] = useState(false);
  const [showMergeOptions, setShowMergeOptions] = useState(false);

  const { data: check, isLoading, refetch } = useCloseCheck(session.ID, open);
  const closeAgent = useCloseAgent();
  const sync = useSyncSession();
  const merge = useMergeSession();
  const del = useDeleteWorktree();

  const handleClose = useCallback(
    (isOpen: boolean) => {
      if (!isOpen) {
        setStep("check");
        setShowSyncOptions(false);
        setShowMergeOptions(false);
      }
      onOpenChange(isOpen);
    },
    [onOpenChange],
  );

  const handleSync = () => {
    sync.mutate(
      { sessionId: session.ID, rebase: syncStrategy === "rebase" },
      {
        onSuccess: (data) => {
          setShowSyncOptions(false);
          if (data.Conflicts?.length) {
            toast.warning(`Sync completed with ${data.Conflicts.length} conflict(s)`);
          } else {
            toast.success("Synced with base branch");
          }
          refetch();
        },
        onError: (err) => toast.error(`Sync failed: ${(err as Error).message}`),
      },
    );
  };

  const handleMerge = () => {
    merge.mutate(
      { sessionId: session.ID, create_pr: mergeMethod === "pr" },
      {
        onSuccess: (data) => {
          setShowMergeOptions(false);
          if (data.Conflicts?.length) {
            toast.warning(`Merge has ${data.Conflicts.length} conflict(s)`);
          } else if (data.PRCreated && data.PRURL) {
            toast.success("PR created", {
              description: data.PRURL,
              action: { label: "Open", onClick: () => window.open(data.PRURL, "_blank") },
            });
          } else if (data.Success) {
            toast.success("Merged successfully");
          }
          refetch();
        },
        onError: (err) => toast.error(`Merge failed: ${(err as Error).message}`),
      },
    );
  };

  const handleComplete = () => {
    closeAgent.mutate(
      { session_id: session.ID, status: "completed" },
      {
        onSuccess: () => {
          toast.success("Session completed");
          if (check?.worktree_exists) {
            setStep("cleanup");
          } else {
            handleClose(false);
          }
        },
        onError: (err) => toast.error(`Failed: ${(err as Error).message}`),
      },
    );
  };

  const handleDeleteWorktree = () => {
    del.mutate(
      { sessionId: session.ID, force: true },
      {
        onSuccess: () => {
          toast.success("Worktree deleted");
          handleClose(false);
        },
        onError: (err) => toast.error(`Delete failed: ${(err as Error).message}`),
      },
    );
  };

  const isActionPending = sync.isPending || merge.isPending || closeAgent.isPending || del.isPending;

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-lg">
        {step === "check" && (
          <>
            <DialogHeader>
              <DialogTitle>Complete Session</DialogTitle>
              <DialogDescription>
                Review the state of{" "}
                <code className="text-xs font-mono">{session.Branch}</code>{" "}
                before closing.
              </DialogDescription>
            </DialogHeader>

            {isLoading ? (
              <div className="space-y-3">
                <Skeleton className="h-6 w-full" />
                <Skeleton className="h-6 w-3/4" />
                <Skeleton className="h-6 w-1/2" />
              </div>
            ) : check ? (
              <div className="space-y-4">
                {/* Status indicators */}
                <div className="grid grid-cols-2 gap-3 text-sm">
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">Working tree</span>
                    <Badge
                      variant="outline"
                      className={cn(
                        "text-xs",
                        check.is_dirty
                          ? "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300"
                          : "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300",
                      )}
                    >
                      {check.is_dirty ? "dirty" : "clean"}
                    </Badge>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">Ahead / Behind</span>
                    <span className="text-xs font-mono">
                      <span className={check.ahead_count > 0 ? "text-amber-600 dark:text-amber-400" : "text-green-600 dark:text-green-400"}>
                        +{check.ahead_count}
                      </span>
                      {" / "}
                      <span className={check.behind_count > 0 ? "text-red-600 dark:text-red-400" : "text-green-600 dark:text-green-400"}>
                        -{check.behind_count}
                      </span>
                    </span>
                  </div>
                  {check.conflict_state !== "none" && (
                    <div className="col-span-2 flex items-center justify-between">
                      <span className="text-muted-foreground">Conflicts</span>
                      <Badge
                        variant="outline"
                        className="text-xs bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300"
                      >
                        {check.conflict_state}
                      </Badge>
                    </div>
                  )}
                </div>

                {/* Warnings */}
                {check.warnings.length > 0 && (
                  <div className="rounded-md border border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/30 p-3 space-y-1.5">
                    {check.warnings.map((w, i) => (
                      <p key={i} className="text-xs text-amber-800 dark:text-amber-300">
                        {w.message}
                      </p>
                    ))}
                  </div>
                )}

                {/* Actions */}
                <div className="space-y-3">
                  {check.behind_count > 0 && (
                    <div className="flex items-center gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setShowSyncOptions(!showSyncOptions)}
                        disabled={isActionPending}
                      >
                        Sync with base
                      </Button>
                      {showSyncOptions && (
                        <>
                          <Select value={syncStrategy} onValueChange={(v) => setSyncStrategy(v as "merge" | "rebase")}>
                            <SelectTrigger className="w-[120px] h-8">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="merge">Merge</SelectItem>
                              <SelectItem value="rebase">Rebase</SelectItem>
                            </SelectContent>
                          </Select>
                          <Button size="sm" onClick={handleSync} disabled={isActionPending}>
                            {sync.isPending ? "Syncing..." : "Go"}
                          </Button>
                        </>
                      )}
                    </div>
                  )}
                  {check.ahead_count > 0 && (
                    <div className="flex items-center gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setShowMergeOptions(!showMergeOptions)}
                        disabled={isActionPending}
                      >
                        Merge to base
                      </Button>
                      {showMergeOptions && (
                        <>
                          <Select value={mergeMethod} onValueChange={(v) => setMergeMethod(v as "pr" | "local")}>
                            <SelectTrigger className="w-[160px] h-8">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="pr">Pull Request</SelectItem>
                              <SelectItem value="local">Local merge</SelectItem>
                            </SelectContent>
                          </Select>
                          <Button size="sm" onClick={handleMerge} disabled={isActionPending}>
                            {merge.isPending ? "Merging..." : "Go"}
                          </Button>
                        </>
                      )}
                    </div>
                  )}
                </div>
              </div>
            ) : null}

            <DialogFooter>
              <Button variant="outline" onClick={() => handleClose(false)}>
                Cancel
              </Button>
              <Button
                variant={check?.ready_to_close ? "default" : "outline"}
                onClick={handleComplete}
                disabled={isLoading || isActionPending}
              >
                {closeAgent.isPending
                  ? "Completing..."
                  : check?.ready_to_close
                    ? "Complete"
                    : "Complete Anyway"}
              </Button>
            </DialogFooter>
          </>
        )}

        {step === "cleanup" && (
          <>
            <DialogHeader>
              <DialogTitle>Delete Worktree?</DialogTitle>
              <DialogDescription>
                Session is now completed. Would you like to remove the worktree?
              </DialogDescription>
            </DialogHeader>

            {session.WorktreePath && (
              <p className="text-xs font-mono text-muted-foreground break-all">
                {session.WorktreePath}
              </p>
            )}

            <DialogFooter>
              <Button variant="outline" onClick={() => handleClose(false)}>
                Keep
              </Button>
              <Button
                variant="destructive"
                onClick={handleDeleteWorktree}
                disabled={del.isPending}
              >
                {del.isPending ? "Deleting..." : "Delete Worktree"}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
