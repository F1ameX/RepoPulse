import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import styles from './Analysis.module.css';

export function AnalysisLayout({ children }: { children: ReactNode }) {
  return (
    <div className={styles.page}>
      <header className="appHeader">
        <Link to="/" className="appLogo">
          RepoPulse
        </Link>
      </header>
      <main className={styles.content}>{children}</main>
    </div>
  );
}
export function AnalysisTitle({ repository }: { repository?: { owner: string; name: string } }) {
  return (
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
  );
}
