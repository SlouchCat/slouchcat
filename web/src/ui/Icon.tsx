import styles from './Icon.module.css';

// Line icons on a 24 px grid, drawn with currentColor so they take the text colour of their
// context. Decorative only: the surrounding control carries the accessible name.
const paths = {
  play: <path d="M8 5.5v13a1 1 0 0 0 1.5.86l10.4-6.5a1 1 0 0 0 0-1.72L9.5 4.64A1 1 0 0 0 8 5.5Z" />,
  pause: (
    <>
      <rect x="6.5" y="5" width="3.5" height="14" rx="1" />
      <rect x="14" y="5" width="3.5" height="14" rx="1" />
    </>
  ),
  info: (
    <>
      <circle cx="12" cy="12" r="9" fill="none" />
      <path d="M12 11v5.5M12 7.6v.1" fill="none" />
    </>
  ),
  back: <path d="M15 5l-7 7 7 7" fill="none" />,
  chevronLeft: <path d="M14.5 6l-6 6 6 6" fill="none" />,
  chevronRight: <path d="M9.5 6l6 6-6 6" fill="none" />,
  fullscreen: <path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5" fill="none" />,
  fullscreenExit: <path d="M9 4v5H4M15 4v5h5M9 20v-5H4M15 20v-5h5" fill="none" />,
  volume: (
    <>
      <path d="M4 9.5v5h3.5L12 19V5L7.5 9.5H4Z" />
      <path d="M15.5 9a4 4 0 0 1 0 6M18 6.5a7.5 7.5 0 0 1 0 11" fill="none" />
    </>
  ),
  volumeMute: (
    <>
      <path d="M4 9.5v5h3.5L12 19V5L7.5 9.5H4Z" />
      <path d="M16 9.5l5 5M21 9.5l-5 5" fill="none" />
    </>
  ),
  logout: <path d="M14 4h4a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-4M10 16l-4-4 4-4M6 12h10" fill="none" />,
  refresh: <path d="M20 11a8 8 0 1 0-2.3 5.7M20 5v6h-6" fill="none" />,
  plus: <path d="M12 5v14M5 12h14" fill="none" />,
  warning: (
    <>
      <path d="M12 4 2.8 19.5h18.4L12 4Z" fill="none" />
      <path d="M12 10v4.5M12 17v.1" fill="none" />
    </>
  ),
  folder: (
    <path
      d="M3.5 7.5a2 2 0 0 1 2-2h4l2 2.5h7a2 2 0 0 1 2 2v7.5a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2V7.5Z"
      fill="none"
    />
  ),
  lock: (
    <>
      <rect x="5" y="10.5" width="14" height="9.5" rx="2" fill="none" />
      <path d="M8 10.5V8a4 4 0 0 1 8 0v2.5" fill="none" />
    </>
  ),
  check: <path d="M5 12.5l4.5 4.5L19 7.5" fill="none" />,
  copy: (
    <>
      <rect x="9" y="9" width="11" height="11" rx="2" fill="none" />
      <path d="M15 9V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v7a2 2 0 0 0 2 2h3" fill="none" />
    </>
  ),
  external: (
    <path
      d="M14 4h6v6M20 4l-9 9M18 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4"
      fill="none"
    />
  ),
} as const;

export type IconName = keyof typeof paths;

interface IconProps {
  name: IconName;
  size?: 'sm' | 'md' | 'lg';
}

export function Icon({ name, size = 'md' }: IconProps) {
  return (
    <svg
      className={`${styles.icon} ${styles[size]}`}
      viewBox="0 0 24 24"
      aria-hidden="true"
      focusable="false"
    >
      {paths[name]}
    </svg>
  );
}
