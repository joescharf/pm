// TypeScript interfaces matching Go models (PascalCase JSON — no json tags on Go structs)

export interface Project {
  ID: string;
  Name: string;
  Path: string;
  Description: string;
  RepoURL: string;
  Language: string;
  GroupName: string;
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

export interface AgentSession {
  ID: string;
  ProjectID: string;
  IssueID: string;
  Branch: string;
  WorktreePath: string;
  Status: SessionStatus;
  Outcome: string;
  CommitCount: number;
  StartedAt: string;
  EndedAt: string | null;
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
