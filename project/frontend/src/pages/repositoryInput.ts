export const REPOSITORY_URL = {
  maxCodePoints: 2048,
  protocol: "https:",
  host: "github.com",
  gitSuffix: ".git",
} as const;

export const REPOSITORY_MESSAGES = {
  required: "Укажите URL репозитория",
  tooLong: `URL должен содержать не более ${REPOSITORY_URL.maxCodePoints} символов`,
  format: "Укажите ссылку вида https://github.com/owner/repository",
  unsupported: "Поддерживаются только репозитории GitHub",
  pending: "Проверяем доступность репозитория",
  available: "Публичный репозиторий доступен",
  unavailable: "Публичный репозиторий недоступен",
  moved: (name: string) => `Репозиторий перемещён: ${name}`,
  temporary: (code: string) =>
    `Не удалось проверить доступность репозитория: ошибка ${code}.\nПовторите попытку позже`,
} as const;

export type UrlValidation =
  | { ok: true; canonicalUrl: string }
  | { ok: false; message: string };

export function validateRepositoryUrl(input: string): UrlValidation {
  const messages = REPOSITORY_MESSAGES;

  const fail = (
    message: string = REPOSITORY_MESSAGES.format,
  ): UrlValidation => ({
    ok: false,
    message,
  });

  const value = input.replace(/^ +| +$/g, "");

  if (!value) {
    return fail(messages.required);
  }

  if ([...value].length > REPOSITORY_URL.maxCodePoints) {
    return fail(messages.tooLong);
  }

  if (/[^\x21-\x7e]|[\\%]/.test(value)) {
    return fail();
  }

  const urlMatch = /^https:\/\/([^/?#]+)(.*)$/i.exec(value);

  if (!urlMatch) {
    return fail();
  }

  const [, authority, path] = urlMatch;

  try {
    const parsedUrl = new URL(value);

    if (parsedUrl.hostname.toLowerCase() !== REPOSITORY_URL.host) {
      return fail(messages.unsupported);
    }
  } catch {
    return fail();
  }

  if (authority.toLowerCase() !== REPOSITORY_URL.host) {
    return fail();
  }

  const pathMatch = /^\/([A-Za-z0-9-]+)\/([A-Za-z0-9_.-]+)\/?$/.exec(path);

  if (!pathMatch) {
    return fail();
  }

  const [, owner, repository] = pathMatch;
  const suffix = REPOSITORY_URL.gitSuffix;

  const name = repository.endsWith(suffix)
    ? repository.slice(0, -suffix.length)
    : repository;

  if (!name || name === "." || name === "..") {
    return fail();
  }

  return {
    ok: true,
    canonicalUrl:
      `${REPOSITORY_URL.protocol}//${REPOSITORY_URL.host}/` +
      `${owner.toLowerCase()}/${name.toLowerCase()}`,
  };
}

const FIELD_ERRORS: Readonly<Record<string, string>> = {
  URL_REQUIRED: REPOSITORY_MESSAGES.required,
  URL_TOO_LONG: REPOSITORY_MESSAGES.tooLong,
  INVALID_REPOSITORY_URL: REPOSITORY_MESSAGES.format,
  UNSUPPORTED_PLATFORM: REPOSITORY_MESSAGES.unsupported,
};

export function repositoryErrorMessage(code: string): string {
  if (code === "REPOSITORY_NOT_FOUND_OR_PRIVATE")
    return REPOSITORY_MESSAGES.unavailable;

  const fieldMessage = Object.hasOwn(FIELD_ERRORS, code)
    ? FIELD_ERRORS[code]
    : undefined;

  return fieldMessage ?? REPOSITORY_MESSAGES.temporary(code);
}

export type ConfirmedRepository = { canonicalUrl: string; movedName?: string };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

export function readConfirmedRepository(
  value: unknown,
  requestedUrl: string,
): ConfirmedRepository | null {
  if (!isRecord(value) || !isRecord(value.repository)) return null;

  const repository = value.repository;

  if (
    repository.platform !== "github" ||
    typeof repository.github_id !== "string" ||
    !repository.github_id ||
    typeof repository.canonical_url !== "string" ||
    typeof repository.owner !== "string" ||
    typeof repository.name !== "string"
  )
    return null;

  const canonicalUrl = `${REPOSITORY_URL.protocol}//${REPOSITORY_URL.host}/${repository.owner.toLowerCase()}/${repository.name.toLowerCase()}`;

  if (
    !/^[A-Za-z0-9-]+$/.test(repository.owner) ||
    !/^[A-Za-z0-9_.-]+$/.test(repository.name) ||
    repository.name === "." ||
    repository.name === ".." ||
    repository.canonical_url !== canonicalUrl
  )
    return null;

  const moved = value.moved_from !== null && value.moved_from !== undefined;

  if (moved && value.moved_from !== requestedUrl) return null;

  if (canonicalUrl !== requestedUrl && !moved) return null;

  return {
    canonicalUrl,
    ...(moved ? { movedName: `${repository.owner}/${repository.name}` } : {}),
  };
}
