import type { Coverage, DataStatus, MetricId, Sample } from '@/types';

export const STATUS_LABELS: Record<DataStatus, string> = {
  complete: 'Полные данные',
  partial: 'Частичные данные',
  unavailable: 'Недоступно',
  not_applicable: 'Не применимо',
};

export const METRIC_LABELS: Partial<Record<MetricId, string>> = {
  ACT_COMMIT_FREQ: 'Частота коммитов',
  ACT_DAYS_SINCE_LAST: 'Дней с последнего коммита',
  ACT_TREND: 'Тренд активности',
};

const PERCENTAGE_IDS: ReadonlySet<MetricId> = new Set<MetricId>([
  'DOC_SUPPORT_SHARE',
  'CI_SUCCESS_RATE',
]);

export function roundHalfUp(value: number, decimals: number): string {
  if (value === 0) return (0).toFixed(decimals);

  const sign = Math.sign(value);
  const absValue = Math.abs(value);
  const str = absValue.toString();

  let shifted: number;
  if (str.includes('e')) {
    shifted = absValue * 10 ** decimals;
  } else {
    shifted = Number(`${str}e${decimals}`);
  }

  const rounded = Math.round(shifted);

  if (str.includes('e')) {
    return (sign * (rounded / 10 ** decimals)).toFixed(decimals);
  }
  return (sign * Number(`${rounded}e-${decimals}`)).toFixed(decimals);
}

function formatPercentHalfUp(raw: number, decimals: number): string {
  if (!Number.isFinite(raw)) return 'N/A';
  if (raw === 0) return `${(0).toFixed(decimals)}%`;

  const sign = Math.sign(raw);
  const absValue = Math.abs(raw);
  const str = absValue.toString();
  const shift = decimals + 2;

  let shifted: number;
  if (str.includes('e')) {
    shifted = absValue * 10 ** shift;
  } else {
    shifted = Number(`${str}e${shift}`);
  }
  const rounded = Math.round(shifted);
  const percentValue = rounded / 10 ** decimals;
  return `${(sign * percentValue).toFixed(decimals)}%`;
}

export function formatRawValue(id: MetricId, raw: unknown): string {
  if (raw === null || raw === undefined) return 'N/A';
  if (typeof raw === 'boolean') return raw ? 'Обнаружено' : 'Не обнаружено';
  if (typeof raw === 'string') return raw;
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    if (PERCENTAGE_IDS.has(id)) return formatPercentHalfUp(raw, 1);
    if (id === 'ACT_COMMIT_FREQ') return roundHalfUp(raw, 2);
    if (id === 'ACT_DAYS_SINCE_LAST') return Math.round(raw).toString();
    if (id === 'ACT_TREND') return roundHalfUp(raw, 4);
    return String(raw);
  }
  return 'N/A';
}

export function formatSample(sample: Sample, id: MetricId): string {
  const { collected, total, numerator, denominator, excluded, excluded_reason } = sample;
  const parts: string[] = [];
  if (collected !== null) parts.push(`собрано: ${collected}`);
  if (total !== null) parts.push(`всего: ${total}`);
  if (collected === null && total === null) {
    parts.push('Размер выборки неизвестен');
  } else if (collected !== null && total === null) {
    parts.push('полный объём неизвестен');
  } else if (collected === null && total !== null) {
    parts.push('собрано неизвестно');
  }
  if (numerator !== null) {
    parts.push(`${id === 'ACT_COMMIT_FREQ' ? 'коммитов' : 'числитель'}: ${numerator}`);
  }
  if (denominator !== null) {
    parts.push(`${id === 'ACT_COMMIT_FREQ' ? 'недель' : 'знаменатель'}: ${denominator}`);
  }
  if (excluded !== null) {
    parts.push(`исключено: ${excluded}${excluded_reason ? ` (${excluded_reason})` : ''}`);
  }
  return parts.join(', ');
}

export function formatCoverage(coverage: Coverage): string {
  if (coverage.total === 0) return 'Неизвестно';
  const pct = Number.isInteger(coverage.percentage)
    ? String(coverage.percentage)
    : coverage.percentage.toFixed(1);
  const parts: string[] = [`рассчитано: ${coverage.calculated} из ${coverage.total}`, `${pct}%`];
  if (coverage.partial > 0) parts.push(`частично: ${coverage.partial}`);
  if (coverage.unavailable > 0) parts.push(`недоступно: ${coverage.unavailable}`);
  if (coverage.not_applicable > 0) parts.push(`не применимо: ${coverage.not_applicable}`);
  return parts.join(', ');
}

export function isSafeUrl(url: string | null | undefined): url is string {
  if (!url || !/^https:\/\/[^/\\]/i.test(url)) return false;
  try {
    const parsed = new URL(url);
    return (
      parsed.protocol === 'https:' && !!parsed.hostname && !parsed.username && !parsed.password
    );
  } catch {
    return false;
  }
}
