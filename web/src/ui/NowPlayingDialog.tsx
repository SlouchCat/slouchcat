import { Dialog, Heading, Modal, ModalOverlay } from 'react-aria-components';
import { AriaButton } from './AriaButton';
import { focusOnMount } from './focusOnMount';
import { useSpaceToggle } from './MiniPlayer';
import styles from './NowPlayingDialog.module.css';
import { StationLogo } from './StationLogo';

// «Jetzt läuft» (concept 16, 10.3): the running station in a full view over the page, opened from
// the mini player. Stage 1 has no time-shift slider: «Live» jumps to the live edge. Escape closes
// it and the focus returns to the mini player (react-aria). The texts come from the app (rule 9).

interface NowPlayingToggle {
  pressed: boolean;
  label: string;
  onPress: () => void;
}

interface NowPlayingDialogProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  /** The heading («Jetzt läuft»). */
  heading: string;
  closeLabel: string;
  stationName: string;
  logoUrl?: string;
  /** Country and region of the station, as one line beside its name. */
  place?: string;
  /** The big line: the title, or the station's programme when it sends none. */
  title: string;
  /** Under it: the artist, or why there is no title. */
  subtitle?: string;
  /** Play or pause: `pressed` = playing, the label names what a press does. */
  toggle: NowPlayingToggle;
  liveLabel: string;
  onLive: () => void;
  favorite?: NowPlayingToggle;
  /** Quality, genre, language … as label and value. */
  facts: readonly { label: string; value: string }[];
  /** Where the sound comes from («Über Slouchcat, Standort des Servers»). */
  hint: string;
}

export function NowPlayingDialog({
  isOpen,
  onOpenChange,
  heading,
  closeLabel,
  stationName,
  logoUrl,
  place,
  title,
  subtitle,
  toggle,
  liveLabel,
  onLive,
  favorite,
  facts,
  hint,
}: NowPlayingDialogProps) {
  const space = useSpaceToggle(toggle.onPress);
  return (
    <ModalOverlay
      className={styles.overlay}
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      isDismissable
    >
      <Modal className={styles.modal}>
        <Dialog className={styles.dialog} data-testid="radio-now-playing">
          <div className={styles.view} {...space}>
            <div className={styles.head}>
              <AriaButton
                variant="ghost"
                icon="chevronDown"
                iconOnly
                aria-label={closeLabel}
                ref={focusOnMount}
                onPress={() => onOpenChange(false)}
              />
              <Heading slot="title" className={styles.heading}>
                {heading}
              </Heading>
              {favorite && (
                <AriaButton
                  variant="ghost"
                  icon="heart"
                  iconOnly
                  aria-label={favorite.label}
                  aria-pressed={favorite.pressed}
                  onPress={favorite.onPress}
                />
              )}
            </div>
            <div className={styles.body}>
              <div className={styles.picture}>
                <StationLogo name={stationName} src={logoUrl} size="fill" />
              </div>
              <div className={styles.info}>
                <p className={styles.station}>
                  <StationLogo name={stationName} src={logoUrl} size="sm" />
                  <span>{stationName}</span>
                  {place && <span className={styles.place}>{place}</span>}
                </p>
                {/* The live region stays in place, so a screen reader hears the new title; only
                    its content is new, and fades in (none under prefers-reduced-motion). */}
                <div aria-live="polite">
                  <div className={styles.titles} key={`${title}\n${subtitle}`}>
                    <p className={styles.title} data-testid="radio-now-title">
                      {title}
                    </p>
                    {subtitle && <p className={styles.subtitle}>{subtitle}</p>}
                  </div>
                </div>
                <div className={styles.controls}>
                  <AriaButton
                    variant="primary"
                    size="lg"
                    icon={toggle.pressed ? 'pause' : 'play'}
                    iconOnly
                    aria-label={toggle.label}
                    onPress={toggle.onPress}
                  />
                  <AriaButton variant="secondary" onPress={onLive}>
                    {liveLabel}
                  </AriaButton>
                </div>
                {facts.length > 0 && (
                  <dl className={styles.facts}>
                    {facts.map((fact) => (
                      <div key={fact.label}>
                        <dt>{fact.label}</dt>
                        <dd>{fact.value}</dd>
                      </div>
                    ))}
                  </dl>
                )}
                <p className={styles.hint}>{hint}</p>
              </div>
            </div>
          </div>
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}
