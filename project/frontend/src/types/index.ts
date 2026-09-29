export type RepositoryPlatform = "github" | "gitlab";

export type AnalysisStatus =
  | "queued"
  | "running"
  | "completed"
  | "failed";

export type DataStatus =
  | "available"
  | "unavailable"
  | "not_applicable";

export type Severity =
  | "critical"
  | "high"
  | "medium"
  | "low"
  | "info";

export type Priority =
  | "critical"
  | "high"
  | "medium"
  | "low"
  | "info";

export type RecommendationEffort =
  | "small"
  | "medium"
  | "large";

export type ScoreCompleteness =
  | "complete"
  | "partial";

export type EvidenceType =
  | "metric"
  | "file"
  | "repository"
  | "api"
  | "dependency"
  | "advisory";

export type FactorType =
  | "positive"
  | "negative";

export interface Report {
  analysisId: string;
  reportId: string;
  status: AnalysisStatus;
  error: AnalysisError | null;
  repository: Repository;
  analysis: AnalysisMetadata;
  score: HealthScore;
  categories: Category[];
  issues: Issue[];
  recommendations: Recommendation[];
  limitations: Limitation[];
}

export interface Repository {
  platform: RepositoryPlatform;
  url: string;
  owner: string;
  name: string;
  description: string | null;
  defaultBranch: string;
  commitSha: string | null;
  visibility: "public";
}

export interface AnalysisMetadata {
  startedAt: string;
  completedAt: string | null;
  durationSeconds: number | null;
  schemaVersion: string;
  methodology: Methodology;
}

export interface Methodology {
  id: string;
  version: string;
  name: string;
}

export interface HealthScore {
  value: number | null;
  label: string;
  summary: string;
  keyFactors: ScoreFactor[];
  dataCompleteness: DataCompleteness;
}

export interface ScoreFactor {
  type: FactorType;
  categoryId: string;
  text: string;
}

export interface DataCompleteness {
  status: ScoreCompleteness;
  percentage: number;
  summary: string;
  limitationsCount: number;
}

export interface Category {
  id: string;
  name: string;
  score: number | null;
  weight: number;
  status: DataStatus;
  summary: string;
  positiveObservations: Observation[];
  negativeObservations: Observation[];
  unavailableReason?: string;
  evidence: Evidence[];
}

export interface Observation {
  id: string;
  text: string;
}

export interface Evidence {
  type: EvidenceType;
  name: string;
  value?: string | number | null;
  unit?: string;
  source?: string;
  path?: string;
  expected?: string | number;
  threshold?: number;
}

export interface Issue {
  id: string;
  categoryId: string;
  title: string;
  description: string;
  severity: Severity;
  evidence: Evidence[];
  relatedPath?: string;
  relatedUrl?: string;
}

export interface Recommendation {
  id: string;
  priority: Priority;
  action: string;
  expectedEffect: string;
  relatedIssueIds: string[];
  effort: RecommendationEffort;
}

export interface Limitation {
  id: string;
  type: "unavailable_data";
  categoryId: string;
  title: string;
  description: string;
  source: string;
  status: "unavailable";
}

export interface AnalysisError {
  code:
    | "INVALID_REPOSITORY_URL"
    | "REPOSITORY_NOT_FOUND_OR_PRIVATE"
    | "REPOSITORY_UNAVAILABLE"
    | "UPSTREAM_RATE_LIMITED"
    | "UPSTREAM_TIMEOUT"
    | "UNSUPPORTED_PLATFORM"
    | "CLONE_FAILED"
    | "ANALYSIS_FAILED"
    | "REPORT_GENERATION_FAILED"
    | "INTERNAL_ERROR";

  message: string;
}