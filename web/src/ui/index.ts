// Public surface of the shared UI primitives, used by viewer and admin pages alike (C-2).
// Features import from '@/ui' only (ESLint rule).
export { AppHeader, type NavItem } from './AppHeader';
export { Button, ButtonLink, SpatialButton, SpatialButtonLink, type ButtonVariant } from './Button';
export { Chip } from './Chip';
export { ConfirmDialog } from './Dialog';
export { HeroCarousel, HERO_PLAY_KEY, type HeroSlide } from './HeroCarousel';
export { Icon, type IconName } from './Icon';
export { Logo } from './Logo';
export { Notice } from './Notice';
export { Page } from './Page';
export { Panel } from './Panel';
export { PosterCard } from './PosterCard';
export { PosterFallback } from './PosterFallback';
export { ProgressBar } from './ProgressBar';
export { RadioGroup } from './RadioGroup';
export { Row } from './Row';
export { Skeleton } from './Skeleton';
export { initSpatialNavigation, SpatialGroup, SpatialPage } from './spatial';
export { Stack } from './Stack';
export { StatusDot, type StatusDotState } from './StatusDot';
export { Text } from './Text';
export { TextField } from './TextField';
export { TextLink } from './TextLink';
export { VisuallyHidden } from './VisuallyHidden';
