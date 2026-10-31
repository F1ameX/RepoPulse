import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, expect, it } from 'vitest';
import AnalysisStatusPage from './AnalysisStatusPage';
import ReportPage from './ReportPage';
import ErrorPage from './ErrorPage';

afterEach(cleanup);

it.each([
  [AnalysisStatusPage, '/analysis/not-a-uuid', /Анализ недоступен/],
  [ReportPage, '/report/not-a-uuid', /Отчёт недоступен/],
] as const)('invalid UUID shows a safe denial', (Page, path, message) => {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Page />
    </MemoryRouter>,
  );
  expect(screen.getByRole('alert').textContent).toMatch(message);
  expect(screen.getByRole('link', { name: 'Новый анализ' }).getAttribute('href')).toBe('/');
  expect(screen.queryByText(/выполняется|завершён/)).toBeNull();
});

it('ErrorPage offers safe recovery', () => {
  render(
    <MemoryRouter>
      <ErrorPage />
    </MemoryRouter>,
  );
  expect(screen.getByRole('alert').textContent).toContain('Не удалось открыть страницу.');
});