import type { Finding, Recommendation, Report } from '@/types';
import { isKnownStatus, METRIC_LABELS } from './metricFormatting';

export const CATEGORY_LABELS = {
  activity: 'Активность и регулярность',
  documentation: 'Документация',
  testing: 'Тестирование',
  security: 'Безопасность',
  ci_cd: 'CI/CD',
};

export const SEVERITY_LABELS = {
  critical: 'Критическая',
  high: 'Высокая',
  medium: 'Средняя',
  low: 'Низкая',
  informational: 'Информационная',
};

export const PRIORITY_LABELS = { high: 'Высокий', middle: 'Средний', low: 'Низкий' };
export const COMPLEXITY_LABELS = { high: 'Высокая', medium: 'Средняя', low: 'Низкая' };

const SEVERITY_ORDER = ['critical', 'high', 'medium', 'low', 'informational'];
const PRIORITY_ORDER = ['high', 'middle', 'low'];

function compareCodePoints(left: string, right: string): number {
  const a = Array.from(left);
  const b = Array.from(right);
  for (let index = 0; index < Math.min(a.length, b.length); index += 1) {
    const difference = a[index].codePointAt(0)! - b[index].codePointAt(0)!;
    if (difference !== 0) return difference;
  }
  return a.length - b.length;
}

function compareText(left: string, right: string): number {
  return compareCodePoints(
    left.normalize('NFC').toLowerCase(),
    right.normalize('NFC').toLowerCase(),
  );
}

function hasStrings(value: Record<string, unknown>, fields: string[]): boolean {
  return fields.every((field) => typeof value[field] === 'string');
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function uniqueItems<T extends { id: string }>(
  value: unknown,
  valid: (item: unknown) => item is T,
): value is T[] {
  return (
    Array.isArray(value) &&
    value.length <= Object.keys(METRIC_LABELS).length &&
    value.every(valid) &&
    value.every((item: T) => item.id.length > 0) &&
    new Set(value.map((item: T) => item.id)).size === value.length
  );
}

export function validFindings(value: unknown): value is Finding[] {
  if (
    !uniqueItems(value, (item): item is Finding => {
      if (!isObject(item)) return false;
      return (
        hasStrings(item, [
          'id',
          'rule_id',
          'category',
          'title',
          'description',
          'possible_consequence',
        ]) &&
        Object.hasOwn(CATEGORY_LABELS, String(item.category)) &&
        Object.hasOwn(METRIC_LABELS, String(item.rule_id)) &&
        SEVERITY_ORDER.includes(String(item.severity)) &&
        Array.isArray(item.evidence_ids) &&
        item.evidence_ids.every((id) => typeof id === 'string') &&
        Array.isArray(item.vulnerabilities) &&
        item.vulnerabilities.every(
          (value) =>
            isObject(value) &&
            hasStrings(value, ['vulnerability_id', 'ecosystem', 'package', 'version']) &&
            (value.evidence_url === null || typeof value.evidence_url === 'string'),
        )
      );
    })
  )
    return false;
  return new Set(value.map((item) => item.rule_id)).size === value.length;
}

export function validRecommendations(value: unknown): value is Recommendation[] {
  if (
    !uniqueItems(value, (item): item is Recommendation => {
      if (!isObject(item)) return false;
      return (
        hasStrings(item, [
          'id',
          'finding_id',
          'title',
          'action',
          'expected_benefit',
          'explanation',
        ]) &&
        PRIORITY_ORDER.includes(String(item.priority)) &&
        Object.hasOwn(COMPLEXITY_LABELS, String(item.complexity)) &&
        ['short', 'expanded'].includes(String(item.variant_id))
      );
    })
  )
    return false;
  return new Set(value.map((item) => item.finding_id)).size === value.length;
}

export function sortFindings(findings: Finding[]): Finding[] {
  return [...findings].sort(
    (a, b) =>
      SEVERITY_ORDER.indexOf(a.severity) - SEVERITY_ORDER.indexOf(b.severity) ||
      compareText(CATEGORY_LABELS[a.category], CATEGORY_LABELS[b.category]) ||
      compareText(a.title, b.title) ||
      compareCodePoints(a.id, b.id),
  );
}

export function sortRecommendations(recommendations: Recommendation[]): Recommendation[] {
  return [...recommendations].sort(
    (a, b) =>
      PRIORITY_ORDER.indexOf(a.priority) - PRIORITY_ORDER.indexOf(b.priority) ||
      compareText(a.title, b.title) ||
      compareCodePoints(a.id, b.id),
  );
}

export function findingsScope(
  report: Report,
): 'complete' | 'partial' | 'unavailable' | 'not_applicable' | 'error' {
  const categories = Array.isArray(report.categories) ? report.categories : [];
  const metrics = categories.flatMap((category) =>
    Array.isArray(category?.metrics) ? category.metrics : [],
  );
  const expected = Object.keys(METRIC_LABELS);
  if (
    categories.length !== Object.keys(CATEGORY_LABELS).length ||
    categories.some(
      (category) =>
        !Object.hasOwn(CATEGORY_LABELS, category?.id) || !isKnownStatus(category?.status),
    ) ||
    new Set(categories.map((category) => category?.id)).size !== categories.length ||
    metrics.length !== expected.length ||
    expected.some((id) => metrics.filter((metric) => metric?.input?.id === id).length !== 1) ||
    metrics.some((metric) => !isKnownStatus(metric?.input?.status))
  )
    return 'error';
  if (metrics.every(({ input }) => input.status === 'not_applicable')) return 'not_applicable';
  if (
    !metrics.some(
      ({ input }) =>
        input.status === 'complete' ||
        (input.status === 'partial' && input.raw_value !== null && input.raw_value !== undefined),
    )
  )
    return 'unavailable';
  // A not-applicable metric is not evidence that the corresponding check found no issues.
  // Keep the empty findings state cautious unless every metric is complete.
  if (metrics.some(({ input }) => input.status !== 'complete')) return 'partial';
  return 'complete';
}
