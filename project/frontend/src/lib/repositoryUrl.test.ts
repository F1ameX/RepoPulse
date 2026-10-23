import { describe, expect, it } from 'vitest';
import { validateRepositoryUrl, URL_MESSAGES } from './repositoryUrl';

describe('FR-01 URL validation', () => {
  it.each([
    ['https://github.com/Owner/Repo', 'https://github.com/owner/repo'],
    [' HTTPS://GitHub.Com/Owner/Repo.git/ ', 'https://github.com/owner/repo'],
    ['https://github.com/a-b/repo_1.2-3', 'https://github.com/a-b/repo_1.2-3'],
    ['https://github.com/a/repo.git.git', 'https://github.com/a/repo.git'],
    ['https://github.com/a/repo.GIT', 'https://github.com/a/repo.git'],
  ])('normalizes %s exactly once', (input, canonicalUrl) => {
    expect(validateRepositoryUrl(input)).toEqual({ ok: true, canonicalUrl });
  });

  it.each([
    ['', 'URL_REQUIRED'],
    ['   ', 'URL_REQUIRED'],
    ['https://gitlab.com/owner/repo', 'UNSUPPORTED_PLATFORM'],
    ['https://www.github.com/owner/repo', 'UNSUPPORTED_PLATFORM'],
    ['https://github.com.evil.test/owner/repo', 'UNSUPPORTED_PLATFORM'],
    ...[
      'http://github.com/a/b',
      'github.com/a/b',
      '//github.com/a/b',
      'https:github.com/a/b',
      'https:///github.com/a/b',
      'https://user@github.com/a/b',
      'https://github.com:443/a/b',
      'https://github.com/a/b?',
      'https://github.com/a/b#',
      'https://github.com/a/b?q=1',
      'https://github.com/a/b#readme',
      'https://github.com/a',
      'https://github.com/a/',
      'https://github.com//a/b',
      'https://github.com/a/b//',
      'https://github.com/a/b/tree/main',
      'https://github.com/a/../b',
      'https://github.com/a/.',
      'https://github.com/a/..',
      'https://github.com/a/.git',
      'https://github.com/a/..git',
      'https://github.com/a/...git',
      'https://github.com/a_b/repo',
      'https://github.com/a/b%2F',
      'https://github.com/a/b c',
      'https://github.com/я/b',
      'https://github.com/a/b\\',
      '\thttps://github.com/a/b',
      'https://github.com/a/b\n',
      'https://github.com/a/b\r',
      'https://github.com/a/b\u00a0',
      'https://gitlab.com/a/b\n',
      'http://gitlab.com/a/b',
    ].map((input) => [input, 'INVALID_REPOSITORY_URL']),
  ])('rejects %j with %s', (input, code) => {
    expect(validateRepositoryUrl(input)).toEqual({
      ok: false,
      code,
      message: URL_MESSAGES[code as keyof typeof URL_MESSAGES],
    });
  });

  it('accepts 2048 characters and rejects 2049 without truncation', () => {
    const prefix = 'https://github.com/a/';
    const input = prefix + 'b'.repeat(2048 - prefix.length);
    expect(validateRepositoryUrl('  ' + input + '  ')).toEqual({ ok: true, canonicalUrl: input });
    expect(validateRepositoryUrl(input + 'b')).toMatchObject({ code: 'URL_TOO_LONG' });
  });

  it('counts Unicode code points and checks length before syntax', () => {
    expect(validateRepositoryUrl('😀'.repeat(2048))).toMatchObject({
      code: 'INVALID_REPOSITORY_URL',
    });
    expect(validateRepositoryUrl('😀'.repeat(2049))).toMatchObject({ code: 'URL_TOO_LONG' });
  });
});
