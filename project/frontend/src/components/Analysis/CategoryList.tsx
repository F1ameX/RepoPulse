import { Button, Message } from '@/components';
import { isKnownStatus, METRIC_LABELS } from '@/lib/metricFormatting';
import type { Category, CategoryId, MetricId, Report } from '@/types';
import { CategoryCard } from './CategoryCard';
import { ReportLimitations } from './ReportLimitations';
import styles from './Analysis.module.css';

const CATEGORIES: { id: CategoryId; title: string; metrics: MetricId[] }[] = [
  {
    id: 'activity',
    title: 'Активность и регулярность',
    metrics: ['ACT_COMMIT_FREQ', 'ACT_DAYS_SINCE_LAST', 'ACT_TREND'],
  },
  { id: 'documentation', title: 'Документация', metrics: ['DOC_README', 'DOC_SUPPORT_SHARE'] },
  { id: 'testing', title: 'Тестирование', metrics: ['TEST_PRESENCE'] },
  { id: 'security', title: 'Безопасность', metrics: ['SEC_SECRETS', 'SEC_VULN_RISK', 'SEC_SAST'] },
  { id: 'ci_cd', title: 'CI/CD', metrics: ['CI_PRESENT', 'CI_SUCCESS_RATE', 'CI_REQUIRED_CHECKS'] },
];

function orderCategory(category: Category | undefined, ids: MetricId[]): Category | null {
  if (
    !category ||
    !isKnownStatus(category.status) ||
    !Array.isArray(category.metrics) ||
    (category.score !== null && !Number.isFinite(category.score)) ||
    category.coverage?.total !== ids.length ||
    !Number.isInteger(category.coverage.calculated)
  ) {
    return null;
  }

  const metrics = ids.map((id) => category.metrics.find((metric) => metric?.input?.id === id));
  if (
    metrics.some(
      (metric) =>
        !metric ||
        !isKnownStatus(metric.input.status) ||
        !metric.input.sample ||
        !Array.isArray(metric.input.exclusions) ||
        !Array.isArray(metric.input.evidence) ||
        !('raw_value' in metric.input) ||
        (metric.normalized_score !== null && !Number.isFinite(metric.normalized_score)),
    )
  )
    return null;

  return { ...category, metrics: metrics.filter((metric) => metric !== undefined) };
}

export function CategoryList({ report, onReload }: { report: Report; onReload?: () => void }) {
  const reported = Array.isArray(report.categories) ? report.categories : [];
  const categories = CATEGORIES.map(({ id, title, metrics }) => ({
    id,
    title,
    category: orderCategory(
      reported.find((category) => category?.id === id),
      metrics,
    ),
  }));
  const targets = categories.flatMap(({ title, category }) =>
    (category?.metrics ?? []).map((metric) => ({
      category: title,
      label: METRIC_LABELS[metric.input.id],
      input: metric.input,
    })),
  );

  return (
    <>
      <section aria-label="Категории отчёта" className={styles.categoryList}>
        <h2 className={styles.sectionTitle}>Категории</h2>
        {categories.map(({ id, title, category }) =>
          category ? (
            <CategoryCard
              key={id}
              title={title}
              category={category}
              sha={report.repository.commit_sha}
              reportDetails
            />
          ) : (
            <section key={id} className={styles.categoryCard} aria-label={`Категория: ${title}`}>
              <header className={styles.categoryHeader}>
                <h3 className={styles.categoryTitle}>{title}</h3>
              </header>
              <Message
                variant="error"
                actions={
                  onReload && (
                    <Button variant="secondary" onClick={onReload}>
                      Повторить загрузку
                    </Button>
                  )
                }
              >
                Не удалось загрузить категорию
              </Message>
            </section>
          ),
        )}
      </section>
      <ReportLimitations
        targets={targets}
        sources={report.sources}
        coverage={report.coverage}
        incomplete={categories.some(({ category }) => !category)}
      />
    </>
  );
}
