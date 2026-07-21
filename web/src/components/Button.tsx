import type { ComponentPropsWithoutRef } from 'react';

export interface ButtonProps extends ComponentPropsWithoutRef<'button'> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg';
}

export function Button({ variant = 'secondary', size = 'md', className, type = 'button', ...props }: ButtonProps) {
  const classes = `btn btn-${variant}${size === 'md' ? '' : ` btn-${size}`}${className ? ` ${className}` : ''}`;
  return <button {...props} type={type} className={classes} />;
}
