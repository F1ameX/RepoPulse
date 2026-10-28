import { afterEach, expect, it, vi } from 'vitest';
import api from './api';
import { repositoryService } from './repositoryService';

const valid = {
  repository: {
    platform: 'github',
    github_id: '1',
    owner: 'Owner',
    name: 'Repo',
    canonical_url: 'https://github.com/owner/repo',
    default_branch: null,
    commit_sha: null,
  },
  moved_from: null,
};
afterEach(() => vi.restoreAllMocks());

it.each([
  null,
  {},
  '<html>Proxy error</html>',
  { moved_from: null },
  { ...valid, moved_from: 'https://example.com/a/b' },
  { ...valid, repository: { ...valid.repository, github_id: null } },
  { ...valid, repository: { ...valid.repository, canonical_url: 'https://example.com/a/b' } },
  { ...valid, repository: { ...valid.repository, commit_sha: '' } },
])('does not declare success for a malformed API response: %j', async (data) => {
  vi.spyOn(api, 'post').mockResolvedValue({ data });
  await expect(
    repositoryService.validateRepository('https://github.com/owner/repo'),
  ).rejects.toMatchObject({
    code: 'REPOSITORY_CHECK_FAILED',
  });
});

it('accepts unknown branch/revision and preserves the original input and abort signal', async () => {
  const post = vi.spyOn(api, 'post').mockResolvedValue({ data: valid });
  const signal = new AbortController().signal;
  const url = ' HTTPS://GitHub.com/Owner/Repo.git/ ';
  await expect(repositoryService.validateRepository(url, signal)).resolves.toEqual(valid);
  expect(post).toHaveBeenCalledWith('/repositories/validate', { repository_url: url }, { signal });
});

it('does not strip .git twice from confirmed metadata', async () => {
  const data = {
    ...valid,
    repository: {
      ...valid.repository,
      name: 'repo.git',
      canonical_url: 'https://github.com/owner/repo.git',
    },
    moved_from: 'https://github.com/old/repo.git',
  };
  vi.spyOn(api, 'post').mockResolvedValue({ data });
  await expect(
    repositoryService.validateRepository('https://github.com/old/repo.git.git'),
  ).resolves.toEqual(data);
});
