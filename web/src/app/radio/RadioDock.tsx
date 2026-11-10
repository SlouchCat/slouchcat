import type { TFunction } from 'i18next';
import { lazy, Suspense, useState, type ComponentProps, type ComponentType } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { codeMessage } from '@/api/errors';
import { volumeIsReadOnly } from '@/features/player/layout';
import { formatPlayerTime } from '@/lib/format';
import { MiniPlayer, SpatialGroup, type MiniPlayerState } from '@/ui';
import type {
  NowPlayingDialog as NowPlayingDialogType,
  VolumeControl as VolumeControlType,
} from '@/ui/aria';
import { RADIO_SEGMENT_SECONDS } from './engine';
import { facetValueName, radioLanguageName, stationPlace } from './labels';
import { wantsSound, type RadioNotice, type RadioStation } from './priority';
import {
  radioImageSize,
  useRadioFavorite,
  type RadioImageSize,
  type RadioNowEntry,
} from './queries';
import { useRadio, useRadioOffset } from './useRadio';

// The radio's controls in the app shell (AppLayout): the mini player while a station is chosen
// and, opened from it, «Jetzt läuft». The dialog and the volume build on react-aria and load on
// first use, so the start bundle stays without it (P-10).

type DialogModule = { default: ComponentType<ComponentProps<typeof NowPlayingDialogType>> };
type VolumeModule = { default: ComponentType<ComponentProps<typeof VolumeControlType>> };

const NowPlayingDialog = lazy(() =>
  import('@/ui/aria').then(
    (module): DialogModule => ({ default: module.NowPlayingDialog }),
    (): DialogModule => ({ default: () => null }),
  ),
);

const VolumeControl = lazy(() =>
  import('@/ui/aria').then(
    (module): VolumeModule => ({ default: module.VolumeControl }),
    (): VolumeModule => ({ default: () => null }),
  ),
);

/** Focus key of the mini player in the shell's arrow-key navigation (AppLayout). */
export const RADIO_FOCUS_KEY = 'radio';

function noticeText(t: TFunction, notice: RadioNotice): string {
  switch (notice.kind) {
    case 'watchLimit':
      // Title, limit and today's minutes, as the player page says them.
      return [
        t('player.watchLimit.title'),
        notice.limitMinutes === undefined
          ? t('player.watchLimit.bodyUnknown')
          : t('player.watchLimit.body', { count: notice.limitMinutes }),
        notice.usedMinutes !== undefined &&
          t('player.watchLimit.used', { count: notice.usedMinutes }),
      ]
        .filter(Boolean)
        .join(' ');
    case 'ended':
      return t('radio.player.ended');
    case 'failed':
      return t('radio.player.failed');
    case 'problem':
      return notice.code === 'network'
        ? t('errors.code.network')
        : (codeMessage(t, notice.code) ?? t('errors.code.unknown'));
  }
}

function facts(t: TFunction, language: string, station: RadioStation) {
  const list: { label: string; value: string }[] = [];
  if (station.codec || station.bitrateKbps) {
    list.push({
      label: t('radio.player.facts.quality'),
      value: [
        station.codec,
        station.bitrateKbps && t('radio.player.kbps', { count: station.bitrateKbps }),
      ]
        .filter(Boolean)
        .join(' '),
    });
  }
  const genres = station.genres.length > 0 ? station.genres : station.kinds;
  if (genres.length > 0) {
    list.push({
      label: t(station.genres.length > 0 ? 'radio.player.facts.genre' : 'radio.player.facts.kind'),
      value: genres
        .map((key) => facetValueName(t, station.genres.length > 0 ? 'genre' : 'kind', key))
        .join(', '),
    });
  }
  if (station.languages.length > 0) {
    list.push({
      label: t('radio.player.facts.language'),
      value: station.languages.map((code) => radioLanguageName(language, code)).join(', '),
    });
  }
  return list;
}

/** «12 Minuten 30 Sekunden zurück», «Live»: the spoken time shift. */
function shiftText(t: TFunction, offset: number): string {
  if (offset < RADIO_SEGMENT_SECONDS) return t('radio.player.live');
  const minutes = Math.floor(offset / 60);
  const seconds = offset % 60;
  const parts = [
    minutes > 0 && t('radio.player.shift.minutes', { count: minutes }),
    seconds > 0 && t('radio.player.shift.seconds', { count: seconds }),
  ].filter(Boolean);
  return t('radio.player.shift.back', { time: parts.join(' ') });
}

/** «−12:30», «Live»: the shown time shift. */
const shiftShown = (t: TFunction, offset: number) =>
  offset < RADIO_SEGMENT_SECONDS
    ? t('radio.player.live')
    : t('radio.player.shift.behind', { time: formatPlayerTime(offset * 1000, 0) });

/** The URL of an entry's cover in a size; undefined without a cover. */
const coverOf = (entry: RadioNowEntry | null | undefined, size: RadioImageSize) =>
  imageUrl(entry?.cover && { url: radioImageSize(entry.cover.url, size) ?? entry.cover.url });

export function RadioDock() {
  const { t, i18n } = useTranslation();
  const radio = useRadio();
  const offset = useRadioOffset();
  const favorite = useRadioFavorite();
  const [open, setOpen] = useState(false);
  // Safari on the iPhone does not let a page set the volume: only the mute (11.2).
  const [muteOnly] = useState(() => volumeIsReadOnly());
  const { state, playing, entry, history, timeShiftSeconds } = radio;
  const station = state.station;
  if (!station) return null;

  const notice = state.notice;
  const isLive = offset < RADIO_SEGMENT_SECONDS;
  const view: MiniPlayerState = notice
    ? 'error'
    : state.phase === 'starting'
      ? 'starting'
      : playing
        ? isLive
          ? 'live'
          : 'shifted'
        : 'paused';
  const stateLabel = {
    error: undefined,
    starting: t('radio.player.starting'),
    live: t('radio.player.live'),
    shifted: shiftShown(t, offset),
    paused: t('radio.player.paused'),
  }[view];
  const windowMinutes = Math.round(timeShiftSeconds / 60);
  const longPause = radio.longPause
    ? t('radio.player.longPause', { minutes: windowMinutes })
    : undefined;
  const time = new Intl.DateTimeFormat(i18n.language, { hour: '2-digit', minute: '2-digit' });
  const titleLine = entry
    ? entry.artist && entry.title
      ? t('radio.player.artistTitle', { artist: entry.artist, title: entry.title })
      : entry.text
    : t('radio.player.live');
  // While the station starts the toggle already pauses (the station then opens paused).
  const on = wantsSound(state);
  const toggle = {
    pressed: on,
    label: notice ? t('radio.player.retry') : on ? t('radio.player.pause') : t('radio.player.play'),
    onPress: radio.toggle,
  };
  const favoriteToggle = {
    pressed: station.favorite,
    label: t('radio.player.favorite', { name: station.name }),
    onPress: () => favorite.mutate({ stationId: station.id, favorite: !station.favorite }),
  };
  const logo = imageUrl(station.logo);
  // The shared volume of the device (concept 19 E3, OE-1), no boost: the radio has no gain node.
  const volume = {
    volume: radio.volume,
    muted: radio.muted,
    muteOnly,
    onVolumeChange: radio.setVolume,
    onToggleMute: radio.toggleMute,
  };

  return (
    <>
      <SpatialGroup focusKey={RADIO_FOCUS_KEY}>
        <MiniPlayer
          label={t('radio.player.region')}
          stationName={station.name}
          logoUrl={logo}
          coverUrl={coverOf(entry, 'w96')}
          line={notice ? noticeText(t, notice) : titleLine}
          state={view}
          stateLabel={stateLabel}
          hint={
            longPause && !notice
              ? { text: longPause, actionLabel: t('radio.player.toLive'), onAction: radio.toLive }
              : undefined
          }
          openLabel={t('radio.player.open', { name: station.name })}
          onOpen={() => setOpen(true)}
          toggle={toggle}
          favorite={notice ? undefined : favoriteToggle}
          volume={{
            control: (
              <Suspense fallback={null}>
                <VolumeControl {...volume} placement="above" />
              </Suspense>
            ),
            onToggleMute: radio.toggleMute,
          }}
          closeLabel={t('radio.player.stop')}
          onClose={() => {
            setOpen(false);
            if (notice) radio.dismiss();
            else radio.stop();
          }}
        />
      </SpatialGroup>
      {open && (
        <Suspense fallback={null}>
          <NowPlayingDialog
            isOpen
            onOpenChange={setOpen}
            heading={t('radio.player.nowPlaying')}
            closeLabel={t('radio.player.close')}
            stationName={station.name}
            logoUrl={logo}
            coverUrl={coverOf(entry, 'w512')}
            place={stationPlace(t, i18n.language, station)}
            title={entry ? (entry.title ?? entry.text) : t('radio.player.programme')}
            subtitle={entry ? (entry.title ? entry.artist : undefined) : t('radio.player.noTitles')}
            toggle={toggle}
            liveLabel={t('radio.player.toLive')}
            onLive={radio.toLive}
            isLive={isLive}
            timeShift={
              timeShiftSeconds > 0 && state.phase === 'live'
                ? {
                    label: t('radio.player.shift.label', { minutes: windowMinutes }),
                    maxSeconds: timeShiftSeconds,
                    offset,
                    onSeek: radio.seek,
                    valueText: (value) => shiftText(t, value),
                    shownText: (value) => shiftShown(t, value),
                    startLabel: t('radio.player.shift.behind', {
                      time: formatPlayerTime(timeShiftSeconds * 1000, 0),
                    }),
                    endLabel: t('radio.player.live'),
                  }
                : undefined
            }
            notice={notice ? undefined : longPause}
            played={
              entry || history.length > 0
                ? {
                    heading: t('radio.player.played.heading'),
                    empty: t('radio.player.played.empty'),
                    entries: history.map((played) => ({
                      key: `${played.at}-${played.text}`,
                      title: played.title ?? played.text,
                      subtitle: played.title ? played.artist : undefined,
                      time: time.format(played.at),
                      dateTime: new Date(played.at).toISOString(),
                      coverUrl: coverOf(played, 'w96'),
                    })),
                  }
                : undefined
            }
            favorite={notice ? undefined : favoriteToggle}
            facts={facts(t, i18n.language, station)}
            hint={t('radio.player.serverHint')}
            volume={volume}
          />
        </Suspense>
      )}
    </>
  );
}
