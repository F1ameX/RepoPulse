import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';
import styles from './Message.module.css';

export type MessageVariant = 'info' | 'success' | 'warning' | 'error';

export interface MessageProps {
  variant?: MessageVariant;
  children: ReactNode;
  onDismiss?: () => void;
  className?: string;
  icon?: ReactNode | false;
  actions?: ReactNode;
}

const icons: Record<MessageVariant, ReactNode> = {
  info: (
    <svg
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      aria-hidden
      className={styles.icon}
    >
      <circle cx="8" cy="8" r="6.5" />
      <path d="M8 7v4M8 5v.01" />
    </svg>
  ),
  success: (
    <svg
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
      className={styles.icon}
    >
      <circle cx="8" cy="8" r="6.5" />
      <path d="M5 8.2l2 2 4-4" />
    </svg>
  ),
  warning: (
    <svg
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
      className={styles.icon}
    >
      <path d="M8 1.8L14.7 13.5H1.3L8 1.8z" />
      <path d="M8 6.2v3M8 11.2v.01" />
    </svg>
  ),
  error: (
    <svg
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      aria-hidden
      className={styles.icon}
    >
      <circle cx="8" cy="8" r="6.5" />
      <path d="M8 4.5v4M8 11v.01" />
    </svg>
  ),
};

export function Message({
  variant = 'info',
  children,
  onDismiss,
  className,
  icon,
  actions,
}: MessageProps) {
  const isUrgent = variant === 'error' || variant === 'warning';

  const renderedIcon = icon === false ? null : (icon ?? icons[variant]);

  return (
    <div
      role={isUrgent ? 'alert' : 'status'}
      className={cn(styles.message, styles[variant], className)}
    >
      {renderedIcon}
      <span className={styles.text}>{children}</span>
      {actions && <span className={styles.actions}>{actions}</span>}
      {onDismiss && (
        <button
          type="button"
          className={styles.close}
          onClick={onDismiss}
          aria-label="Закрыть сообщение"
        >
          ×
        </button>
      )}
    </div>
  );
}
