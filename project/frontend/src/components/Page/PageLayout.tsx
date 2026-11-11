import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { cn } from '@/lib/utils';
import { useAuth } from '@/components/Auth/AuthContext';
import { AuthNavigation, LogoutFeedback } from '@/components/Auth/AuthNavigation';
import styles from './Page.module.css';

export function PageLayout({
  children,
  centered = false,
}: {
  children: ReactNode;
  centered?: boolean;
}) {
  const auth = useAuth();
  const logoutState = auth?.logoutState ?? 'idle';
  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <Link to="/" className={styles.logo}>
          RepoPulse
        </Link>
        {auth && (
          <nav aria-label="Аккаунт" className={styles.navigation}>
            <AuthNavigation />
          </nav>
        )}
      </header>
      <main className={cn(styles.content, centered && styles.centeredContent)}>
        {logoutState === 'idle' ? children : <LogoutFeedback pending={logoutState === 'pending'} />}
      </main>
    </div>
  );
}
