import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { ApiError } from '@/services/api';
import { authService } from '@/services/authService';
import { repositoryService } from '@/services/repositoryService';
import type { ValidatedRepository } from '@/services/repositoryService';
import { URL_MESSAGES } from '@/lib/repositoryUrl';
import RepositoryInputPage from './RepositoryInputPage';

const guest = { kind: 'guest' as const, login: null, expires_at: '2026-10-25T00:00:00Z' };
const valid: ValidatedRepository = {
  repository: {
    platform: 'github',
    github_id: '1',
    owner: 'owner',
    name: 'repo',
    canonical_url: 'https://github.com/owner/repo',
    default_branch: 'main',
    commit_sha: null,
  },
  moved_from: null,
};
const success = 'Репозиторий доступен. URL прошёл проверку.';
const temporary = 'Не удалось проверить доступность репозитория. Повторите попытку позже';
const input = () => screen.getByRole('textbox', { name: /URL репозитория/ }) as HTMLInputElement;
const submit = () => fireEvent.submit(input().closest('form')!);
const enter = (value = 'https://github.com/owner/repo') =>
  fireEvent.change(input(), { target: { value } });

function CurrentPath() {
  return <output aria-label="Текущий адрес">{useLocation().pathname}</output>;
}
function renderForm() {
  return render(
    <MemoryRouter>
      <RepositoryInputPage />
      <CurrentPath />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  sessionStorage.clear();
  vi.spyOn(authService, 'getSessionContext').mockResolvedValue({ account: null, guest });
  vi.spyOn(authService, 'createGuestSession').mockResolvedValue(guest);
  vi.spyOn(repositoryService, 'validateRepository').mockResolvedValue(valid);
  vi.spyOn(repositoryService, 'createAnalysis').mockResolvedValue({
    outcome: 'accepted',
    analysis_id: '6f4a2b18-7c93-4e51-9d20-83a6c0b145ef',
    report_id: null,
    status: 'queued',
    repository: valid.repository,
  });
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('repository input and analysis launch', () => {
  it.each([
    '',
    'https://gitlab.com/a/b',
    'https://github.com:443/a/b',
    'https://github.com/a/b?',
    'x'.repeat(2049),
  ])('rejects invalid input before any request: %s', async (value) => {
    renderForm();
    enter(value);
    submit();
    expect(screen.getByRole('alert').textContent).toBeTruthy();
    expect(input().getAttribute('aria-invalid')).toBe('true');
    expect(input().getAttribute('aria-describedby')?.split(' ')).toEqual([
      'repository-hint',
      'repository-error',
    ]);
    expect(document.activeElement).toBe(input());
    expect(input().value).toBe(value);
    expect(authService.getSessionContext).not.toHaveBeenCalled();
    expect(repositoryService.validateRepository).not.toHaveBeenCalled();
    enter();
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByText('Поддерживаются публичные репозитории GitHub')).toBeTruthy();
  });

  it('does not silently remove pasted line breaks', () => {
    renderForm();
    fireEvent.paste(input(), { clipboardData: { getData: () => 'https://github.com/a/b\n' } });
    submit();
    expect(screen.getByText(URL_MESSAGES.INVALID_REPOSITORY_URL)).toBeTruthy();
    expect(authService.getSessionContext).not.toHaveBeenCalled();
  });

  it('confirms a guest, validates original input and opens the accepted analysis', async () => {
    vi.mocked(authService.getSessionContext).mockResolvedValueOnce({ account: null, guest: null });
    renderForm();
    const raw = ' HTTPS://GitHub.com/Owner/Repo.git.git/ ';
    enter(raw);
    submit();
    await screen.findByText(success);
    expect(authService.createGuestSession).toHaveBeenCalledTimes(1);
    expect(authService.getSessionContext).toHaveBeenCalledTimes(3);
    expect(repositoryService.validateRepository).toHaveBeenCalledWith(raw, expect.any(AbortSignal));
    await waitFor(() => expect(repositoryService.createAnalysis).toHaveBeenCalledTimes(1));
    expect(repositoryService.createAnalysis).toHaveBeenCalledWith(
      { request_id: expect.any(String), repository_url: raw },
      expect.any(AbortSignal),
    );
    await waitFor(() =>
      expect(screen.getByLabelText('Текущий адрес').textContent).toBe(
        '/analysis/6f4a2b18-7c93-4e51-9d20-83a6c0b145ef',
      ),
    );
    expect(input().value).toBe(raw);
  });

  it('reuses a verified guest and does not treat letter case as a repository move', async () => {
    renderForm();
    enter('https://GitHub.com/Owner/Repo');
    submit();
    await screen.findByText(success);
    expect(authService.createGuestSession).not.toHaveBeenCalled();
    expect(screen.queryByText(/Репозиторий перемещён/)).toBeNull();
  });

  it('announces a server-confirmed move', async () => {
    vi.mocked(repositoryService.validateRepository).mockResolvedValue({
      ...valid,
      moved_from: 'https://github.com/old/repo',
    });
    renderForm();
    enter('https://github.com/old/repo');
    submit();
    await screen.findByText('Репозиторий перемещён: owner/repo');
  });

  it('blocks simultaneous submissions and disables controls while pending', async () => {
    let finish!: (value: ValidatedRepository) => void;
    vi.mocked(repositoryService.validateRepository).mockReturnValue(
      new Promise((resolve) => {
        finish = resolve;
      }),
    );
    renderForm();
    enter();
    submit();
    submit();
    await waitFor(() => expect(repositoryService.validateRepository).toHaveBeenCalledTimes(1));
    expect(screen.queryByText('Проверяем доступность репозитория')).toBeNull();
    expect(input().disabled).toBe(true);
    expect(
      (screen.getByRole('button', { name: 'Анализировать' }) as HTMLButtonElement).disabled,
    ).toBe(true);
    await act(async () => {
      finish(valid);
    });
    await screen.findByText(success);
    await waitFor(() => expect(input().disabled).toBe(false));
  });

  it.each([
    ['REPOSITORY_NOT_FOUND_OR_PRIVATE', 'Публичный репозиторий недоступен'],
    ['UPSTREAM_RATE_LIMITED', temporary],
    ['UPSTREAM_TIMEOUT', temporary],
    ['NETWORK_ERROR', temporary],
    ['INTERNAL_ERROR', temporary],
    ['AUTH_UNAVAILABLE', temporary],
    ['REPOSITORY_CHECK_FAILED', temporary],
  ])('provides explicit retry and correction for %s, then recovers', async (code, message) => {
    vi.mocked(repositoryService.validateRepository).mockRejectedValueOnce(
      new ApiError(code, 'sensitive upstream detail', 503),
    );
    renderForm();
    enter();
    submit();
    await screen.findByText(message);
    expect(screen.queryByText('sensitive upstream detail')).toBeNull();
    expect(screen.getByRole('button', { name: 'Изменить URL' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    await screen.findByText(success);
    expect(repositoryService.validateRepository).toHaveBeenCalledTimes(2);
    expect(input().value).toBe('https://github.com/owner/repo');
  });

  it('correction focuses and preserves input without another request', async () => {
    vi.mocked(repositoryService.validateRepository).mockRejectedValueOnce(
      new ApiError('REPOSITORY_NOT_FOUND_OR_PRIVATE', ''),
    );
    renderForm();
    enter();
    submit();
    fireEvent.click(await screen.findByRole('button', { name: 'Изменить URL' }));
    expect(document.activeElement).toBe(input());
    expect(input().value).toBe('https://github.com/owner/repo');
    expect(repositoryService.validateRepository).toHaveBeenCalledTimes(1);
  });

  it('shows cookie instructions if the new guest cannot be confirmed', async () => {
    vi.mocked(authService.getSessionContext).mockResolvedValue({ account: null, guest: null });
    renderForm();
    enter();
    submit();
    await screen.findByText('Для работы RepoPulse разрешите cookie для этого сайта');
    expect(repositoryService.validateRepository).not.toHaveBeenCalled();
    expect(authService.createGuestSession).toHaveBeenCalledTimes(1);
  });

  it('does not replace an expired account with a guest automatically', async () => {
    vi.mocked(authService.getSessionContext).mockRejectedValue(new ApiError('SESSION_EXPIRED', ''));
    renderForm();
    enter();
    submit();
    await screen.findByText('Сессия истекла. Войдите снова');
    expect(authService.createGuestSession).not.toHaveBeenCalled();
    expect(repositoryService.validateRepository).not.toHaveBeenCalled();
  });

  it('renders backend URL validation beside the input and focuses it', async () => {
    vi.mocked(repositoryService.validateRepository).mockRejectedValue(
      new ApiError('INVALID_REPOSITORY_URL', 'internal detail'),
    );
    renderForm();
    enter();
    submit();
    await screen.findByText(URL_MESSAGES.INVALID_REPOSITORY_URL);
    expect(document.activeElement).toBe(input());
  });

  it('aborts the pending request on unmount', async () => {
    vi.mocked(repositoryService.validateRepository).mockReturnValue(new Promise(() => {}));
    const view = renderForm();
    enter();
    submit();
    await waitFor(() => expect(repositoryService.validateRepository).toHaveBeenCalledTimes(1));
    const signal = vi.mocked(repositoryService.validateRepository).mock.calls[0][1];
    view.unmount();
    expect(signal?.aborted).toBe(true);
  });

  it('does not create a guest after leaving a pending session check', async () => {
    let resolveSession!: (value: { account: null; guest: null }) => void;
    vi.mocked(authService.getSessionContext).mockReturnValue(
      new Promise((resolve) => {
        resolveSession = resolve;
      }),
    );
    const view = renderForm();
    enter();
    submit();
    view.unmount();
    await act(async () => {
      resolveSession({ account: null, guest: null });
    });
    expect(authService.createGuestSession).not.toHaveBeenCalled();
    expect(repositoryService.validateRepository).not.toHaveBeenCalled();
  });

  it('does not replace a confirmed account with a guest', async () => {
    vi.mocked(authService.getSessionContext).mockResolvedValue({
      account: { kind: 'account', login: 'owner', expires_at: guest.expires_at },
      guest: null,
    });
    renderForm();
    enter();
    submit();
    await screen.findByText(success);
    expect(authService.createGuestSession).not.toHaveBeenCalled();
  });

  it('recovers from a session service outage without losing the URL', async () => {
    vi.mocked(authService.getSessionContext).mockRejectedValueOnce(
      new ApiError('INTERNAL_ERROR', 'HTTP 502', 502),
    );
    renderForm();
    enter();
    submit();
    await screen.findByText(temporary);
    expect(screen.queryByText('HTTP 502')).toBeNull();
    expect(repositoryService.validateRepository).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    await screen.findByText(success);
    expect(repositoryService.validateRepository).toHaveBeenCalledTimes(1);
    expect(input().value).toBe('https://github.com/owner/repo');
  });

  it('confirms the context on retry after a lost guest creation response', async () => {
    vi.mocked(authService.getSessionContext).mockResolvedValueOnce({ account: null, guest: null });
    vi.mocked(authService.createGuestSession).mockRejectedValueOnce(
      new ApiError('NETWORK_ERROR', 'request timed out'),
    );
    renderForm();
    enter();
    submit();
    await screen.findByText(temporary);
    expect(repositoryService.validateRepository).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    await screen.findByText(success);
    expect(authService.createGuestSession).toHaveBeenCalledTimes(1);
  });

  it('removes the old success when a changed URL fails local validation', async () => {
    renderForm();
    enter();
    submit();
    await screen.findByText(success);
    enter('https://github.com/owner/repo\u00a0');
    expect(screen.queryByText(success)).toBeNull();
    submit();
    expect(screen.getByText(URL_MESSAGES.INVALID_REPOSITORY_URL)).toBeTruthy();
    expect(authService.getSessionContext).toHaveBeenCalledTimes(2);
    expect(repositoryService.validateRepository).toHaveBeenCalledTimes(1);
  });
});
