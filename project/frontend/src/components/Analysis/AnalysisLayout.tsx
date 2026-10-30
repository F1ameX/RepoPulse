import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import styles from './Analysis.module.css';

export function AnalysisLayout({ children }: { children: ReactNode }) {
  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <Link to="/" className={styles.logo}>
          RepoPulse
        </Link>
      </header>
      <main className={styles.content}>{children}</main>
    </div>
  );
}
export function AnalysisTitle({ repository }: { repository?: { owner: string; name: string } }) {
  return (
    <div className={styles.titleBlock}>
      <h1 className={styles.title}>
        {repository ? (
          <>
            {repository.owner}/<wbr />
            {repository.name}
          </>
        ) : (
          'Анализ репозитория'
        )}
      </h1>
    </div>
  );
}
