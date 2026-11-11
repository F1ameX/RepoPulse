import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, expect, it } from 'vitest';
import type { Category, Evidence, Metric, Report } from '@/types';
import { MetricDetails } from './MetricDetails';
import { CategoryCard } from './CategoryCard';
import { CategoryList } from './CategoryList';

afterEach(cleanup);

function makeMetric(
  overrides: Partial<Metric['input']> = {},
  metricOverrides: Partial<Metric> = {},
): Metric {
  return {
    input: {
      id: 'ACT_COMMIT_FREQ',
      raw_value: 0,
      unit: 'коммитов в неделю',
      status: 'complete',
      reason: null,
      window: null,
      sample: {
        collected: 0,
        total: 0,
        numerator: 0,
        denominator: 0,
        excluded: null,
        excluded_reason: null,
      },
      series: [],
      evidence: [],
      evidence_limited: false,
      evidence_total: 0,
      observed_at: null,
      exclusions: [],
      ...overrides,
    },
    normalized_score: 0,
    rule_version: 'v1',
    rule: '',
    threshold: null,
    explanation: '',
    ...metricOverrides,
  };
}

function makeCategory(overrides: Partial<Category> = {}): Category {
  return {
    id: 'activity',
    score: null,
    status: 'unavailable',
    reason: { code: 'X', message: 'Нет данных', cannot_conclude: 'Вывод невозможен' },
    coverage: {
      calculated: 0,
      total: 0,
      percentage: 0,
      partial: 0,
      unavailable: 0,
      not_applicable: 0,
    },
    effective_weight: null,
    explanation: '',
    metrics: [],
    ...overrides,
  };
}

it('FR-13: 0 — валидное значение, не N/A', () => {
  render(<MetricDetails metric={makeMetric({ raw_value: 0 })} />);
  expect(screen.getByText('0.00')).toBeTruthy();
  expect(screen.queryByText('N/A')).toBeNull();
});

it('FR-13: false рендерится как «Не обнаружено», не N/A', () => {
  render(<MetricDetails metric={makeMetric({ id: 'TEST_PRESENCE', raw_value: false })} />);
  expect(screen.getByText('Не обнаружено')).toBeTruthy();
  expect(screen.queryByText('N/A')).toBeNull();
});

it('FR-13: неизвестная выборка не выглядит нулевой', () => {
  render(
    <MetricDetails
      metric={makeMetric({
        sample: {
          collected: null,
          total: null,
          numerator: null,
          denominator: null,
          excluded: null,
          excluded_reason: null,
        },
      })}
    />,
  );
  expect(screen.getByText(/Размер выборки неизвестен/)).toBeTruthy();
  expect(screen.queryByText(/собрано: 0/)).toBeNull();
});

it('FR-10: единица берётся из input.unit, не хардкодится', () => {
  render(<MetricDetails metric={makeMetric({ unit: 'шт/нед' })} />);
  expect(screen.getByText(/шт\/нед/)).toBeTruthy();
});

it('FR-10: unavailable + reason → N/A и оба текста причины', () => {
  render(
    <MetricDetails
      metric={makeMetric(
        {
          raw_value: null,
          status: 'unavailable',
          reason: {
            code: 'timeout',
            message: 'Знаменатель равен нулю',
            cannot_conclude: 'Полная оценка недоступна',
          },
        },
        { normalized_score: null },
      )}
    />,
  );
  expect(screen.getByText('N/A')).toBeTruthy();
  expect(screen.getByText('Не рассчитан')).toBeTruthy();
  expect(screen.getAllByText(/Знаменатель равен нулю/).length).toBeGreaterThanOrEqual(1);
  expect(screen.getAllByText(/Полная оценка недоступна/).length).toBeGreaterThanOrEqual(1);
});

it('FR-10: javascript:/userinfo/секреты — не ссылки', () => {
  const base: Evidence = {
    id: 'e1',
    source: 'git',
    kind: 'commit',
    description: 'safe',
    path: null,
    revision: null,
    url: null,
    run_id: null,
    attempt: null,
    dependency: null,
    vulnerability_id: null,
    secret_match_type: null,
  };
  render(
    <MetricDetails
      metric={makeMetric({
        raw_value: 1,
        evidence: [
          { ...base, id: 'safe', url: 'https://github.com/a/b/commit/abc' },
          { ...base, id: 'js', url: 'javascript:alert(1)' },
          { ...base, id: 'userinfo', url: 'https://github.com@evil.example/' },
          { ...base, id: 'secret', secret_match_type: 'aws_key', url: 'https://github.com/a/b' },
        ],
        evidence_limited: false,
        evidence_total: 4,
      })}
    />,
  );
  expect(screen.getAllByRole('link', { name: '[ссылка]' })).toHaveLength(1);
  expect(screen.getAllByText(/Ссылка на источник недоступна/)).toHaveLength(2);
  expect(screen.getByText('aws_key')).toBeTruthy();
  expect(screen.queryByText('https://github.com/a/b')).toBeNull();
});

it('FR-09: карточка без балла — «Нет данных», причина и «Неизвестно»', () => {
  render(<CategoryCard category={makeCategory()} />);
  const score = screen.getByText('Балл').parentElement!;
  expect(score.textContent).toContain('Нет данных');
  expect(screen.getAllByText(/Нет данных/).length).toBeGreaterThanOrEqual(1);
  expect(screen.getByText(/Вывод невозможен/)).toBeTruthy();
  expect(screen.getByText('Неизвестно')).toBeTruthy();
});

it('FR-09: карточка с баллом, покрытием и раскрытием', () => {
  render(
    <CategoryCard
      category={makeCategory({
        score: 37.5,
        status: 'partial',
        reason: null,
        coverage: {
          calculated: 2,
          total: 3,
          percentage: 66.7,
          partial: 1,
          unavailable: 0,
          not_applicable: 0,
        },
        metrics: [makeMetric()],
      })}
      defaultOpen
    />,
  );
  expect(screen.getByText('37.5')).toBeTruthy();
  expect(screen.getByText(/рассчитано: 2 из 3/)).toBeTruthy();
  expect(screen.getByText(/66\.7%/)).toBeTruthy();
  expect(screen.getByText('Показатели (1)')).toBeTruthy();
});

it('FR-09: пустая категория без раскрытия', () => {
  render(<CategoryCard category={makeCategory({ metrics: [] })} />);
  expect(screen.queryByText(/Показатели \(/)).toBeNull();
  expect(screen.getByText('Показатели отсутствуют')).toBeTruthy();
});

it('CategoryList рендерит все категории', () => {
  const report = {
    categories: [
      makeCategory({ id: 'activity' }),
      makeCategory({ id: 'security', reason: null, status: 'complete', score: 100 }),
    ],
    repository: { commit_sha: null },
  } as unknown as Report;
  render(<CategoryList report={report} />);
  expect(screen.getByRole('region', { name: 'Категории отчёта' })).toBeTruthy();
  expect(screen.getByRole('region', { name: 'Категория: Активность' })).toBeTruthy();
  expect(screen.getByRole('region', { name: 'Категория: Безопасность' })).toBeTruthy();
});

it('FR-13: partial с причиной → причина показана рядом с баллом', () => {
  render(
    <MetricDetails
      metric={makeMetric({
        raw_value: 12,
        status: 'partial',
        reason: {
          code: 'partial_data',
          message: 'Получена часть истории',
          cannot_conclude: 'Полная оценка недоступна',
        },
      })}
    />,
  );
  expect(screen.getByText('12.00')).toBeTruthy();
  expect(screen.getAllByText(/Получена часть истории/).length).toBeGreaterThanOrEqual(1);
  expect(screen.getAllByText(/Полная оценка недоступна/).length).toBeGreaterThanOrEqual(1);
});

it('FR-13: collected известен, total — нет → явно сказано «полный объём неизвестен»', () => {
  render(
    <MetricDetails
      metric={makeMetric({
        sample: {
          collected: 20,
          total: null,
          numerator: null,
          denominator: null,
          excluded: null,
          excluded_reason: null,
        },
      })}
    />,
  );
  const sampleLine = screen.getByText(/Выборка:/).parentElement!;
  expect(sampleLine.textContent).toContain('собрано: 20');
  expect(sampleLine.textContent).toContain('полный объём неизвестен');
});

it('FR-10: порог и версия методики выводятся, rule — нет', () => {
  render(
    <MetricDetails
      metric={makeMetric(undefined, {
        rule: 'commits_in_window / weeks',
        rule_version: 'rhs-mvp-1.1.0',
        threshold: 10,
      })}
    />,
  );
  expect(screen.getByText(/Порог:/).parentElement?.textContent).toContain('10');
  expect(screen.getByText(/Версия методики:/).parentElement?.textContent).toContain(
    'rhs-mvp-1.1.0',
  );
  expect(screen.queryByText(/Правило:/)).toBeNull();
});

it('FR-10: CI_SUCCESS_RATE показывается процентом с одним знаком', () => {
  render(<MetricDetails metric={makeMetric({ id: 'CI_SUCCESS_RATE', raw_value: 0.75 })} />);
  expect(screen.getByText('75.0%')).toBeTruthy();
});

it('FR-10: DOC_SUPPORT_SHARE показывается процентом с одним знаком', () => {
  render(<MetricDetails metric={makeMetric({ id: 'DOC_SUPPORT_SHARE', raw_value: 0.4567 })} />);
  expect(screen.getByText('45.7%')).toBeTruthy();
});

it('FR-09: score null и все метрики not_applicable → «Неприменимо»', () => {
  render(
    <CategoryCard
      category={makeCategory({
        reason: null,
        status: 'not_applicable',
        metrics: [
          makeMetric({ status: 'not_applicable', raw_value: null }, { normalized_score: null }),
        ],
      })}
    />,
  );
  expect(screen.getByText('Неприменимо')).toBeTruthy();
  expect(screen.queryByText('Нет данных')).toBeNull();
});

it('FR-09: score null при наличии применимых метрик → «Нет данных»', () => {
  render(
    <CategoryCard
      category={makeCategory({ reason: null, status: 'unavailable', metrics: [makeMetric()] })}
    />,
  );
  expect(screen.getByText('Нет данных')).toBeTruthy();
  expect(screen.queryByText('Неприменимо')).toBeNull();
});

it('FR-09: category.explanation выводится в карточке', () => {
  render(
    <CategoryCard
      category={makeCategory({ reason: null, explanation: 'Пояснение категории' })}
    />,
  );
  expect(screen.getByText('Пояснение категории')).toBeTruthy();
});