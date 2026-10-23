export const URL_MESSAGES = {
  URL_REQUIRED: 'Укажите URL репозитория',
  URL_TOO_LONG: 'URL должен содержать не более 2048 символов',
  INVALID_REPOSITORY_URL: 'Укажите ссылку вида https://github.com/owner/repository',
  UNSUPPORTED_PLATFORM: 'Поддерживаются только репозитории GitHub',
} as const;

export type RepositoryUrlResult =
  | { ok: true; canonicalUrl: string }
  | { ok: false; code: keyof typeof URL_MESSAGES; message: string };

// FR-01: inspect the original spelling before a parser can repair it.
export function validateRepositoryUrl(input: string): RepositoryUrlResult {
  const fail = (code: keyof typeof URL_MESSAGES): RepositoryUrlResult => ({
    ok: false,
    code,
    message: URL_MESSAGES[code],
  });
  const value = input.replace(/^ +| +$/g, '');
  if (!value) return fail('URL_REQUIRED');
  if ([...value].length > 2048) return fail('URL_TOO_LONG');
  if (
    [...value].some((ch) => ch.charCodeAt(0) <= 32 || ch.charCodeAt(0) >= 127) ||
    value.includes('\\') ||
    value.includes('%')
  )
    return fail('INVALID_REPOSITORY_URL');
  const parts = /^https:\/\/([^/?#]+)([^?#]*)(?:[?#].*)?$/i.exec(value);
  if (!parts) return fail('INVALID_REPOSITORY_URL');
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    return fail('INVALID_REPOSITORY_URL');
  }
  if (parsed.hostname.toLowerCase() !== 'github.com') return fail('UNSUPPORTED_PLATFORM');
  // URL.port omits an explicit default :443; inspect the raw authority.
  if (parts[1].toLowerCase() !== 'github.com' || /[?#]/.test(value))
    return fail('INVALID_REPOSITORY_URL');
  const path = parts[2].replace(/\/$/, '').split('/');
  if (
    path.length !== 3 ||
    path[0] !== '' ||
    !/^[A-Za-z0-9-]+$/.test(path[1]) ||
    !/^[A-Za-z0-9_.-]+$/.test(path[2])
  )
    return fail('INVALID_REPOSITORY_URL');
  const name = path[2].replace(/\.git$/, '');
  if (!name || name === '.' || name === '..') return fail('INVALID_REPOSITORY_URL');
  return {
    ok: true,
    canonicalUrl: 'https://github.com/' + path[1].toLowerCase() + '/' + name.toLowerCase(),
  };
}
