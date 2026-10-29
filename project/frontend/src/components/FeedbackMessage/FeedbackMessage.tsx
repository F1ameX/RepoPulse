import type { ReactNode } from 'react';
import { Message } from '@/components/Message/Message';
import styles from './FeedbackMessage.module.css';

export type FeedbackVariant = 'info' | 'success' | 'warning' | 'error';

export interface FeedbackView {
  variant: FeedbackVariant;
  message: ReactNode;
  hint?: ReactNode;
  actions?: ReactNode;
  icon?: ReactNode | false;
}

export interface FeedbackMessageProps {
  view: FeedbackView | null;
  className?: string;
}

export function FeedbackMessage({ view, className }: FeedbackMessageProps) {
  if (!view) return null;

  return (
    <Message
      variant={view.variant}
      icon={view.icon}
      actions={view.actions}
      className={className}
    >
      <span className={styles.body}>
        {view.message}
        {view.hint && <span className={styles.hint}>{view.hint}</span>}
      </span>
    </Message>
  );
}