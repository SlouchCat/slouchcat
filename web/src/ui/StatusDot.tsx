import styles from './StatusDot.module.css';

interface StatusDotProps {
  state: 'pending' | 'success' | 'danger';
}

/** Decorative status indicator; the accompanying text carries the meaning. */
export function StatusDot({ state }: StatusDotProps) {
  const stateClass = state === 'pending' ? '' : styles[state];
  return <span aria-hidden="true" className={`${styles.dot} ${stateClass}`} />;
}
