import { Link, type LinkProps } from 'react-router-dom';
import { cn } from '@/lib/utils';
import type { ButtonSize, ButtonVariant } from './Button';
import styles from './Button.module.css';

interface ButtonLinkProps extends LinkProps {
  variant?: ButtonVariant;
  size?: ButtonSize;
}

export function ButtonLink({
  variant = 'primary',
  size = 'md',
  className,
  ...props
}: ButtonLinkProps) {
  return (
    <Link {...props} className={cn(styles.button, styles[variant], styles[size], className)} />
  );
}
