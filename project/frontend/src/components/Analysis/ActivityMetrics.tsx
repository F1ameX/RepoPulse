import type { DataStatus, Evidence, Metric, MetricId, MetricInput, Reason, Report } from '@/types';
import { formatUtc } from '@/lib/analysisView';
import styles from './Analysis.module.css';

const TARGET_METRICS: MetricId[] = ['ACT_COMMIT_FREQ', 'ACT_DAYS_SINCE_LAST', 'ACT_TREND'];

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

function formatSample(sample: MetricInput['sample'], id: MetricId): string {
  const { collected, total, numerator, denominator, excluded, excluded_reason } = sample;
  const parts: string[] = [];
  if (collected !== null) parts.push(`собрано: ${collected}`);
  if (total !== null) parts.push(`всего: ${total}`);
  if (collected === null && total === null) parts.push('Размер выборки неизвестен');
  if (numerator !== null)
    parts.push(`${id === 'ACT_COMMIT_FREQ' ? 'коммитов' : 'числитель'}: ${numerator}`);
  if (denominator !== null)
    parts.push(`${id === 'ACT_COMMIT_FREQ' ? 'недель' : 'знаменатель'}: ${denominator}`);
  if (excluded !== null)
    parts.push(`исключено: ${excluded}${excluded_reason ? ` (${excluded_reason})` : ''}`);
  return parts.join(', ');
}

function isSafeUrl(url: string | null | undefined): url is string {
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

function EvidenceList({
  evidence,
  limited,
  total,
  reason,
}: {
  evidence: Evidence[];
  limited: boolean;
  total: number | null;
  reason: Reason | null;
}) {
  if (!evidence || evidence.length === 0) {
    return (
      <div className={styles.evidenceBlock}>
        <strong>Основания:</strong>{' '}
        <span className={styles.mono}>
          Основания недоступны
          {reason ? ` (${reason.message})` : ''}
        </span>
      </div>
    );
  }

  return (
    <div className={styles.evidenceBlock}>
      <strong>Основания:</strong>
      <ul>
        {evidence.map((ev) => (
          <li key={ev.id}>
            <span>{ev.secret_match_type ?? ev.description}</span>
            {!ev.secret_match_type && (
              <span className={styles.mono}>{` (источник: ${ev.source})`}</span>
            )}
            {ev.path && <span className={styles.mono}>{` [${ev.path}]`}</span>}
            {ev.revision && <span className={styles.mono}>{` rev: ${ev.revision}`}</span>}
            {ev.secret_match_type ? null : isSafeUrl(ev.url) ? (
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
                {ev.url && ` ${ev.url}`}
              </span>
            )}
          </li>
        ))}
      </ul>
      {limited && (
        <p className={styles.hint}>
          Показано {evidence.length} оснований; список ограничен профилем.
          {total !== null && ` Всего: ${total}.`}
        </p>
      )}
    </div>
  );
}

function MetricRow({ metric, sha }: { metric: Metric; sha: string | null }) {
  const { input } = metric;
  const label = METRIC_LABELS[input.id] ?? input.id;

  const isUnavailable =
    input.status === 'unavailable' ||
    input.status === 'not_applicable' ||
    input.raw_value === null ||
    input.raw_value === undefined;

  const displayRaw = isUnavailable ? 'N/A' : formatRawValue(input.id, input.raw_value);

  const reason =
    isUnavailable && input.reason
      ? {
          specific: input.reason.message,
          general: input.reason.cannot_conclude,
        }
      : null;

  return (
    <div className={styles.metricRow}>
      <dt>
        {label}
        <span className={styles.metricId}>{input.id}</span>
      </dt>
      <dd className={styles.metricDetails}>
        <div className={styles.metricValueLine}>
          <div>
            <span className={styles.metricCaption}>Исходное значение</span>
            <span className={styles.metricValue}>{displayRaw}</span>
            {!isUnavailable && input.id !== 'ACT_TREND' && (
              <span> {input.id === 'ACT_COMMIT_FREQ' ? 'коммитов в неделю' : 'дн.'}</span>
            )}
          </div>
          <div>
            <span className={styles.metricCaption}>Балл</span>
            <span className={styles.metricValue}>
              {metric.normalized_score === null
                ? 'Не рассчитан'
                : roundHalfUp(metric.normalized_score, 1)}
            </span>
          </div>
        </div>
        {reason && (
          <div className={styles.naReason}>
            {reason.specific}. {reason.general}
          </div>
        )}
        <div>
          <strong>Состояние:</strong>{' '}
          <span className={styles.mono}>{STATUS_LABELS[input.status] ?? input.status}</span>
        </div>
        <div>
          <strong>Окно:</strong>{' '}
          {input.window
            ? `[${formatUtc(input.window.from)}, ${formatUtc(input.window.until)})`
            : sha
              ? `По снимку ${sha}`
              : 'Не определено'}
        </div>
        {input.observed_at && (
          <div>
            <strong>
              {input.id === 'ACT_DAYS_SINCE_LAST' ? 'Последний коммит:' : 'Дата наблюдения:'}
            </strong>{' '}
            {formatUtc(input.observed_at)}
          </div>
        )}
        <div>
          <strong>Выборка:</strong> {formatSample(input.sample, input.id)}
        </div>
        {input.exclusions.length > 0 && (
          <div>
            <strong>Исключения:</strong>{' '}
            {input.exclusions.map(({ reason, count }) => `${reason}: ${count}`).join(', ')}
          </div>
        )}
        {input.series && input.series.length > 0 && (
          <div>
            <strong>Коммиты по периодам:</strong>{' '}
            <span className={styles.mono}>{input.series.join(', ')}</span>
          </div>
        )}
        <div>
          <strong>Порог:</strong> {metric.threshold ?? 'Не задан'}
        </div>
        {metric.explanation && <div>{metric.explanation}</div>}
        <div>
          <strong>Версия методики:</strong>{' '}
          <span className={styles.mono}>{metric.rule_version}</span>
        </div>
        <EvidenceList
          evidence={input.evidence}
          limited={input.evidence_limited}
          total={input.evidence_total}
          reason={input.reason}
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
          <MetricRow key={metric.input.id} metric={metric} sha={report.repository.commit_sha} />
        ))}
      </dl>
    </section>
  );
}
