import styles from './Skeleton.module.css';

interface SkeletonProps {
  shape?: 'poster' | 'line' | 'title' | 'hero' | 'block';
}

/** Placeholder with the final size of the content, so loading never shifts the layout. */
export function Skeleton({ shape = 'line' }: SkeletonProps) {
  return <span aria-hidden="true" className={`${styles.skeleton} ${styles[shape]}`} />;
}
