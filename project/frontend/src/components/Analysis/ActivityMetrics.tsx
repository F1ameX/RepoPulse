import type { Evidence, Metric, MetricId, MetricInput, Report } from '@/types';
import { formatUtc } from '@/lib/analysisView';
import styles from './Analysis.module.css';

const TARGET_METRICS: MetricId[] = ['ACT_COMMIT_FREQ', 'ACT_DAYS_SINCE_LAST', 'ACT_TREND'];

const METRIC_LABELS: Partial<Record<MetricId, string>> = {
  ACT_COMMIT_FREQ: 'Частота коммитов',
  ACT_DAYS_SINCE_LAST: 'Дней с последнего коммита',
  ACT_TREND: 'Тренд активности',
};

function formatRawValue(id: MetricId, raw: unknown): string {
  if (raw === null || raw === undefined) return 'N/A';
  if (typeof raw === 'boolean') return raw ? 'Да' : 'Нет';
  if (typeof raw === 'string') return raw;
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    if (id === 'ACT_COMMIT_FREQ') return raw.toFixed(2);
    if (id === 'ACT_DAYS_SINCE_LAST') return Math.round(raw).toString();
    if (id === 'ACT_TREND') return raw.toFixed(4);
    return String(raw);
  }
  return 'N/A';
}

function formatSample(sample: MetricInput['sample']): string {
  const { collected, total, numerator, denominator } = sample;
  if (collected === null && total === null && numerator === null && denominator === null) {
    return 'Не определено';
  }
  const parts: string[] = [];
  if (collected !== null) parts.push(`собрано: ${collected}`);
  if (total !== null) parts.push(`всего: ${total}`);
  if (numerator !== null) parts.push(`числитель: ${numerator}`);
  if (denominator !== null) parts.push(`знаменатель: ${denominator}`);
  return parts.join(', ');
}

function isSafeUrl(url: string | null | undefined): url is string {
  return typeof url === 'string' && url.startsWith('https://');
}

function EvidenceList({
  evidence,
  limited,
  total,
}: {
  evidence: Evidence[];
  limited: boolean;
  total: number;
}) {
  if (evidence.length === 0) return null;
  return (
    <div className={styles.evidenceBlock}>
      <strong>Evidence:</strong>
      <ul>
        {evidence.map((ev) => (
          <li key={ev.id}>
            <span>{ev.description}</span>
            {ev.path && <span className={styles.mono}> ({ev.path})</span>}
            {isSafeUrl(ev.url) && (
              <>
                {' '}
                <a href={ev.url} target="_blank" rel="noopener noreferrer">
                  [ссылка]
                </a>
              </>
            )}
          </li>
        ))}
      </ul>
      {limited && total > evidence.length && (
        <p className={styles.hint}>
          Показано {evidence.length} из {total}
        </p>
      )}
    </div>
  );
}

function MetricRow({ metric }: { metric: Metric }) {
  const { input } = metric;
  const label = METRIC_LABELS[input.id] ?? input.id;

  const isUnavailable =
    input.status === 'unavailable' ||
    input.status === 'not_applicable' ||
    input.raw_value === null ||
    input.raw_value === undefined;

  const displayRaw = isUnavailable ? 'N/A' : formatRawValue(input.id, input.raw_value);
  const reason = isUnavailable && input.reason
    ? input.reason.cannot_conclude || input.reason.message
    : null;

  return (
    <div className={styles.metricRow}>
      <dt className={styles.metricLabel}>{label}:</dt>
      <dd className={styles.metricDetails}>
        <div className={styles.metricValueLine}>
          <span>
            <strong>Сырое значение:</strong>{' '}
            <span className={styles.rawValue}>{displayRaw}</span>
          </span>
          {reason && <span className={styles.naReason}>({reason})</span>}
        </div>
        <div>
          <strong>Балл:</strong> {metric.normalized_score ?? 'N/A'}
        </div>
        <div>
          <strong>Окно:</strong>{' '}
          {input.window
            ? `[${formatUtc(input.window.from)}, ${formatUtc(input.window.until)})`
            : 'Не определено'}
        </div>
        <div>
          <strong>Выборка:</strong> {formatSample(input.sample)}
        </div>
        <div>
          <strong>Правило:</strong> {metric.rule}
        </div>
        <EvidenceList
          evidence={input.evidence}
          limited={input.evidence_limited}
          total={input.evidence_total ?? 0}
        />
      </dd>
    </div>
  );
}

export function ActivityMetrics({ report }: { report: Report }) {
  const activityCategory = report.categories.find((c) => c.id === 'activity');
  if (!activityCategory) return null;

  const metrics = TARGET_METRICS.map((id) =>
    activityCategory.metrics.find((m) => m.input.id === id),
  ).filter((m): m is Metric => m !== undefined);

  if (metrics.length === 0) return null;

  return (
    <section aria-label="Показатели активности" className={styles.activitySection}>
      <h2 className={styles.sectionTitle}>Активность репозитория</h2>
      {activityCategory.status === 'unavailable' && activityCategory.reason && (
        <p className={styles.sectionWarning} role="status">
          {activityCategory.reason.cannot_conclude || activityCategory.reason.message}
        </p>
      )}
      <dl className={styles.fields}>
        {metrics.map((metric) => (
          <MetricRow key={metric.input.id} metric={metric} />
        ))}
      </dl>
    </section>
  );
}