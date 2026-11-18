import { useId } from 'react';
import type { Category, CategoryId } from '@/types';
import { formatCoverage, isKnownStatus, roundHalfUp, STATUS_LABELS } from '@/lib/metricFormatting';
import { MetricDetails } from './MetricDetails';
import styles from './Analysis.module.css';

const CATEGORY_LABELS: Partial<Record<CategoryId, string>> = {
  activity: 'Активность',
  documentation: 'Документация',
  testing: 'Тестирование',
  security: 'Безопасность',
  ci_cd: 'CI/CD',
};

function emptyScoreLabel(category: Category): string {
  if (
    category.metrics.length > 0 &&
    category.metrics.every((metric) => metric.input.status === 'not_applicable')
  ) {
    return 'Неприменимо';
  }
  return 'Нет данных';
}

export function CategoryCard({
  category,
  sha,
  title,
  defaultOpen = false,
  reportDetails = false,
}: {
  category: Category;
  sha?: string | null;
  title?: string;
  defaultOpen?: boolean;
  reportDetails?: boolean;
}) {
  const label = title ?? CATEGORY_LABELS[category.id] ?? category.id;
  const explanationId = useId();
  const explanation = category.explanation?.trim();

  if (!isKnownStatus(category.status)) {
    return (
      <section className={styles.categoryCard} aria-label={`Категория: ${label}`}>
        <header className={styles.categoryHeader}>
          <h3 className={styles.categoryTitle}>{label}</h3>
        </header>
        <p className={styles.naReason} role="alert">
          Не удалось отобразить категорию: отсутствует состояние полноты данных.
        </p>
      </section>
    );
  }

  const hasScore = category.score !== null;
  const scoreText = hasScore ? roundHalfUp(category.score!, 1) : emptyScoreLabel(category);

  return (
    <section className={styles.categoryCard} aria-label={`Категория: ${label}`}>
      <header className={styles.categoryHeader}>
        <h3 className={styles.categoryTitle}>{label}</h3>
        <div className={styles.categoryScore}>
          <span className={styles.metricCaption}>Балл</span>
          <span className={styles.metricValue}>{scoreText}</span>
        </div>
      </header>

      <div className={styles.categoryMeta}>
        <div>
          <strong>Состояние:</strong>{' '}
          <span>
            {hasScore && category.coverage.calculated < category.coverage.total
              ? STATUS_LABELS.partial
              : STATUS_LABELS[category.status]}
          </span>
        </div>
        <div>
          {reportDetails ? (
            `Рассчитано ${category.coverage.calculated} из ${category.coverage.total} метрик`
          ) : (
            <>
              <strong>Покрытие:</strong> {formatCoverage(category.coverage)}
            </>
          )}
        </div>
      </div>

      {category.reason && (
        <p className={styles.naReason} role="status">
          {category.reason.message}. {category.reason.cannot_conclude}
        </p>
      )}

      {category.explanation && (
        <p id={explanationId} className={styles.hint}>
          {category.explanation}
        </p>
      )}
      {reportDetails && !hasScore && !category.reason && (
        <p className={styles.naReason}>Причина недоступности не определена</p>
      )}

      {category.metrics.length > 0 ? (
        <details className={styles.categoryDetails} open={defaultOpen}>
          <summary className={styles.categorySummary}>
            {reportDetails ? 'Показать основания' : `Показатели (${category.metrics.length})`}
          </summary>
          <dl className={styles.fields}>
            {category.metrics.map((metric) => (
              <MetricDetails
                key={metric.input.id}
                metric={metric}
                sha={sha}
                anchor={reportDetails ? `metric-${metric.input.id}` : undefined}
                sharedExplanationId={
                  metric.input.id === 'TEST_PRESENCE' &&
                  explanation &&
                  explanation === metric.explanation?.trim()
                    ? explanationId
                    : undefined
                }
              />
            ))}
          </dl>
        </details>
      ) : (
        <p className={styles.hint}>Показатели отсутствуют</p>
      )}
    </section>
  );
}
