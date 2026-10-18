import type { ReactNode } from 'react';
import styles from './Text.module.css';

type Variant = 'display' | 'title' | 'body' | 'caption';
type Tone = 'primary' | 'secondary' | 'muted';

interface TextProps {
  children: ReactNode;
  variant?: Variant;
  tone?: Tone;
  as?: 'p' | 'span' | 'h1' | 'h2' | 'h3';
}

/** Typography primitive; sizes, weights and colours come from the design tokens. */
export function Text({ children, variant = 'body', tone = 'primary', as: Tag = 'p' }: TextProps) {
  return <Tag className={`${styles.text} ${styles[variant]} ${styles[tone]}`}>{children}</Tag>;
}
