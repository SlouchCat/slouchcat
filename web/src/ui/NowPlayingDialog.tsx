import { useState, type ComponentProps } from 'react';
import { Dialog, Heading, Modal, ModalOverlay } from 'react-aria-components';
import { AriaButton } from './AriaButton';
import { focusOnMount } from './focusOnMount';
import { useSpaceToggle } from './MiniPlayer';
import styles from './NowPlayingDialog.module.css';
import { PlayedList, type PlayedEntry } from './PlayedList';
import { StationLogo } from './StationLogo';
import { TimeShiftSlider } from './TimeShiftSlider';

// «Jetzt läuft» (concept 16, 10.3 and 8): the running station in a full view over the page,
// opened from the mini player: the title's cover large and blurred behind the view, the time shift
// slider from the oldest position to live (absent while the server keeps no buffer), «Live» and
// «Zuvor gespielt». A new title fades in, without motion under prefers-reduced-motion. Escape
// closes it and the focus returns to the mini player (react-aria). Texts from the app (rule 9).

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
  /** The cover of the title that plays (w512); the logo shows while it has none or it fails. */
  coverUrl?: string;
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
  /** Plays at live: «Live» shows pressed. */
  isLive?: boolean;
  /** The time shift slider; absent without a window (the server keeps no buffer, D-9). */
  timeShift?: ComponentProps<typeof TimeShiftSlider>;
  /** A note over the controls (the pause lasted longer than the window). */
  notice?: string;
  /** «Zuvor gespielt»; absent where the station sends no titles. */
  played?: { heading: string; empty: string; entries: readonly PlayedEntry[] };
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
  coverUrl,
  place,
  title,
  subtitle,
  toggle,
  liveLabel,
  onLive,
  isLive,
  timeShift,
  notice,
  played,
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
        {coverUrl && <Blurred key={coverUrl} src={coverUrl} />}
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
              {/* Keyed by the picture: a new cover fades in over the old place. */}
              <div className={styles.picture} key={coverUrl ?? logoUrl ?? ''}>
                <StationLogo
                  name={stationName}
                  src={coverUrl ?? logoUrl}
                  fallbackSrc={coverUrl ? logoUrl : undefined}
                  size="fill"
                />
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
                {notice && (
                  <p className={styles.notice} role="status" data-testid="radio-now-notice">
                    {notice}
                  </p>
                )}
                <div className={styles.controls}>
                  <AriaButton
                    variant="primary"
                    size="lg"
                    icon={toggle.pressed ? 'pause' : 'play'}
                    iconOnly
                    aria-label={toggle.label}
                    onPress={toggle.onPress}
                  />
                  {timeShift && <TimeShiftSlider {...timeShift} />}
                  <AriaButton
                    variant="secondary"
                    aria-pressed={isLive}
                    onPress={onLive}
                    data-testid="radio-now-live"
                  >
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
                {played && <PlayedList {...played} stationName={stationName} logoUrl={logoUrl} />}
              </div>
            </div>
          </div>
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}

/** The cover as the view's blurred background; gone when it does not load. */
function Blurred({ src }: { src: string }) {
  const [failed, setFailed] = useState(false);
  if (failed) return null;
  return (
    <div className={styles.backdrop} aria-hidden="true">
      <img src={src} alt="" decoding="async" onError={() => setFailed(true)} />
    </div>
  );
}
