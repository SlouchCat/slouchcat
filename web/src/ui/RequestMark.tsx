import { Button } from './Button';

interface RequestMarkProps {
  /** Spoken and shown on hover: «Dune anfordern», «Dune: Benachrichtige mich». */
  label: string;
  /** `plus` requests the title, `bell` joins another profile's open wish (concept 25, F-7). */
  icon?: 'plus' | 'bell';
  onPress: () => void;
  /** Offline: the wishes live on the server. */
  isDisabled?: boolean;
  /** For a row with one Tab stop (a roving tabindex): the mark of a poster that is not the stop. */
  excludeFromTabOrder?: boolean;
}

/**
 * The round request button on the poster of a title outside the library (concept 25, 4.1): goes
 * into `PosterCard`'s `mark` slot, beside the card's link, after it in the Tab order and outside
 * the arrow keys, like the watchlist button. It opens the request dialog; a title the profile has
 * already wished for carries its state in the caption instead.
 */
export function RequestMark({
  label,
  icon = 'plus',
  onPress,
  isDisabled,
  excludeFromTabOrder,
}: RequestMarkProps) {
  return (
    <Button
      variant="secondary"
      size="sm"
      icon={icon}
      iconOnly
      overlay
      aria-label={label}
      title={label}
      isDisabled={isDisabled}
      excludeFromTabOrder={excludeFromTabOrder}
      onPress={onPress}
    />
  );
}
