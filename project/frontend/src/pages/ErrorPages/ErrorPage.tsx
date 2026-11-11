import { ButtonLink } from '@/components/Button/ButtonLink';
import { PageLayout } from '@/components/Page/PageLayout';
import styles from '@/components/Page/Page.module.css';

export default function ErrorPage({
  title = 'Не удалось открыть страницу',
  action = 'К проверке репозитория',
}: { title?: string; action?: string } = {}) {
  return (
    <PageLayout>
      <div role="alert" className={styles.recovery}>
        <h1 className={styles.title}>{title}</h1>
        <ButtonLink to="/">{action}</ButtonLink>
      </div>
    </PageLayout>
  );
}
