import type { Metric } from '@/types';
import { formatUtc } from '@/lib/analysisView';
import {
  formatRawValue,
  formatSample,
  METRIC_LABELS,
  roundHalfUp,
  STATUS_LABELS,
} from '@/lib/metricFormatting';
import { EvidenceList } from './EvidenceList';
import styles from './Analysis.module.css';

export function MetricDetails({ metric, sha }: { metric: Metric; sha?: string | null }) {
  const { input } = metric;
  const label = METRIC_LABELS[input.id] ?? input.id;
  const unit = input.unit;

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
            {!isUnavailable && unit && <span> {unit}</span>}
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