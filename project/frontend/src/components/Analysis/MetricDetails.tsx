import type { Metric } from '@/types';
import { formatUtc } from '@/lib/analysisView';
import {
  formatRawValue,
  formatSample,
  isKnownStatus,
  METRIC_LABELS,
  roundHalfUp,
  STATUS_LABELS,
} from '@/lib/metricFormatting';
import { EvidenceList } from './EvidenceList';
import styles from './Analysis.module.css';

export function MetricDetails({
  metric,
  sha,
  label,
  anchor,
  sharedExplanationId,
}: {
  metric: Metric;
  sha?: string | null;
  label?: string;
  anchor?: string;
  sharedExplanationId?: string;
}) {
  const { input } = metric;
  const title = label ?? METRIC_LABELS[input.id] ?? input.id;
  const unit =
    input.unit === 'commits_per_week'
      ? 'коммитов в неделю'
      : input.unit === 'days'
        ? 'дн.'
        : ['bool', 'ratio'].includes(input.unit)
          ? ''
          : input.unit;

  if (!isKnownStatus(input.status)) {
    return (
      <div className={styles.metricRow}>
        <dt>{title}</dt>
        <dd className={styles.metricDetails}>
          <div className={styles.naReason} role="alert">
            Не удалось отобразить метрику: отсутствует состояние полноты данных.
          </div>
        </dd>
      </div>
    );
  }

  const isUnavailable =
    input.status === 'unavailable' ||
    input.status === 'not_applicable' ||
    (input.id !== 'SEC_SECRETS' &&
      (input.raw_value === null || input.raw_value === undefined));

  const secretScanResult =
    input.status === 'unavailable' || input.status === 'not_applicable'
      ? 'N/A'
      : input.evidence.length > 0 ||
          (input.evidence_total !== null && input.evidence_total > 0) ||
          input.raw_value === true
        ? 'Обнаружены потенциальные совпадения'
        : (input.evidence_total === 0 || input.raw_value === false) &&
            input.status === 'complete'
          ? 'Совпадений в проверенном охвате не обнаружено'
          : (input.evidence_total === 0 || input.raw_value === false) &&
              input.status === 'partial'
            ? 'В проверенной части совпадений не обнаружено'
            : input.raw_value !== null
              ? 'Результат получен; значение не отображается'
              : 'Результат проверки не представлен';

  const displayRaw =
    input.id === 'SEC_SECRETS'
      ? secretScanResult
      : input.id === 'SEC_SAST' && !isUnavailable && typeof input.raw_value === 'boolean'
        ? input.raw_value
          ? 'Конфигурация найдена'
          : input.status === 'complete'
            ? 'Конфигурация не найдена'
            : 'В проверенной области конфигурация не обнаружена'
        : isUnavailable
          ? 'N/A'
          : formatRawValue(input.id, input.raw_value);
  const showUnit = !isUnavailable && !!unit && !displayRaw.endsWith('%');

  const reason = input.reason
    ? { specific: input.reason.message, general: input.reason.cannot_conclude }
    : null;

  const evidenceLimit = input.id === 'TEST_PRESENCE' ? 10 : undefined;

  return (
    <div
      className={styles.metricRow}
      id={anchor}
      tabIndex={anchor ? -1 : undefined}
      aria-describedby={sharedExplanationId}
    >
      <dt>{title}</dt>
      <dd className={styles.metricDetails}>
        <div className={styles.metricValueLine}>
          <div>
            <span className={styles.metricCaption}>
              {input.id === 'SEC_VULN_RISK'
                ? 'Risk'
                : input.id === 'SEC_SAST'
                  ? 'Конфигурация SAST'
                  : input.id === 'SEC_SECRETS'
                    ? 'Результат сканирования'
                    : 'Исходное значение'}
            </span>
            <span className={styles.metricValue}>{displayRaw}</span>
            {showUnit && <span> {unit}</span>}
          </div>
          <div>
            <span className={styles.metricCaption}>Балл</span>
            <span className={styles.metricValue}>
              {metric.normalized_score === null
                ? input.status === 'complete' &&
                  input.raw_value === null &&
                  input.sample.denominator === 0
                  ? 'Не рассчитан: нет объектов для расчёта'
                  : 'Не рассчитан'
                : roundHalfUp(metric.normalized_score, 1)}
            </span>
          </div>
        </div>

        {input.id === 'SEC_SECRETS' && input.status !== 'complete' && (
          <div className={styles.hint} style={{ marginBottom: '8px' }}>
            Неполное сканирование истории не подтверждает отсутствие секретов в непроверенных ревизиях.
          </div>
        )}

        {input.id === 'SEC_SAST' && (
          <div className={styles.hint} style={{ marginBottom: '8px' }}>
            Наличие конфигурации не подтверждает успешный запуск SAST.
          </div>
        )}

        {input.id === 'SEC_VULN_RISK' && input.raw_value === null && (
          <div className={styles.hint} style={{ marginBottom: '8px' }}>
            Risk не рассчитан: недостаточно данных для оценки; неизвестная версия или уровень OSV не означают отсутствие уязвимости.
          </div>
        )}

        {input.id === 'CI_SUCCESS_RATE' && !isUnavailable && (
          <div className={styles.hint} style={{ marginBottom: '8px' }}>
            Доля успешных CI-запусков (runs), а не отдельных тестов.
          </div>
        )}

        {reason && (
          <div className={styles.naReason}>
            {reason.specific}. {reason.general}
          </div>
        )}
        <div>
          <strong>Состояние:</strong> <span>{STATUS_LABELS[input.status]}</span>
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
            <strong>Коммиты по периодам:</strong> <span>{input.series.join(', ')}</span>
          </div>
        )}
        <div>
          <strong>Порог:</strong> {metric.threshold ?? 'Не задан'}
        </div>
        {!sharedExplanationId && metric.explanation && <div>{metric.explanation}</div>}
        <div>
          <strong>Версия методики:</strong> <span>{metric.rule_version}</span>
        </div>
        <EvidenceList
          evidence={input.evidence}
          limited={input.evidence_limited}
          total={input.evidence_total}
          reason={input.reason}
          limit={evidenceLimit}
          secret={input.id === 'SEC_SECRETS'}
        />
      </dd>
    </div>
  );
}