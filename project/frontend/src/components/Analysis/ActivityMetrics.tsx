import type {
  DataStatus,
  Evidence,
  Metric,
  MetricId,
  MetricInput,
  Report,
} from '@/types';
import { formatUtc } from '@/lib/analysisView';
import styles from './Analysis.module.css';

const TARGET_METRICS: MetricId[] = [
  'ACT_COMMIT_FREQ',
  'ACT_DAYS_SINCE_LAST',
  'ACT_TREND',
];

const METRIC_LABELS: Partial<Record<MetricId, string>> = {
  ACT_COMMIT_FREQ: 'Частота коммитов',
  ACT_DAYS_SINCE_LAST: 'Дней с последнего коммита',
  ACT_TREND: 'Тренд активности',
};

const STATUS_LABELS: Record<DataStatus, string> = {
  complete: 'Полные данные',
  partial: 'Частичные данные',
  unavailable: 'Недоступно',
  not_applicable: 'Не применимо',
};

function roundHalfUp(value: number, decimals: number): string {
  if (value === 0) return (0).toFixed(decimals);
  const sign = Math.sign(value);
  const absValue = Math.abs(value);
  const shifted = Number(`${absValue}e${decimals}`);
  const rounded = Math.round(shifted);
  return (sign * Number(`${rounded}e-${decimals}`)).toFixed(decimals);
}

function formatRawValue(id: MetricId, raw: unknown): string {
  if (raw === null || raw === undefined) return 'N/A';
  if (typeof raw === 'boolean') return raw ? 'Да' : 'Нет';
  if (typeof raw === 'string') return raw;
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    if (id === 'ACT_COMMIT_FREQ') return roundHalfUp(raw, 2);
    if (id === 'ACT_DAYS_SINCE_LAST') return Math.round(raw).toString();
    if (id === 'ACT_TREND') return roundHalfUp(raw, 4);
    return String(raw);
  }
  return 'N/A';
}

function formatSample(sample: MetricInput['sample']): string {
  const { collected, total, numerator, denominator } = sample;
  if (
    collected === null &&
    total === null &&
    numerator === null &&
    denominator === null
  ) {
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
  if (!url) return false;
  try {
    const parsed = new URL(url);
    return (
      parsed.protocol === 'https:' &&
      !!parsed.hostname &&
      !parsed.username &&
      !parsed.password
    );
  } catch {
    return false;
  }
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
  if (!evidence || evidence.length === 0) {
    return (
      <div className={styles.evidenceBlock}>
        <strong>Evidence:</strong>{' '}
        <span className={styles.mono}>Основания не сохранены</span>
      </div>
    );
  }

  return (
    <div className={styles.evidenceBlock}>
      <strong>Evidence:</strong>
      <ul>
        {evidence.map((ev) => (
          <li key={ev.id}>
            <span>{ev.description}</span>
            {/* Используем шаблонные строки, чтобы React не разбивал текст на куски */}
            <span className={styles.mono}>{` (source: ${ev.source})`}</span>
            {ev.path && <span className={styles.mono}>{` [${ev.path}]`}</span>}
            {ev.revision && (
              <span className={styles.mono}>{` rev: ${ev.revision}`}</span>
            )}
            {isSafeUrl(ev.url) ? (
              <>
                {' '}
                <a href={ev.url} target="_blank" rel="noopener noreferrer">
                  [ссылка]
                </a>
              </>
            ) : (
              <span className={styles.mono}>
                {' '}
                (Ссылка на источник недоступна)
              </span>
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

  const displayRaw = isUnavailable
    ? 'N/A'
    : formatRawValue(input.id, input.raw_value);

  const reason =
    isUnavailable && input.reason
      ? {
          specific: input.reason.message,
          general: input.reason.cannot_conclude,
        }
      : null;

  return (
    <div className={styles.metricRow}>
      <dt className={styles.metricLabel}>{label}:</dt>
      <dd className={styles.metricDetails}>
        <div className={styles.metricValueLine}>
          <span>
            <strong>Сырое значение:</strong>{' '}
            <span className={styles.rawValue}>{displayRaw}</span>
            {input.unit && (
              <span className={styles.mono}> {input.unit}</span>
            )}
          </span>
          {reason && (
            <span className={styles.naReason}>
              ({reason.specific}. {reason.general})
            </span>
          )}
        </div>
        <div>
          <strong>Балл:</strong> {metric.normalized_score ?? 'N/A'}
        </div>
        <div>
          <strong>Состояние:</strong>{' '}
          <span className={styles.mono}>
            {STATUS_LABELS[input.status] ?? input.status}
          </span>
        </div>
        <div>
          <strong>Окно:</strong>{' '}
          {input.window
            ? `[${formatUtc(input.window.from)}, ${formatUtc(input.window.until)})`
            : 'Не определено'}
        </div>
        {input.observed_at && (
          <div>
            <strong>Наблюдалось:</strong> {formatUtc(input.observed_at)}
          </div>
        )}
        <div>
          <strong>Выборка:</strong> {formatSample(input.sample)}
        </div>
        {input.series && input.series.length > 0 && (
          <div>
            <strong>Ряд:</strong>{' '}
            <span className={styles.mono}>{input.series.join(', ')}</span>
          </div>
        )}
        <div>
          <strong>Правило:</strong> {metric.rule}
          {metric.threshold !== null && ` (порог: ${metric.threshold})`}
        </div>
        <div>
          <strong>Версия правила:</strong>{' '}
          <span className={styles.mono}>{metric.rule_version}</span>
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
    <section
      aria-label="Показатели активности"
      className={styles.activitySection}
    >
      <h2 className={styles.sectionTitle}>Активность репозитория</h2>
      {activityCategory.status === 'unavailable' && activityCategory.reason && (
        <p className={styles.sectionWarning} role="status">
          {activityCategory.reason.cannot_conclude ||
            activityCategory.reason.message}
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