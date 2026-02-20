// TypeScript interfaces matching Go models (PascalCase JSON — no json tags on Go structs)

export interface Project {
  ID: string;
  Name: string;
  Path: string;
  Description: string;
  RepoURL: string;
  Language: string;
  GroupName: string;
  BranchCount: number;
  HasGitHubPages: boolean;
  PagesURL: string;
  BuildCmd: string;
  ServeCmd: string;
  ServePort: number;
  CreatedAt: string;
  UpdatedAt: string;
}

export type IssueStatus = "open" | "in_progress" | "done" | "closed";
export type IssuePriority = "low" | "medium" | "high";
export type IssueType = "feature" | "bug" | "chore";

export interface Issue {
  ID: string;
  ProjectID: string;
  Title: string;
  Description: string;
  Body: string;
  AIPrompt: string;
  Status: IssueStatus;
  Priority: IssuePriority;
  Type: IssueType;
  Tags: string[] | null;
  GitHubIssue: number;
  CreatedAt: string;
  UpdatedAt: string;
  ClosedAt: string | null;
}

export type SessionStatus = "active" | "idle" | "completed" | "abandoned";
export type ConflictState = "none" | "sync_conflict" | "merge_conflict";

export interface AgentSession {
  ID: string;
  ProjectID: string;
  IssueID: string;
  Branch: string;
  WorktreePath: string;
  Status: SessionStatus;
  Outcome: string;
  CommitCount: number;
  LastCommitHash: string;
  LastCommitMessage: string;
  LastActiveAt: string | null;
  ProjectName?: string;
  StartedAt: string;
  EndedAt: string | null;
  LastError: string;
  LastSyncAt: string | null;
  ConflictState: ConflictState;
  ConflictFiles: string;
  Discovered: boolean;
}

export interface SessionDetail extends AgentSession {
  ProjectName: string;
  WorktreeExists: boolean;
  IsDirty?: boolean;
  CurrentBranch?: string;
  AheadCount?: number;
  BehindCount?: number;
}

export interface SyncSessionRequest {
  rebase?: boolean;
  force?: boolean;
  dry_run?: boolean;
}

export interface SyncSessionResponse {
  SessionID: string;
  Branch: string;
  Success: boolean;
  Ahead: number;
  Behind: number;
  Synced: boolean;
  Conflicts: string[] | null;
  Error: string;
}

export interface MergeSessionRequest {
  base_branch?: string;
  create_pr?: boolean;
  pr_title?: string;
  pr_body?: string;
  pr_draft?: boolean;
  force?: boolean;
  dry_run?: boolean;
}

export interface MergeSessionResponse {
  SessionID: string;
  Branch: string;
  Success: boolean;
  PRCreated: boolean;
  PRURL: string;
  Conflicts: string[] | null;
  Error: string;
}

export interface DiscoverWorktreesResponse {
  discovered: AgentSession[];
  count: number;
}

export interface Tag {
  ID: string;
  Name: string;
  CreatedAt: string;
}

export interface HealthScore {
  Total: number;
  GitCleanliness: number;
  ActivityRecency: number;
  IssueHealth: number;
  ReleaseFreshness: number;
  BranchHygiene: number;
}

export interface ReleaseAsset {
  name: string;
  downloadCount: number;
  size: number;
}

// statusEntry has explicit camelCase json tags in Go
export interface StatusEntry {
  project: Project;
  branch: string;
  isDirty: boolean;
  openIssues: number;
  inProgressIssues: number;
  health: number;
  lastActivity: string;
  latestVersion?: string;
  releaseDate?: string;
  versionSource?: string;
  releaseAssets?: ReleaseAsset[];
}

export type ReviewVerdict = "pass" | "fail";
export type ReviewCategory = "pass" | "fail" | "skip" | "na";

export interface IssueReview {
  ID: string;
  IssueID: string;
  SessionID: string;
  Verdict: ReviewVerdict;
  Summary: string;
  CodeQuality: ReviewCategory;
  RequirementsMatch: ReviewCategory;
  TestCoverage: ReviewCategory;
  UIUX: ReviewCategory;
  FailureReasons: string[] | null;
  DiffStats: string;
  ReviewedAt: string;
  CreatedAt: string;
}

export interface LaunchAgentRequest {
  issue_ids: string[];
  project_id: string;
}

export interface LaunchAgentResponse {
  session_id: string;
  branch: string;
  worktree_path: string;
  command: string;
}

export interface CloseAgentRequest {
  session_id: string;
  status?: "idle" | "completed" | "abandoned";
}

export interface CloseAgentResponse {
  session_id: string;
  status: string;
  ended_at?: string;
}
