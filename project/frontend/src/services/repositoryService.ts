import api, { ApiError } from './api';

export interface ValidatedRepository {
  repository: {
    platform: 'github';
    github_id: string;
    default_branch: string | null;
    commit_sha: string | null;
    owner: string;
    name: string;
    canonical_url: string;
  };
  moved_from: string | null;
}

export type AnalysisStatusValue = 'queued' | 'running' | 'completed' | 'failed';

export interface CreateAnalysisRequest {
  repository_url: string;
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
  error?: { code: string; message: string } | null;
}

function parseValidatedRepository(data: unknown): ValidatedRepository {
  const value = data as Partial<ValidatedRepository> | null;
  const repository = value?.repository;
  if (
    !repository ||
    repository.platform !== 'github' ||
    typeof repository.github_id !== 'string' ||
    !repository.github_id ||
    typeof repository.owner !== 'string' ||
    !/^[A-Za-z0-9-]+$/.test(repository.owner) ||
    typeof repository.name !== 'string' ||
    !/^[A-Za-z0-9_.-]+$/.test(repository.name) ||
    repository.canonical_url !==
      `https://github.com/${repository.owner}/${repository.name}`.toLowerCase() ||
    !(repository.default_branch === null || typeof repository.default_branch === 'string') ||
    !(
      repository.commit_sha === null ||
      (typeof repository.commit_sha === 'string' && /^[0-9a-f]{40}$/.test(repository.commit_sha))
    ) ||
    !(
      value?.moved_from === null ||
      (typeof value?.moved_from === 'string' &&
        /^https:\/\/github\.com\/[a-z0-9-]+\/[a-z0-9_.-]+$/.test(value.moved_from))
    )
  ) {
    throw new ApiError('REPOSITORY_CHECK_FAILED', 'Не удалось проверить доступность репозитория');
  }
  return value as ValidatedRepository;
}

export const repositoryService = {
  async validateRepository(
    repositoryUrl: string,
    signal?: AbortSignal,
  ): Promise<ValidatedRepository> {
    const { data } = await api.post<unknown>(
      '/repositories/validate',
      { repository_url: repositoryUrl },
      { signal },
    );
    return parseValidatedRepository(data);
  },

  async createAnalysis(payload: CreateAnalysisRequest): Promise<CreateAnalysisResponse> {
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
