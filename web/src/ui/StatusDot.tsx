import styles from './StatusDot.module.css';

export type StatusDotState = 'pending' | 'active' | 'success' | 'warning' | 'danger';

interface StatusDotProps {
  state: StatusDotState;
}

/** Decorative status indicator; the accompanying text carries the meaning. */
export function StatusDot({ state }: StatusDotProps) {
  const stateClass = state === 'pending' ? '' : styles[state];
  return <span aria-hidden="true" className={`${styles.dot} ${stateClass}`} />;
}
