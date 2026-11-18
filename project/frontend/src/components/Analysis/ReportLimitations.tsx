import type { MouseEvent } from 'react';
import { isKnownStatus, roundHalfUp, STATUS_LABELS } from '@/lib/metricFormatting';
import type { Coverage, DataStatus, MetricInput, Reason, Sample, SourceState } from '@/types';
import styles from './Analysis.module.css';

type Target = { category: string; label: string; input: MetricInput };

function coverageText({ collected, total }: Sample): string {
  if (collected == null)
    return total == null
      ? 'Размер выборки неизвестен'
      : `Всего ${total}; полученный объём неизвестен`;
  if (total == null) return `Получено ${collected}; полный объём неизвестен`;
  return `Получено ${collected} из ${total}${total > 0 ? ` (${roundHalfUp((collected / total) * 100, 1)}%)` : ''}`;
}

function LimitationDetails({
  status,
  reason,
  sample,
}: {
  status: DataStatus;
  reason: Reason | null;
  sample: Sample;
}) {
  return (
    <>
      <div>{STATUS_LABELS[status]}</div>
      <div>{reason?.message || 'Причина недоступности не определена'}</div>
      <div>{coverageText(sample)}</div>
      {reason?.cannot_conclude && <div>{reason.cannot_conclude}</div>}
    </>
  );
}

function MetricLink({ target }: { target: Target }) {
  const anchor = `metric-${target.input.id}`;
  const reveal = (event: MouseEvent<HTMLAnchorElement>) => {
    const metric = document.getElementById(anchor);
    const details = metric?.closest('details');
    if (!metric || !details) return;
    event.preventDefault();
    details.open = true;
    metric.focus({ preventScroll: true });
    metric.scrollIntoView?.({ block: 'start' });
  };
  return (
    <a href={`#${anchor}`} onClick={reveal}>
      {target.category} · {target.label}
    </a>
  );
}

export function ReportLimitations({
  targets,
  sources,
  coverage,
  incomplete,
}: {
  targets: Target[];
  sources: SourceState[] | undefined;
  coverage: Coverage | undefined;
  incomplete: boolean;
}) {
  const sourceEntries = (Array.isArray(sources) ? sources : [])
    .filter(
      (source) =>
        source && source.sample && isKnownStatus(source.status) && source.status !== 'complete',
    )
    .map((source) => ({
      source,
      targets: targets.filter(({ input }) => source.metric_ids?.includes(input.id)),
    }));
  const metricEntries = targets.filter(
    ({ input }) =>
      (input.status !== 'complete' || input.reason) &&
      !sourceEntries.some(
        ({ source }) =>
          source.metric_ids?.includes(input.id) &&
          source.reason?.code === input.reason?.code &&
          source.reason?.message === input.reason?.message &&
          source.reason?.cannot_conclude === input.reason?.cannot_conclude &&
          source.status === input.status,
      ),
  );
  const invalidSources =
    !Array.isArray(sources) ||
    sources.some((source) => !source?.sample || !isKnownStatus(source.status));
  const entries = [
    ...metricEntries.map((target) => ({
      order: targets.indexOf(target),
      node: (
        <li key={target.input.id}>
          <MetricLink target={target} />
          <LimitationDetails {...target.input} />
        </li>
      ),
    })),
    ...sourceEntries.map(({ source, targets: affected }) => ({
      order: affected.length ? targets.indexOf(affected[0]) : targets.length,
      node: (
        <li key={`${source.source}/${source.scope}`}>
          <strong>
            Источник: {source.source} · {source.scope}
          </strong>
          <LimitationDetails {...source} />
          {affected.length > 0 && (
            <ul>
              {affected.map((target) => (
                <li key={target.input.id}>
                  <MetricLink target={target} />
                </li>
              ))}
            </ul>
          )}
        </li>
      ),
    })),
  ].sort((a, b) => a.order - b.order);

  return (
    <section className={styles.categoryList} aria-label="Ограничения анализа">
      <h2 className={styles.sectionTitle}>Ограничения анализа</h2>
      {coverage &&
        [coverage.partial, coverage.unavailable, coverage.not_applicable].every(
          Number.isInteger,
        ) && (
          <p>
            Частичные данные: {coverage.partial} · Недоступно: {coverage.unavailable} · Неприменимо:{' '}
            {coverage.not_applicable}
          </p>
        )}
      {(incomplete || invalidSources) && (
        <p role="alert">Не удалось загрузить все сведения об ограничениях</p>
      )}
      {!incomplete && !invalidSources && !metricEntries.length && !sourceEntries.length && (
        <p>Ограничения по полученным данным не выявлены</p>
      )}
      {entries.length > 0 && (
        <ul className={styles.limitations}>{entries.map(({ node }) => node)}</ul>
      )}
    </section>
  );
}
