export type RepositoryPlatform = 'github';
export type AnalysisStatusValue = 'queued' | 'running' | 'completed' | 'failed';
export type DataStatus = 'complete' | 'partial' | 'unavailable' | 'not_applicable';
export type DataSource = 'git' | 'github' | 'osv';
export type CategoryId = 'activity' | 'documentation' | 'testing' | 'security' | 'ci_cd';

export type MetricId =
  | 'ACT_COMMIT_FREQ'
  | 'ACT_DAYS_SINCE_LAST'
  | 'ACT_TREND'
  | 'DOC_README'
  | 'DOC_SUPPORT_SHARE'
  | 'TEST_PRESENCE'
  | 'SEC_SECRETS'
  | 'SEC_VULN_RISK'
  | 'SEC_SAST'
  | 'CI_PRESENT'
  | 'CI_SUCCESS_RATE'
  | 'CI_REQUIRED_CHECKS';

export type Severity = 'critical' | 'high' | 'medium' | 'low' | 'informational';
export type VulnerabilitySeverity = 'critical' | 'high' | 'medium' | 'low';
export type Priority = 'high' | 'middle' | 'low';
export type RecommendationComplexity = 'low' | 'medium' | 'high';
export type RecommendationVariant = 'short' | 'expanded';
export type DependencyEcosystem = 'npm' | 'Maven' | 'Go';
export type ExplanationMode = 'ai_catalog_selection' | 'deterministic_fallback';

export type ApiErrorCode =
  | 'VALIDATION_FAILED'
  | 'URL_REQUIRED'
  | 'URL_TOO_LONG'
  | 'INVALID_REPOSITORY_URL'
  | 'UNSUPPORTED_PLATFORM'
  | 'REPOSITORY_NOT_FOUND_OR_PRIVATE'
  | 'REPOSITORY_CHECK_FAILED'
  | 'SUBMISSION_CHANGED'
  | 'SUBMISSION_PENDING'
  | 'SESSION_REQUIRED'
  | 'SESSION_EXPIRED'
  | 'LOGIN_REQUIRED'
  | 'INVALID_CREDENTIALS'
  | 'LOGIN_TAKEN'
  | 'ANALYSIS_NOT_FOUND'
  | 'REPORT_NOT_FOUND'
  | 'UPSTREAM_RATE_LIMITED'
  | 'UPSTREAM_TIMEOUT'
  | 'BUDGET_EXCEEDED'
  | 'ANALYSIS_FAILED'
  | 'REPORT_SAVE_FAILED'
  | 'CSRF_REJECTED'
  | 'INTERNAL_ERROR'
  | 'METHOD_NOT_ALLOWED'
  | 'NOT_IMPLEMENTED'
  | 'NOT_FOUND';

export interface FieldError {
  field: string;
  code: string;
  message: string;
}

export interface ApiError {
  code: ApiErrorCode;
  message: string;
  trace_id: string;
  field_errors: FieldError[];
}

export interface ErrorEnvelope {
  error: ApiError;
}

export interface Repository {
  platform: RepositoryPlatform;
  github_id: string;
  canonical_url: string;
  owner: string;
  name: string;
  default_branch: string | null;
  commit_sha: string | null;
}

export interface Window {
  from: string;
  until: string;
}

export interface Versions {
  schema: string;
  methodology: string;
  scoring_config: string;
  collector_profile: string;
  secret_rules: string;
  recommendation_catalog: string;
  explanation_schema: string;
  prompt: string;
}

export interface Reason {
  code: string;
  message: string;
  cannot_conclude: string;
}

export interface Sample {
  collected: number | null;
  total: number | null;
  numerator: number | null;
  denominator: number | null;
  excluded: number | null;
  excluded_reason: string | null;
}

export interface SourceState {
  source: DataSource;
  scope: string;
  status: DataStatus;
  reason: Reason | null;
  sample: Sample;
  obtained_at: string | null;
  fingerprint: string | null;
}

export interface DependencyOrigin {
  manifest_path: string;
  version_source: string;
}

export interface Dependency {
  ecosystem: DependencyEcosystem;
  package: string;
  version: string;
  manifest_path: string;
  version_source: string;
  origins: DependencyOrigin[];
}

export interface Vulnerability {
  vulnerability_id: string;
  ecosystem: DependencyEcosystem;
  package: string;
  version: string;
  severity: VulnerabilitySeverity | null;
  obtained_at: string;
  evidence_url: string | null;
}

export interface Evidence {
  id: string;
  source: DataSource;
  kind: string;
  description: string;
  path: string | null;
  revision: string | null;
  url: string | null;
  run_id: string | null;
  attempt: number | null;
  dependency: Dependency | null;
  vulnerability_id: string | null;
  secret_match_type: string | null;
}

export interface ExcludedCount {
  reason: string;
  count: number;
}

export interface MetricInput {
  id: MetricId;
  raw_value: number | boolean | string | null;
  unit: string;
  status: DataStatus;
  reason: Reason | null;
  window: Window | null;
  sample: Sample;
  series: number[];
  evidence: Evidence[];
  evidence_limited: boolean;
  evidence_total: number | null;
  observed_at: string | null;
  exclusions: ExcludedCount[];
}

export interface ValidateRepositoryRequest {
  repository_url: string;
}

export interface ValidatedRepository {
  repository: Repository;
  moved_from: string | null;
}

export interface CreateAnalysisRequest {
  request_id: string;
  repository_url: string;
}

export interface CreateAnalysisResponse {
  outcome: 'accepted' | 'reused';
  analysis_id: string;
  report_id: string | null;
  status: AnalysisStatusValue;
  repository: Repository;
}

export interface AnalysisStatus {
  analysis_id: string;
  repository: Repository;
  status: AnalysisStatusValue;
  stage: string | null;
  created_at: string;
  started_at: string | null;
  completed_at: string | null;
  report_id: string | null;
  error: ApiError | null;
}

export interface Metric {
  input: MetricInput;
  normalized_score: number | null;
  rule_version: string;
  rule: string;
  threshold: number | null;
  explanation: string;
}

export interface Coverage {
  calculated: number;
  total: number;
  percentage: number;
  partial: number;
  unavailable: number;
  not_applicable: number;
}

export interface Category {
  id: CategoryId;
  score: number | null;
  status: DataStatus;
  reason: Reason | null;
  coverage: Coverage;
  effective_weight: number | null;
  explanation: string;
  metrics: Metric[];
}

export interface Finding {
  id: string;
  rule_id: MetricId;
  category: CategoryId;
  title: string;
  description: string;
  severity: Severity;
  possible_consequence: string;
  evidence_ids: string[];
  vulnerabilities: Vulnerability[];
}

export interface Recommendation {
  id: string;
  finding_id: string;
  title: string;
  action: string;
  expected_benefit: string;
  priority: Priority;
  complexity: RecommendationComplexity;
  variant_id: RecommendationVariant;
  explanation: string;
}

export interface Explanation {
  mode: ExplanationMode;
  fallback_reason: string | null;
  model: string | null;
  catalog_version: string;
  schema_version: string;
  prompt_version: string;
}

export interface Report {
  report_id: string;
  analysis_id: string;
  snapshot_id: string;
  repository: Repository;
  assessment_at: string;
  window: Window;
  completed_at: string;
  versions: Versions;
  overall_score: number | null;
  overall_reason: Reason | null;
  summary: string;
  coverage: Coverage;
  categories: Category[];
  sources: SourceState[];
  dependencies: Dependency[];
  findings: Finding[];
  recommendations: Recommendation[];
  explanation: Explanation;
}

export interface ReportSummary {
  report_id: string;
  repository: Repository;
  completed_at: string;
  overall_score: number | null;
  partial: boolean;
}

export interface HistoryPage {
  items: ReportSummary[];
  next_cursor: string | null;
}
