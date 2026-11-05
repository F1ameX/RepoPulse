import { Link } from 'react-router-dom';
import { PageLayout } from '@/components/Page/PageLayout';
import styles from '@/components/Page/Page.module.css';

export default function ErrorPage() {
  return (
    <PageLayout>
      <div role="alert" className={styles.recovery}>
        <h1 className={styles.title}>Не удалось открыть страницу</h1>
        <Link to="/" className={styles.actionLink}>
          К проверке репозитория
        </Link>
      </div>
    </PageLayout>
  );
}
