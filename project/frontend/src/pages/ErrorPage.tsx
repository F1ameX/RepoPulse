import { Link } from 'react-router-dom';
import { AnalysisLayout } from '@/components/Analysis/AnalysisLayout';
import styles from '@/components/Analysis/Analysis.module.css';

export default function ErrorPage() {
  return (
    <AnalysisLayout>
      <div role="alert" className={styles.error}>
        <h1 className={styles.title}>Не удалось открыть страницу</h1>
        <Link to="/" className={styles.newAnalysis}>
          К проверке репозитория
        </Link>
      </div>
    </AnalysisLayout>
  );
}
