import { useId } from 'react';
import type { InputHTMLAttributes, ReactNode, Ref } from 'react';
import { cn } from '@/lib/utils';
import styles from './Input.module.css';

export type InputSize = 'sm' | 'md' | 'lg';

export interface InputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'> {
  label?: string;
  hint?: string;
  error?: string;
  action?: ReactNode;
  size?: InputSize;
  ref?: Ref<HTMLInputElement>;
}

export function Input({
  label,
  hint,
  error,
  action,
  size = 'md',
  id,
  className,
  ref,
  'aria-describedby': externalDescription,
  ...rest
}: InputProps) {
  const autoId = useId();
  const inputId = id ?? autoId;
  const errorId = `${inputId}-error`;
  const hintId = `${inputId}-hint`;

  const describedBy =
    [externalDescription, hint && !error ? hintId : null, error ? errorId : null]
      .filter(Boolean)
      .join(' ') || undefined;

  const input = (
    <input
      ref={ref}
      id={inputId}
      aria-invalid={error ? true : undefined}
      aria-describedby={describedBy}
      className={cn(styles.input, styles[size], error && styles.hasError)}
      {...rest}
    />
  );

  return (
    <div className={cn(styles.field, className)}>
      {label && (
        <label htmlFor={inputId} className={styles.label}>
          {label}
          {rest.required && (
            <span className={styles.required} aria-hidden>
              *
            </span>
          )}
        </label>
      )}
      {action ? (
        <div className={styles.control}>
          {input}
          {action}
        </div>
      ) : (
        input
      )}
      {error && (
        <span id={errorId} className={styles.error} role="alert">
          {error}
        </span>
      )}
      {hint && !error && (
        <span id={hintId} className={styles.hint}>
          {hint}
        </span>
      )}
    </div>
  );
}
