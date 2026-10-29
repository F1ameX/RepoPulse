import api, { ApiError } from './api';
import type {
  ValidatedRepository,
  CreateAnalysisRequest,
  CreateAnalysisResponse,
  AnalysisStatus,
  Report,
} from '@/types';

export type {
  ValidatedRepository,
  CreateAnalysisRequest,
  CreateAnalysisResponse,
  AnalysisStatus,
  AnalysisStatusValue,
} from '@/types';

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

  async createAnalysis(
    payload: CreateAnalysisRequest,
    signal?: AbortSignal,
  ): Promise<CreateAnalysisResponse> {
    const { data } = await api.post('/analyses', payload, {
      timeout: 35_000,
      signal,
    });
    return data;
  },

  async getAnalysis(analysisId: string, signal?: AbortSignal): Promise<AnalysisStatus> {
    const { data } = await api.get(`/analyses/${encodeURIComponent(analysisId)}`, {
      timeout: 15_000,
      signal,
    });
    return data;
  },

  async getReport(reportId: string, signal?: AbortSignal): Promise<Report> {
    const { data } = await api.get(`/reports/${encodeURIComponent(reportId)}`, {
      timeout: 15_000,
      signal,
    });
    return data;
  },
};
