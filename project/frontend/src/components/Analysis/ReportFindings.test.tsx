import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import type { Evidence, Finding, Recommendation, Report } from '@/types';
import { sortFindings, sortRecommendations } from '@/lib/reportFindings';
import { ReportFindings } from './ReportFindings';

afterEach(cleanup);

function report(): Report {
  return controlReport('report-zero');
}

function controlReport(name: string): Report {
  return JSON.parse(
    readFileSync(resolve('../../contracts/examples', `${name}.json`), 'utf8'),
  ) as Report;
}

it.each(['report-zero', 'report-partial'])(
  '%s renders every pair from the contract DTO and keeps fallback actions',
  (name) => {
    const data = controlReport(name);
    render(<ReportFindings report={data} />);
    const findings = within(screen.getByRole('region', { name: 'Проблемы' }));
    const recommendations = within(screen.getByRole('region', { name: 'Рекомендации' }));
    expect(findings.getAllByRole('heading', { level: 3 })).toHaveLength(data.findings.length);
    expect(recommendations.getAllByRole('heading', { level: 3 })).toHaveLength(
      data.recommendations.length,
    );
    for (const item of data.recommendations) {
      expect(recommendations.getByText(item.action, { exact: false })).toBeTruthy();
      expect(
        recommendations.getAllByText(item.expected_benefit, { exact: false }).length,
      ).toBeGreaterThan(0);
      expect(recommendations.getAllByText(item.explanation).length).toBeGreaterThan(0);
    }
    expect(screen.queryByText(/AI/)).toBeNull();
  },
);

it('orders by severity, category, NFC/lowercase title and ID without mutating the DTO', () => {
  const base = report().findings[0];
  const items: Finding[] = [
    { ...base, id: 'b', title: 'e\u0301', severity: 'high' },
    { ...base, id: 'a', title: 'É', severity: 'high' },
    { ...base, id: 'c', title: 'Other', severity: 'critical' },
    { ...base, id: 'd', category: 'activity', severity: 'high' },
    { ...base, id: 'low', severity: 'low' },
    { ...base, id: 'medium', severity: 'medium' },
    { ...base, id: 'info', severity: 'informational' },
  ];
  expect(sortFindings(items).map(({ id }) => id)).toEqual([
    'c',
    'd',
    'a',
    'b',
    'medium',
    'low',
    'info',
  ]);
  expect(items.map(({ id }) => id)).toEqual(['b', 'a', 'c', 'd', 'low', 'medium', 'info']);
  const rec = report().recommendations[0];
  const recommendations: Recommendation[] = [
    { ...rec, id: 'b', title: 'e\u0301', priority: 'middle' },
    { ...rec, id: 'a', title: 'É', priority: 'middle' },
    { ...rec, id: 'c', priority: 'high' },
    { ...rec, id: 'low', priority: 'low' },
  ];
  expect(sortRecommendations(recommendations).map(({ id }) => id)).toEqual(['c', 'a', 'b', 'low']);
});

it('sorts by displayed category names and Unicode code points, including supplementary characters', () => {
  const base = report().findings[0];
  const items: Finding[] = [
    { ...base, id: 'testing', category: 'testing' },
    { ...base, id: 'ci', category: 'ci_cd' },
    { ...base, id: 'security', category: 'security' },
  ];
  expect(sortFindings(items).map(({ id }) => id)).toEqual(['ci', 'security', 'testing']);
  const titles = [
    { ...base, id: 'supplementary', title: '\u{10000}' },
    { ...base, id: 'bmp', title: '\uE000' },
  ];
  expect(sortFindings(titles).map(({ id }) => id)).toEqual(['bmp', 'supplementary']);
  const rec = report().recommendations[0];
  expect(
    sortRecommendations(titles.map(({ title, id }) => ({ ...rec, title, id }))).map(({ id }) => id),
  ).toEqual(['bmp', 'supplementary']);
  expect(
    sortRecommendations([
      { ...rec, id: 'a' },
      { ...rec, id: 'A' },
    ]).map(({ id }) => id),
  ).toEqual(['A', 'a']);
});

it('keeps the displayed order after remounting with the same report', () => {
  const data = report();
  const first = render(<ReportFindings report={data} />);
  const titles = screen.getAllByRole('heading', { level: 3 }).map((element) => element.textContent);
  first.unmount();
  render(<ReportFindings report={data} />);
  expect(
    screen.getAllByRole('heading', { level: 3 }).map((element) => element.textContent),
  ).toEqual(titles);
});

it.each(['complete', 'partial', 'unavailable', 'not_applicable'] as const)(
  'distinguishes the empty %s list',
  (status) => {
    const data = report();
    data.findings = [];
    data.recommendations = [];
    data.categories.forEach((category) =>
      category.metrics.forEach(({ input }) => {
        input.status = status;
      }),
    );
    render(<ReportFindings report={data} />);
    const messages = {
      complete: 'По выполненным проверкам проблемы не обнаружены',
      partial:
        'По доступным результатам проблемы не обнаружены, но часть проверок неполная или недоступна; отсутствие проблем не подтверждено.',
      unavailable: 'Недостаточно данных для выявления проблем',
      not_applicable: 'Применимые проверки отсутствуют',
    };
    expect(
      within(screen.getByRole('region', { name: 'Проблемы' })).getByText(messages[status]),
    ).toBeTruthy();
    expect(screen.getByText('Для этого отчёта рекомендации не сформированы')).toBeTruthy();
    expect(
      within(screen.getByRole('region', { name: 'Проблемы' })).queryByRole('link', {
        name: 'Ограничения анализа',
      }) !== null,
    ).toBe(['partial', 'unavailable'].includes(status));
  },
);

it('does not call an empty unavailable contract report a successful check', () => {
  render(<ReportFindings report={controlReport('report-null')} />);
  expect(screen.queryByText('По выполненным проверкам проблемы не обнаружены')).toBeNull();
  expect(screen.getByText('Недостаточно данных для выявления проблем')).toBeTruthy();
});

it('distinguishes malformed and missing lists and reloads the current report', () => {
  const data = report();
  const reload = vi.fn();
  Object.assign(data, { findings: null, recommendations: [{ id: 'broken' }] });
  render(<ReportFindings report={data} onReload={reload} />);
  expect(screen.getByText('Не удалось загрузить список проблем')).toBeTruthy();
  expect(screen.getByText('Не удалось загрузить рекомендации')).toBeTruthy();
  fireEvent.click(screen.getAllByRole('button', { name: 'Повторить загрузку' })[0]);
  expect(reload).toHaveBeenCalledOnce();
});

it('rejects duplicate IDs and missing metric states instead of reporting no problems', () => {
  const data = report();
  data.findings = [data.findings[0], data.findings[0]];
  const view = render(<ReportFindings report={data} />);
  expect(screen.getByText('Не удалось загрузить список проблем')).toBeTruthy();
  data.findings = [];
  data.recommendations = [];
  data.categories[0].metrics = [];
  view.rerender(<ReportFindings report={data} />);
  expect(screen.getByText('Не удалось загрузить список проблем')).toBeTruthy();
  expect(screen.queryByText('По выполненным проверкам проблемы не обнаружены')).toBeNull();
});

it('follows exact recommendation links and never substitutes an orphan finding', () => {
  const data = report();
  data.recommendations[0].finding_id = 'foreign';
  render(<ReportFindings report={data} />);
  expect(screen.getByText('Основание рекомендации недоступно')).toBeTruthy();
  const forward = screen.getByRole('link', { name: /^К рекомендации$/ });
  fireEvent.click(forward);
  expect(document.activeElement?.textContent).toContain(data.recommendations[1].title);
  const link = within(screen.getByRole('region', { name: 'Рекомендации' })).getByRole('link', {
    name: 'Показать основание',
  });
  fireEvent.click(link);
  const target = document.getElementById(link.getAttribute('href')!.slice(1));
  expect(document.activeElement).toBe(target);
  expect(target?.textContent).toContain(data.findings[1].title);
});

it('reveals the matching metric in the same report', () => {
  const data = report();
  render(
    <>
      <details>
        <summary>Документация</summary>
        <div id="metric-DOC_README" tabIndex={-1}>
          Основание README
        </div>
      </details>
      <ReportFindings report={data} />
    </>,
  );
  fireEvent.click(
    within(screen.getByRole('region', { name: 'Проблемы' })).getAllByRole('link', {
      name: 'Показать основание',
    })[0],
  );
  expect(document.querySelector('details')?.open).toBe(true);
  expect(document.activeElement?.id).toBe('metric-DOC_README');
});

it('renders markup as text and only opens HTTPS evidence without credentials', () => {
  const data = report();
  const metric = data.categories.find(({ id }) => id === 'documentation')!.metrics[0];
  const evidence: Evidence = {
    id: 'evidence',
    source: 'git',
    kind: 'file',
    description: '<img src=x onerror=alert(1)>',
    path: '<script>bad()</script>',
    revision: 'sha',
    url: 'javascript:alert(1)',
    run_id: null,
    attempt: null,
    dependency: null,
    vulnerability_id: null,
    secret_match_type: null,
  };
  metric.input.evidence = [evidence];
  data.findings[0].evidence_ids = [evidence.id];
  const view = render(<ReportFindings report={data} />);
  expect(screen.getByText(evidence.description)).toBeTruthy();
  expect(document.querySelector('img, script')).toBeNull();
  expect(screen.queryByRole('link', { name: '[ссылка]' })).toBeNull();
  evidence.url = 'https://user:password@example.com/source';
  view.rerender(<ReportFindings report={data} />);
  expect(screen.queryByRole('link', { name: '[ссылка]' })).toBeNull();
  evidence.url = 'https://github.com/owner/repo/blob/sha/file';
  data.categories[0].metrics[0].input.evidence = [{ ...evidence }];
  view.rerender(<ReportFindings report={data} />);
  expect(screen.getByRole('link', { name: '[ссылка]' }).getAttribute('target')).toBe('_blank');
  expect(screen.queryByText('Не удалось загрузить связанные основания')).toBeNull();
  data.findings[0].evidence_ids = ['missing'];
  view.rerender(<ReportFindings report={data} />);
  expect(screen.getByText('Не удалось загрузить связанные основания')).toBeTruthy();
  expect(screen.queryByRole('link', { name: '[ссылка]' })).toBeNull();
});

it('restricts secret findings to safe evidence and does not expose extra secret values', () => {
  const data = report();
  const secret = 'private-token-value';
  const finding = data.findings[0];
  Object.assign(finding, {
    rule_id: 'SEC_SECRETS',
    category: 'security',
    title: secret,
    description: 'Найдено совпадение с шаблоном секрета',
    possible_consequence: 'Возможен доступ к защищённым ресурсам',
    evidence_ids: ['secret', 'secret-2'],
  });
  const metric = data.categories.find(({ id }) => id === 'security')!.metrics[0];
  metric.input.evidence = [
    {
      id: 'secret',
      source: 'git',
      kind: 'secret',
      description: secret,
      path: '.env',
      revision: 'sha',
      url: `https://example.com/${secret}`,
      run_id: null,
      attempt: null,
      dependency: null,
      vulnerability_id: null,
      secret_match_type: 'Подозрительный шаблон',
      value: secret,
    } as Evidence,
  ];
  metric.input.evidence.push({ ...metric.input.evidence[0], id: 'secret-2', path: '.env.local' });
  render(<ReportFindings report={data} />);
  expect(screen.getByText('Потенциальный секрет')).toBeTruthy();
  expect(screen.getByText('Действительность реквизитов не проверялась')).toBeTruthy();
  expect(screen.getByText(finding.description)).toBeTruthy();
  expect(screen.getByText(finding.possible_consequence, { exact: false })).toBeTruthy();
  expect(document.body.textContent).not.toContain(secret);
  expect(document.querySelector(`a[href*="${secret}"]`)).toBeNull();
  expect(screen.getAllByRole('heading', { name: 'Потенциальный секрет' })).toHaveLength(1);
  expect(screen.getByText('[.env]')).toBeTruthy();
  expect(screen.getByText('[.env.local]')).toBeTruthy();
});

it('shows insufficient data when partial sources provide no confirmed value', () => {
  const data = report();
  data.findings = [];
  data.recommendations = [];
  data.categories.forEach((category) =>
    category.metrics.forEach(({ input }) => {
      input.status = 'partial';
      input.raw_value = null;
    }),
  );
  render(<ReportFindings report={data} />);
  expect(screen.getByText('Недостаточно данных для выявления проблем')).toBeTruthy();
  expect(screen.queryByText('По выполненным проверкам проблемы не обнаружены')).toBeNull();
});

it.each(['missing', 'severity', 'rule', 'priority', 'recommendation-id'])(
  'treats invalid %s data as a list error',
  (kind) => {
    const data = report();
    if (kind === 'missing') Reflect.deleteProperty(data, 'findings');
    if (kind === 'severity') Object.assign(data.findings[0], { severity: 'unknown' });
    if (kind === 'rule') data.findings[1].rule_id = data.findings[0].rule_id;
    if (kind === 'priority') Object.assign(data.recommendations[0], { priority: 'medium' });
    if (kind === 'recommendation-id') data.recommendations[1].id = data.recommendations[0].id;
    render(<ReportFindings report={data} />);
    expect(
      screen.getByText(
        kind === 'priority' || kind === 'recommendation-id'
          ? 'Не удалось загрузить рекомендации'
          : 'Не удалось загрузить список проблем',
      ),
    ).toBeTruthy();
  },
);

it('does not render an explanation mode for an empty set and preserves actions across modes', () => {
  const data = report();
  const view = render(<ReportFindings report={data} explanationMode={<p>Режим С7</p>} />);
  expect(screen.getByText('Режим С7')).toBeTruthy();
  const actions = data.recommendations.map(({ action }) => action);
  view.rerender(
    <ReportFindings
      report={{
        ...data,
        explanation: { ...data.explanation, mode: 'ai_catalog_selection', fallback_reason: null },
      }}
    />,
  );
  for (const action of actions) expect(screen.getByText(action, { exact: false })).toBeTruthy();
  data.recommendations = [];
  view.rerender(<ReportFindings report={data} explanationMode={<p>Режим С7</p>} />);
  expect(screen.getByText('Для этого отчёта рекомендации не сформированы')).toBeTruthy();
  expect(screen.queryByText('Режим С7')).toBeNull();
});

it('treats recommendations without any findings as inconsistent data', () => {
  const data = report();
  const action = data.recommendations[0].action;
  data.findings = [];
  render(<ReportFindings report={data} />);
  expect(screen.getByText('Не удалось загрузить рекомендации')).toBeTruthy();
  expect(screen.queryByText(action, { exact: false })).toBeNull();
});

it('links incomplete results to the limitations in this report', () => {
  const data = report();
  data.findings = [];
  data.recommendations = [];
  render(
    <>
      <ReportFindings report={data} />
      <section id="report-limitations" tabIndex={-1}>
        Ограничения
      </section>
    </>,
  );
  fireEvent.click(
    within(screen.getByRole('region', { name: 'Проблемы' })).getByRole('link', {
      name: 'Ограничения анализа',
    }),
  );
  expect(document.activeElement?.id).toBe('report-limitations');
});


it('FR-13/FR-22: renders OSV identity, unknown severity, Risk and declared Maven origin', () => {
  const data = report();
  const security = data.categories.find(({ id }) => id === 'security')!;
  const risk = security.metrics.find(({ input }) => input.id === 'SEC_VULN_RISK')!;
  risk.input.status = 'partial';
  risk.input.raw_value = 1.75;
  risk.normalized_score = 82.4;
  data.findings.push({
    ...data.findings[0],
    id: 'finding:SEC_VULN_RISK',
    rule_id: 'SEC_VULN_RISK',
    category: 'security',
    title: 'Уязвимость зависимости',
    description: 'Найдена уязвимость',
    severity: 'high',
    possible_consequence: 'Риск использования уязвимой версии',
    evidence_ids: [],
    vulnerabilities: [
      {
        vulnerability_id: 'GHSA-example',
        ecosystem: 'Maven',
        package: 'junit:junit',
        version: '4.13.2',
        severity: null,
        obtained_at: '2026-11-25T00:00:00Z',
        evidence_url: 'https://osv.dev/vulnerability/GHSA-example',
      },
    ],
  });

  render(<ReportFindings report={data} />);

  expect(screen.getByText('GHSA-example')).toBeTruthy();
  expect(screen.getByText(/Maven\/junit:junit@4.13.2/)).toBeTruthy();
  expect(screen.getByText('Уровень: Неизвестен')).toBeTruthy();
  expect(screen.getByText('Risk:')).toBeTruthy();
  expect(screen.getByText('1.75')).toBeTruthy();
  expect(screen.getByText(/Манифест: pom.xml · Источник версии: Декларация Maven/)).toBeTruthy();
  expect(screen.getByText(/фактический граф сборки не подтверждён/)).toBeTruthy();
});

it('FR-11/FR-13/FR-22: secret finding uses saved safe text and never renders secret evidence text', () => {
  const data = report();
  const secretMetric = data.categories.find(({ id }) => id === 'security')!.metrics.find(
    ({ input }) => input.id === 'SEC_SECRETS',
  )!;
  secretMetric.input.status = 'partial';
  secretMetric.input.evidence = [
    {
      id: 'secret-evidence',
      source: 'git',
      kind: 'secret_match',
      description: 'DO_NOT_RENDER_SECRET_VALUE',
      path: 'config/app.yml',
      revision: 'abc123',
      url: null,
      run_id: null,
      attempt: null,
      dependency: null,
      vulnerability_id: null,
      secret_match_type: 'suspicious_assignment',
    },
  ];
  data.findings.push({
    ...data.findings[0],
    id: 'finding:SEC_SECRETS',
    rule_id: 'SEC_SECRETS',
    category: 'security',
    title: 'DO_NOT_RENDER_SECRET_TITLE',
    description: 'Каталог: обнаружен потенциально чувствительный реквизит.',
    possible_consequence: 'При подтверждении значения возможна компрометация доступа.',
    severity: 'high',
    evidence_ids: ['secret-evidence'],
    vulnerabilities: [],
  });

  render(<ReportFindings report={data} />);

  expect(screen.getByText('Потенциальный секрет')).toBeTruthy();
  expect(screen.getByText('Каталог: обнаружен потенциально чувствительный реквизит.')).toBeTruthy();
  expect(
    screen.getByText(
      (_, element) =>
        element?.tagName === 'P' &&
        element.textContent?.includes('При подтверждении значения возможна компрометация доступа.') === true,
    ),
  ).toBeTruthy();
  expect(screen.getByText('suspicious_assignment')).toBeTruthy();
  expect(screen.getByText('config/app.yml')).toBeTruthy();
  expect(screen.getByText('abc123')).toBeTruthy();
  expect(screen.queryByText(/DO_NOT_RENDER_SECRET/)).toBeNull();
});


it('FR-11: not-applicable metric does not make otherwise complete checks partial', () => {
  const data = report();
  data.findings = [];
  data.recommendations = [];
  const risk = data.categories.find(({ id }) => id === 'security')!.metrics.find(
    ({ input }) => input.id === 'SEC_VULN_RISK',
  )!;
  risk.input.status = 'not_applicable';
  risk.input.raw_value = null;
  risk.normalized_score = null;
  data.categories.forEach((category) => {
    category.metrics.forEach((metric) => {
      if (metric.input.id !== 'SEC_VULN_RISK') {
        metric.input.status = 'complete';
      }
    });
  });

  render(<ReportFindings report={data} />);

  expect(
    within(screen.getByRole('region', { name: 'Проблемы' })).getByText(
      'По выполненным проверкам проблемы не обнаружены',
    ),
  ).toBeTruthy();
});
