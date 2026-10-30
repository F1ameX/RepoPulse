import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, expect, it } from 'vitest';
import AnalysisStatusPage from './AnalysisStatusPage';
import ReportPage from './ReportPage';
import ErrorPage from './ErrorPage';

afterEach(cleanup);

it.each([AnalysisStatusPage, ReportPage])('does not reveal metadata without a valid ID', (Page) => {
  render(
    <MemoryRouter>
      <Page />
    </MemoryRouter>,
  );
  expect(screen.getByRole('alert').textContent).toMatch(/недоступен/);
  expect(screen.getByRole('link', { name: 'Новый анализ' }).getAttribute('href')).toBe('/');
  expect(screen.queryByText(/выполняется|завершён/)).toBeNull();
});

it('offers safe recovery from an unavailable page', () => {
  render(
    <MemoryRouter>
      <ErrorPage />
    </MemoryRouter>,
  );
  expect(screen.getByRole('alert').textContent).toContain('Не удалось открыть страницу.');
  expect(screen.getByRole('link').getAttribute('href')).toBe('/');
});
