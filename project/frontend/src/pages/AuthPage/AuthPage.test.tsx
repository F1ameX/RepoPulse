import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, expect, it, vi } from 'vitest';
import { ApiError } from '@/services/api';
import { authService, type SessionInfo } from '@/services/authService';
import { repositoryService } from '@/services/repositoryService';
import type { Report } from '@/types';
import AuthPage from './AuthPage';

const password = 'A long password 1';
const report = '/report/00000000-0000-0000-0000-000000000001';
const session: SessionInfo = {
  kind: 'account',
  login: 'flamex',
  expires_at: '2099-01-01T00:00:00Z',
};

afterEach(() => {
  cleanup();
  sessionStorage.clear();
  localStorage.clear();
  vi.restoreAllMocks();
});

function openForm(path: '/register' | '/login', returnTo?: string) {
  render(
    <MemoryRouter initialEntries={[{ pathname: path, state: { returnTo } }]}>
      <Routes>
        <Route path="/register" element={<AuthPage key="register" mode="register" />} />
        <Route path="/login" element={<AuthPage key="login" mode="login" />} />
        <Route path="/history" element={<h1>История</h1>} />
        <Route path="/report/:id" element={<h1>Целевой отчёт</h1>} />
      </Routes>
    </MemoryRouter>,
  );
}

function fill(login: string, secret: string) {
  fireEvent.change(screen.getByLabelText(/Логин/), { target: { value: login } });
  fireEvent.change(screen.getByLabelText(/Пароль/), { target: { value: secret } });
}

function submit() {
  fireEvent.submit(screen.getByLabelText(/Пароль/).closest('form')!);
}

it('keeps both fields after local validation and never sends invalid credentials', () => {
  const register = vi.spyOn(authService, 'register');
  openForm('/register');
  expect(screen.queryByRole('alert')).toBeNull();
  fill('1x_', password);
  submit();
  expect(screen.getByRole('alert').textContent).toContain('Начните логин с английской буквы');
  expect(screen.getByLabelText<HTMLInputElement>(/Логин/).value).toBe('1x_');
  expect(screen.getByLabelText<HTMLInputElement>(/Пароль/).value).toBe(password);
  expect(register).not.toHaveBeenCalled();
});

it('deduplicates registration and opens login with only the normalized login', async () => {
  let finish!: (value: { login: string; message: string }) => void;
  const register = vi.spyOn(authService, 'register').mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  openForm('/register');
  fill(' FlameX ', password);
  submit();
  submit();
  expect(register).toHaveBeenCalledTimes(1);
  expect(register).toHaveBeenCalledWith({ login: 'flamex', password }, expect.any(AbortSignal));
  await act(async () => finish({ login: 'flamex', message: 'Аккаунт создан. Войдите в систему' }));
  await screen.findByText('Аккаунт создан. Войдите в систему');
  expect(screen.getByLabelText<HTMLInputElement>(/Логин/).value).toBe('flamex');
  expect(screen.getByLabelText<HTMLInputElement>(/Пароль/).value).toBe('');
  expect(Object.values(sessionStorage)).not.toContain(password);
});

it.each([
  ['INVALID_CREDENTIALS', 'Неверный логин или пароль'],
  ['NETWORK_ERROR', 'Не удалось завершить вход'],
])('clears the password after %s while keeping the login', async (code, message) => {
  vi.spyOn(authService, 'login').mockRejectedValue(new ApiError(code, 'internal detail'));
  openForm('/login');
  fill('FlameX', password);
  submit();
  await screen.findByText(message);
  expect(screen.getByLabelText<HTMLInputElement>(/Пароль/).value).toBe('');
  expect(screen.getByLabelText<HTMLInputElement>(/Логин/).value).toBe('flamex');
});

it('keeps the registration action after a lost response without retrying the password', async () => {
  const register = vi
    .spyOn(authService, 'register')
    .mockRejectedValue(new ApiError('NETWORK_ERROR', ''));
  openForm('/register');
  fill('FlameX', password);
  submit();
  await screen.findByText(/Не удалось подтвердить регистрацию/);
  expect(screen.getByRole('button', { name: 'Зарегистрироваться' })).toBeTruthy();
  expect(screen.getByRole('button', { name: 'Продолжить как гость' })).toBeTruthy();
  expect(screen.queryByRole('link', { name: 'Войти' })).toBeNull();
  expect(screen.getByLabelText<HTMLInputElement>(/Пароль/).value).toBe('');
  expect(register).toHaveBeenCalledTimes(1);
});

it('clears the password and reports an unconfirmed registration after 15 seconds', async () => {
  const deadline = new AbortController();
  const timeout = vi.spyOn(AbortSignal, 'timeout').mockReturnValue(deadline.signal);
  vi.spyOn(authService, 'register').mockImplementation(
    (_, signal) =>
      new Promise((_, reject) => {
        signal?.addEventListener('abort', () => reject(signal.reason), { once: true });
      }),
  );
  openForm('/register');
  fill('FlameX', password);
  submit();
  expect(timeout).toHaveBeenCalledWith(15_000);
  await act(async () => deadline.abort());
  await screen.findByText(/Не удалось подтвердить регистрацию/);
  expect(screen.getByLabelText<HTMLInputElement>(/Логин/).value).toBe('flamex');
  expect(screen.getByLabelText<HTMLInputElement>(/Пароль/).value).toBe('');
});

it('reports missing cookies and keeps the user on login', async () => {
  vi.spyOn(authService, 'login').mockResolvedValue(session);
  vi.spyOn(authService, 'getSessionContext').mockResolvedValue({ account: null, guest: null });
  openForm('/login');
  fill('FlameX', password);
  submit();
  await screen.findByText(/разрешите cookie/);
  expect(screen.getByLabelText<HTMLInputElement>(/Пароль/).value).toBe('');
  expect(screen.queryByRole('heading', { name: 'История' })).toBeNull();
});

it.each([
  [report, 'Целевой отчёт'],
  ['https://other.example/report/1', 'История'],
  [undefined, 'История'],
])('confirms the account cookie before navigating from %s', async (target, heading) => {
  vi.spyOn(authService, 'login').mockResolvedValue(session);
  const check = vi
    .spyOn(authService, 'getSessionContext')
    .mockResolvedValue({ account: session, guest: null });
  const getReport = vi
    .spyOn(repositoryService, 'getReport')
    .mockResolvedValue({ report_id: report.slice('/report/'.length) } as Report);
  openForm('/login', target);
  fill('FlameX', password);
  submit();
  await screen.findByRole('heading', { name: heading });
  await waitFor(() => expect(check).toHaveBeenCalledTimes(1));
  expect(getReport).toHaveBeenCalledTimes(target === report ? 1 : 0);
});

it('opens history when the server denies access to the requested report', async () => {
  vi.spyOn(authService, 'login').mockResolvedValue(session);
  vi.spyOn(authService, 'getSessionContext').mockResolvedValue({ account: session, guest: null });
  vi.spyOn(repositoryService, 'getReport').mockRejectedValue(new ApiError('REPORT_NOT_FOUND', ''));
  openForm('/login', report);
  fill('FlameX', password);
  submit();
  await screen.findByRole('heading', { name: 'История' });
});
