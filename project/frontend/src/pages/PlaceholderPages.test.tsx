import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, expect, it } from 'vitest';
import AnalysisStatusPage from './AnalysisStatusPage';
import ReportPage from './ReportPage';
import ErrorPage from './ErrorPage';

afterEach(cleanup);

it.each([AnalysisStatusPage, ReportPage])(
  'does not invent an analysis or report before C2',
  (Page) => {
    render(
      <MemoryRouter>
        <Page />
      </MemoryRouter>,
    );
    expect(screen.getByText(/пока не реализован/)).toBeTruthy();
    expect(screen.getByRole('link', { name: 'К проверке репозитория' }).getAttribute('href')).toBe(
      '/',
    );
    expect(screen.queryByText(/выполняется|завершён/)).toBeNull();
  },
);

it('offers safe recovery from an unavailable page', () => {
  render(
    <MemoryRouter>
      <ErrorPage />
    </MemoryRouter>,
  );
  expect(screen.getByRole('alert').textContent).toContain('Не удалось открыть страницу.');
  expect(screen.getByRole('link').getAttribute('href')).toBe('/');
});
