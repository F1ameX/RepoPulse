import api from './api';

export interface ValidatedRepository {
  platform: 'github' | 'gitlab';
  owner: string;
  name: string;
  canonical_url: string;
  visibility: 'public';
}

export type AnalysisStatusValue = 'queued' | 'running' | 'completed' | 'failed';

export interface CreateAnalysisRequest {
  canonical_url: string;
  request_id: string;
}

export interface CreateAnalysisResponse {
  analysis_id: string;
  status: AnalysisStatusValue;
  report_id?: string | null;
}

export interface AnalysisStatus {
  analysis_id: string;
  status: AnalysisStatusValue;
  report_id?: string | null;
}

export const repositoryService = {
  async validateRepository(url: string): Promise<ValidatedRepository> {
    const { data } = await api.post('/repositories/validate', { url });
    return data;
  },

  async createAnalysis(
    payload: CreateAnalysisRequest,
  ): Promise<CreateAnalysisResponse> {
    const { data } = await api.post('/analyses', payload);
    return data;
  },

  async getAnalysis(analysisId: string): Promise<AnalysisStatus> {
    const { data } = await api.get(`/analyses/${analysisId}`);
    return data;
  },

  async getReport(reportId: string): Promise<unknown> {
    const { data } = await api.get(`/reports/${reportId}`);
    return data;
  },
};

export type RepositoryErrorKind = 'permanent' | 'temporary';

export function classifyRepositoryError(code: string): RepositoryErrorKind {
  switch (code) {
    case 'UPSTREAM_RATE_LIMITED':
    case 'UPSTREAM_TIMEOUT':
    case 'NETWORK_ERROR':
    case 'REPOSITORY_UNAVAILABLE':
      return 'temporary';
    case 'INVALID_REPOSITORY_URL':
    case 'REPOSITORY_NOT_FOUND_OR_PRIVATE':
    case 'UNSUPPORTED_PLATFORM':
    case 'CLONE_FAILED':
    case 'ANALYSIS_FAILED':
    default:
      return 'permanent';
  }
}

export function normalizeUrl(input: string): string {
  return input
    .trim()
    .replace(/\/+$/, '')
    .replace(/\.git$/, '')
    .toLowerCase();
}