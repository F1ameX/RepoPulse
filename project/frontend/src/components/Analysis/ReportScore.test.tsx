import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, expect, it, vi } from 'vitest';
import { repositoryService } from '@/services/repositoryService';
import type { Report } from '@/types';
import { AnalysisResult } from './AnalysisResult';
import { CategoryList } from './CategoryList';
import { ReportScore } from './ReportScore';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function report(): Report {
  const data: Report = JSON.parse(
    readFileSync(resolve('../../contracts/examples/report-zero.json'), 'utf8'),
  );
  data.overall_score = 0;
  data.overall_reason = null;
  data.summary = 'Сохранённое объяснение общей оценки';
  data.sources = [];
  data.findings = [];
  data.recommendations = [];
  data.coverage = {
    calculated: 12,
    total: 12,
    percentage: 100,
    partial: 0,
    unavailable: 0,
    not_applicable: 0,
  };
  data.categories.forEach((category) => {
    category.score = 0;
    category.status = 'complete';
    category.reason = null;
    category.coverage = {
      ...data.coverage,
      calculated: category.metrics.length,
      total: category.metrics.length,
    };
    category.metrics.forEach((metric) => {
      metric.input.status = 'complete';
      metric.input.reason = null;
      metric.input.raw_value = 0;
      metric.normalized_score = 0;
    });
  });
  return data;
}

it.each([
  [0, '0,0 / 100'],
  [100, '100,0 / 100'],
  [36.787944, '36,8 / 100'],
  [12.25, '12,3 / 100'],
])('displays saved Score %s with decimal half-up without recomputing categories', (score, text) => {
  const data = report();
  data.overall_score = score;
  const before = JSON.stringify(data);
  render(<ReportScore report={data} />);
  expect(screen.getByText(text)).toBeTruthy();
  expect(screen.getByText(data.summary)).toBeTruthy();
  expect(screen.getByText('Рассчитано метрик: 12 из 12 (100,0%)')).toBeTruthy();
  expect(screen.queryByText(/Оценка основана на неполных данных/)).toBeNull();
  expect(JSON.stringify(data)).toBe(before);
});

it('uses saved coverage and links an incomplete Score to this report limitations', () => {
  const data = report();
  data.coverage = { ...data.coverage, calculated: 1, percentage: 8.333333, unavailable: 11 };
  render(
    <>
      <ReportScore report={data} />
      <CategoryList report={data} />
    </>,
  );
  expect(screen.getByText('Рассчитано метрик: 1 из 12 (8,3%)')).toBeTruthy();
  expect(screen.getByText(/Оценка основана на неполных данных/)).toBeTruthy();
  expect(screen.getByText(/Частичные данные: 0 · Недоступно: 11 · Неприменимо: 0/)).toBeTruthy();
  fireEvent.click(screen.getByRole('link', { name: 'Ограничения анализа' }));
  expect(document.activeElement?.id).toBe('report-limitations');
});

it('warns about source limitations even when all metrics have a saved score', () => {
  const data = report();
  data.sources = [
    {
      source: 'osv',
      scope: 'osv',
      status: 'partial',
      reason: { code: 'timeout', message: 'Источник не ответил вовремя', cannot_conclude: '' },
      sample: data.categories[0].metrics[0].input.sample,
      metric_ids: ['SEC_VULN_RISK'],
      obtained_at: null,
      fingerprint: null,
    },
  ];
  render(<ReportScore report={data} />);
  expect(screen.getByText(/Оценка основана на неполных данных/)).toBeTruthy();
});

it('keeps all five categories available when all metrics are not applicable', () => {
  const data = report();
  const reason = { code: 'not_applicable', message: 'Нет применимых метрик', cannot_conclude: '' };
  data.overall_score = null;
  data.overall_reason = reason;
  data.coverage = { ...data.coverage, calculated: 0, percentage: 0, not_applicable: 12 };
  data.categories.forEach((category) => {
    category.score = null;
    category.status = 'not_applicable';
    category.reason = reason;
    category.coverage = {
      ...category.coverage,
      calculated: 0,
      percentage: 0,
      not_applicable: category.metrics.length,
    };
    category.metrics.forEach((metric) => {
      metric.normalized_score = null;
      metric.input.raw_value = null;
      metric.input.status = 'not_applicable';
      metric.input.reason = reason;
    });
  });
  render(
    <>
      <ReportScore report={data} />
      <CategoryList report={data} />
    </>,
  );
  const score = within(screen.getByRole('region', { name: 'Repo Health Score' }));
  expect(score.getByText('Недостаточно данных для общей оценки')).toBeTruthy();
  expect(score.getByText('Нет применимых метрик')).toBeTruthy();
  expect(score.queryByText(/\/ 100/)).toBeNull();
  expect(score.queryByText(/Оценка основана на неполных данных/)).toBeNull();
  expect(screen.getAllByText('Неприменимо')).toHaveLength(5);
  expect(screen.getAllByText('Показать основания')).toHaveLength(5);
});

it.each([-1, 101, '0', undefined, NaN, Infinity])(
  'rejects invalid Score %s and retries reading',
  (value) => {
    const data = report();
    Object.assign(data, { overall_score: value });
    const reload = vi.fn();
    render(<ReportScore report={data} onReload={reload} />);
    expect(screen.getByText('Не удалось загрузить оценку')).toBeTruthy();
    expect(screen.queryByText('Недостаточно данных для общей оценки')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Повторить загрузку' }));
    expect(reload).toHaveBeenCalledOnce();
  },
);

it.each([
  { total: 11 },
  { calculated: 13 },
  { partial: -1 },
  { unavailable: undefined },
  { not_applicable: 0.5 },
  { percentage: 101 },
])('rejects malformed coverage %j', (coverage) => {
  const data = report();
  Object.assign(data.coverage, coverage);
  render(<ReportScore report={data} />);
  expect(screen.getByText('Не удалось загрузить оценку')).toBeTruthy();
});

it('does not treat a missing reason for null Score as a valid N/A result', () => {
  const data = report();
  data.overall_score = null;
  render(<ReportScore report={data} />);
  expect(screen.getByText('Не удалось загрузить оценку')).toBeTruthy();
});

it.each(['missing', 'duplicate', 'score', 'metric', 'coverage'])(
  'keeps valid categories visible when another category has a %s error',
  (kind) => {
    const data = report();
    const security = data.categories.find(({ id }) => id === 'security')!;
    if (kind === 'missing') data.categories = data.categories.filter(({ id }) => id !== 'security');
    if (kind === 'duplicate') data.categories.push(security);
    if (kind === 'score') security.score = 101;
    if (kind === 'metric') security.metrics[1] = security.metrics[0];
    if (kind === 'coverage') security.coverage.calculated = -1;
    data.categories.reverse();
    const reload = vi.fn();
    render(<CategoryList report={data} onReload={reload} />);
    const categories = within(screen.getByRole('region', { name: 'Категории отчёта' }));
    expect(
      categories.getAllByRole('heading', { level: 3 }).map(({ textContent }) => textContent),
    ).toEqual([
      'Активность и регулярность',
      'Документация',
      'Тестирование',
      'Безопасность',
      'CI/CD',
    ]);
    const failed = within(categories.getByRole('region', { name: 'Категория: Безопасность' }));
    expect(failed.getByText('Не удалось загрузить категорию')).toBeTruthy();
    expect(categories.getAllByText('Показать основания')).toHaveLength(4);
    fireEvent.click(failed.getByRole('button', { name: 'Повторить загрузку' }));
    expect(reload).toHaveBeenCalledOnce();
  },
);

it('opens all five saved directions and preserves Score and date after remount without POST', async () => {
  const data = report();
  data.overall_score = 12.25;
  const getReport = vi.spyOn(repositoryService, 'getReport').mockResolvedValue(data);
  const createAnalysis = vi.spyOn(repositoryService, 'createAnalysis');
  const mount = () =>
    render(
      <MemoryRouter>
        <AnalysisResult reportId={data.report_id} />
      </MemoryRouter>,
    );
  const first = mount();
  await screen.findByText('12,3 / 100');
  const savedDate = within(screen.getByRole('region', { name: 'Шапка отчёта' })).getByText(
    'Завершён:',
  ).nextElementSibling?.textContent;
  const security = screen.getByRole('region', { name: 'Категория: Безопасность' });
  fireEvent.click(within(security).getByText('Показать основания'));
  expect(security.querySelector('details')?.open).toBe(true);
  expect(
    screen.getByRole('region', { name: 'Категория: CI/CD' }).querySelector('details')?.open,
  ).toBe(false);
  expect(within(security).getByText('Конфигурация SAST')).toBeTruthy();
  first.unmount();
  mount();
  await screen.findByText('12,3 / 100');
  expect(
    within(screen.getByRole('region', { name: 'Шапка отчёта' })).getByText('Завершён:')
      .nextElementSibling?.textContent,
  ).toBe(savedDate);
  expect(getReport).toHaveBeenCalledTimes(2);
  expect(getReport.mock.calls.every(([id]) => id === data.report_id)).toBe(true);
  expect(createAnalysis).not.toHaveBeenCalled();
});
