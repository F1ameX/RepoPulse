import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, expect, it, vi } from 'vitest';
import { ApiError } from '@/services/api';
import { authService, type SessionContext } from '@/services/authService';
import { AnalysisAccess } from './AnalysisAccess';

const session: SessionContext = {
  account: null,
  guest: { kind: 'guest', login: null, expires_at: '2099-01-01T00:00:00Z' },
};

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

it('keeps confirmed content visible during access checks and temporary errors', async () => {
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
  let reject!: (error: Error) => void;
  const get = vi
    .spyOn(authService, 'getSessionContext')
    .mockResolvedValueOnce(session)
    .mockReturnValueOnce(
      new Promise((_, fail) => {
        reject = fail;
      }),
    )
    .mockResolvedValue(session);
  render(
    <MemoryRouter>
      <AnalysisAccess>
        <button>Проверить статус</button>
      </AnalysisAccess>
    </MemoryRouter>,
  );
  const button = await screen.findByRole('button', { name: 'Проверить статус' });
  const container = button.parentElement!;
  fireEvent(document, new Event('visibilitychange'));
  expect(container.hidden).toBe(false);
  expect(container.hasAttribute('inert')).toBe(true);
  expect(container.getAttribute('aria-busy')).toBe('true');
  await act(async () => {
    reject(new ApiError('NETWORK_ERROR', ''));
  });
  expect(container.hidden).toBe(false);
  expect(screen.getByText('Не удалось проверить сессию')).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: 'Повторить проверку доступа' }));
  await act(async () => {});
  expect(get).toHaveBeenCalledTimes(3);
  expect(container.hasAttribute('inert')).toBe(false);
  expect(screen.queryByText('Не удалось проверить сессию')).toBeNull();
});

it('removes confirmed content when the session is denied', async () => {
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
  vi.spyOn(authService, 'getSessionContext')
    .mockResolvedValueOnce(session)
    .mockResolvedValueOnce({ account: null, guest: null });
  render(
    <MemoryRouter>
      <AnalysisAccess>
        <p>Сохранённый отчёт</p>
      </AnalysisAccess>
    </MemoryRouter>,
  );
  await screen.findByText('Сохранённый отчёт');
  fireEvent(document, new Event('visibilitychange'));
  await screen.findByText('Сессия истекла. Войдите снова');
  expect(screen.queryByText('Сохранённый отчёт')).toBeNull();
});
