import type { TFunction } from 'i18next';
import { lazy, Suspense, useState, type ComponentProps, type ComponentType } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { codeMessage } from '@/api/errors';
import { MiniPlayer, SpatialGroup, type MiniPlayerState } from '@/ui';
import type { NowPlayingDialog as NowPlayingDialogType } from '@/ui/aria';
import { facetValueName, radioLanguageName, stationPlace } from './labels';
import { wantsSound, type RadioNotice, type RadioStation } from './priority';
import { useRadioFavorite } from './queries';
import { useRadio } from './useRadio';

// The radio's controls in the app shell (AppLayout): the mini player while a station is chosen
// and, opened from it, «Jetzt läuft». The dialog builds on react-aria and loads on first use, so
// the start bundle stays without it (P-10).

type DialogModule = { default: ComponentType<ComponentProps<typeof NowPlayingDialogType>> };

const NowPlayingDialog = lazy(() =>
  import('@/ui/aria').then(
    (module): DialogModule => ({ default: module.NowPlayingDialog }),
    (): DialogModule => ({ default: () => null }),
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

export function RadioDock() {
  const { t, i18n } = useTranslation();
  const radio = useRadio();
  const favorite = useRadioFavorite();
  const [open, setOpen] = useState(false);
  const { state, playing, entry } = radio;
  const station = state.station;
  if (!station) return null;

  const notice = state.notice;
  const view: MiniPlayerState = notice
    ? 'error'
    : state.phase === 'starting'
      ? 'starting'
      : playing
        ? 'live'
        : 'paused';
  const stateLabel = {
    error: undefined,
    starting: t('radio.player.starting'),
    live: t('radio.player.live'),
    paused: t('radio.player.paused'),
  }[view];
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

  return (
    <>
      <SpatialGroup focusKey={RADIO_FOCUS_KEY}>
        <MiniPlayer
          label={t('radio.player.region')}
          stationName={station.name}
          logoUrl={logo}
          line={notice ? noticeText(t, notice) : titleLine}
          state={view}
          stateLabel={stateLabel}
          openLabel={t('radio.player.open', { name: station.name })}
          onOpen={() => setOpen(true)}
          toggle={toggle}
          favorite={notice ? undefined : favoriteToggle}
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
            place={stationPlace(t, i18n.language, station)}
            title={entry ? (entry.title ?? entry.text) : t('radio.player.programme')}
            subtitle={entry ? (entry.title ? entry.artist : undefined) : t('radio.player.noTitles')}
            toggle={toggle}
            liveLabel={t('radio.player.toLive')}
            onLive={radio.toLive}
            favorite={notice ? undefined : favoriteToggle}
            facts={facts(t, i18n.language, station)}
            hint={t('radio.player.serverHint')}
          />
        </Suspense>
      )}
    </>
  );
}
