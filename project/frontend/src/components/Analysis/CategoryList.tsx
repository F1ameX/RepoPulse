import type { Report } from '@/types';
import { CategoryCard } from './CategoryCard';
import styles from './Analysis.module.css';

export function CategoryList({ report }: { report: Report }) {
  if (report.categories.length === 0) return null;
  return (
    <section aria-label="Категории отчёта" className={styles.categoryList}>
      <h2 className={styles.sectionTitle}>Категории</h2>
      {report.categories.map((category) => (
        <CategoryCard
          key={category.id}
          category={category}
          sha={report.repository.commit_sha}
        />
      ))}
    </section>
  );
}