import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { cn } from '@/lib/utils';
import styles from './Page.module.css';

export function PageLayout({
  children,
  centered = false,
}: {
  children: ReactNode;
  centered?: boolean;
}) {
  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <Link to="/" className={styles.logo}>
          RepoPulse
        </Link>
      </header>
      <main className={cn(styles.content, centered && styles.centeredContent)}>{children}</main>
    </div>
  );
}
