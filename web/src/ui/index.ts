// Public surface of the shared UI primitives, used by viewer and admin pages alike (C-2).
// Features import from '@/ui' only (ESLint rule); react-aria based parts (forms, dialogs, the
// player's buttons) come from '@/ui/aria', so the browsing pages never load react-aria (P-10).
export { isAccentName, itemAccentStyle, swatchStyle } from './accent';
export { AppHeader, type NavItem } from './AppHeader';
export { Backdrop } from './Backdrop';
export { LinkRouterProvider } from './AppLink';
export { Button, ButtonLink, SpatialButton, SpatialButtonLink, type ButtonVariant } from './Button';
export { CandidateCard } from './CandidateCard';
export { CastCard } from './CastCard';
export { Chip } from './Chip';
export { ChoiceChips } from './ChoiceChips';
export { EpisodeCard } from './EpisodeCard';
export { EpisodeRow } from './EpisodeRow';
export { FactList } from './FactList';
export { dismissHeroPrepaint, HeroCarousel, HERO_PLAY_KEY, type HeroSlide } from './HeroCarousel';
export { Icon, type IconName } from './Icon';
export { Logo } from './Logo';
export { Notice } from './Notice';
export { Page } from './Page';
export { Panel } from './Panel';
export { PosterCard } from './PosterCard';
export { PosterFallback } from './PosterFallback';
export { ProfileAvatar, ProfileTile } from './ProfileTile';
export { ProgressBar } from './ProgressBar';
export { Row } from './Row';
export { Skeleton } from './Skeleton';
export {
  loadSpatialNavigation,
  setSpatialFocus,
  SpatialGroup,
  SpatialPage,
  spatialFocusKey,
} from './spatial';
export { Stack } from './Stack';
export { StatusDot, type StatusDotState } from './StatusDot';
export { Text } from './Text';
export { TextLink } from './TextLink';
export { VisuallyHidden } from './VisuallyHidden';
