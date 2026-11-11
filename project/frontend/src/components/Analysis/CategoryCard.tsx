import type { Category, CategoryId } from '@/types';
import { formatCoverage, roundHalfUp, STATUS_LABELS } from '@/lib/metricFormatting';
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
}: {
  category: Category;
  sha?: string | null;
  title?: string;
  defaultOpen?: boolean;
}) {
  const label = title ?? CATEGORY_LABELS[category.id] ?? category.id;
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
          <span className={styles.mono}>{STATUS_LABELS[category.status] ?? category.status}</span>
        </div>
        <div>
          <strong>Покрытие:</strong> {formatCoverage(category.coverage)}
        </div>
      </div>

      {category.reason && (
        <p className={styles.naReason} role="status">
          {category.reason.message}. {category.reason.cannot_conclude}
        </p>
      )}

      {category.explanation && <p className={styles.hint}>{category.explanation}</p>}

      {category.metrics.length > 0 ? (
        <details className={styles.categoryDetails} open={defaultOpen}>
          <summary className={styles.categorySummary}>
            Показатели ({category.metrics.length})
          </summary>
          <dl className={styles.fields}>
            {category.metrics.map((metric) => (
              <MetricDetails key={metric.input.id} metric={metric} sha={sha} />
            ))}
          </dl>
        </details>
      ) : (
        <p className={styles.hint}>Показатели отсутствуют</p>
      )}
    </section>
  );
}
