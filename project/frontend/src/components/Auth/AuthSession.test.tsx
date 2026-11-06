import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import App from '@/App';
import { ApiError } from '@/services/api';
import { authService, type SessionContext } from '@/services/authService';
import { repositoryService } from '@/services/repositoryService';

afterEach(() => {
  cleanup();
  sessionStorage.clear();
  localStorage.clear();
  vi.restoreAllMocks();
});

it('hides protected history after an unconfirmed logout and supports a safe retry', async () => {
  let session: SessionContext = {
    account: { kind: 'account', login: 'flamex', expires_at: '2099-01-01T00:00:00Z' },
    guest: null,
  };
  vi.spyOn(authService, 'getSessionContext').mockImplementation(async () => session);
  vi.spyOn(repositoryService, 'listReports').mockResolvedValue({ items: [], next_cursor: null });
  const logout = vi
    .spyOn(authService, 'logout')
    .mockRejectedValueOnce(new ApiError('NETWORK_ERROR', ''))
    .mockImplementationOnce(async () => {
      session = { account: null, guest: null };
    });

  render(<App />);
  fireEvent.click(await screen.findByRole('link', { name: 'История отчётов' }));
  await screen.findByText('Сохранённых отчётов пока нет');
  fireEvent.click(screen.getByRole('button', { name: 'Выйти' }));
  await screen.findByText('Не удалось подтвердить выход');
  expect(screen.queryByText('Сохранённых отчётов пока нет')).toBeNull();
  expect(screen.queryByRole('button', { name: 'Анализировать' })).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: 'Повторить выход' }));
  await screen.findByRole('button', { name: 'Анализировать' });
  const navigation = within(screen.getByRole('navigation', { name: 'Аккаунт' }));
  expect(navigation.getByRole('link', { name: 'Войти' }).getAttribute('href')).toBe('/login');
  expect(logout).toHaveBeenCalledTimes(2);
});
