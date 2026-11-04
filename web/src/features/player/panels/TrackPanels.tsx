import type { Track } from '@slouchcat/api-client';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import {
  AriaButton,
  OptionList,
  OptionMoreRow,
  OptionRow,
  PlayerPanel,
  type OptionMark,
} from '@/ui/aria';
import { audioKey, subtitleKey, trackLabel } from '../labels';
import { isSdh, orderTracks, type PlacedTrack } from '../trackOrder';
import type { SubtitleStyle } from '../useSubtitleStyle';
import { PANEL_IDS } from './panelIds';
import { SubtitleStyleOptions } from './SubtitleStyleOptions';
import styles from './SubtitleStyleOptions.module.css';

const MORE = 'more';

interface TrackListProps {
  label: string;
  tracks: readonly Track[];
  /** The profile's language: its tracks come first, then English (11.10). */
  profileLanguage: string | undefined;
  keyOf: (track: Track) => string;
  marksOf: (track: Track) => OptionMark[];
  selectedKey: string | null;
  onSelect: (key: string) => void;
  /** Rows before the tracks («Aus»). */
  leading?: ReactNode;
}

/**
 * The tracks of a panel in the order of 11.10: the profile's language and English first, the
 * others behind «Weitere Sprachen (n)», folded unless the chosen track is among them or there is
 * nothing else to show.
 */
function TrackList({
  label,
  tracks,
  profileLanguage,
  keyOf,
  marksOf,
  selectedKey,
  onSelect,
  leading,
}: TrackListProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  const nameOf = ({ track, position }: PlacedTrack) =>
    trackLabel(t, track, position, language, { forced: false });
  const { mine, more } = orderTracks(tracks, profileLanguage, nameOf, language);
  const [expanded, setExpanded] = useState(
    () => mine.length === 0 || more.some(({ track }) => keyOf(track) === selectedKey),
  );
  const row = (placed: PlacedTrack) => (
    <OptionRow
      key={keyOf(placed.track)}
      id={keyOf(placed.track)}
      label={nameOf(placed)}
      marks={marksOf(placed.track)}
    />
  );
  return (
    <OptionList
      label={label}
      selectedId={selectedKey}
      onSelect={onSelect}
      actionIds={more.length > 0 ? [MORE] : []}
    >
      {leading}
      {mine.map(row)}
      {more.length > 0 && (
        <OptionMoreRow
          id={MORE}
          label={t('player.panel.moreLanguages', { count: more.length })}
          isExpanded={expanded}
          onToggle={() => setExpanded((shown) => !shown)}
        />
      )}
      {expanded && more.map(row)}
    </OptionList>
  );
}

interface AudioPanelProps {
  tracks: readonly Track[];
  profileLanguage: string | undefined;
  /** Stream index of the audio track the server delivers. */
  selectedIndex: number | undefined;
  /** Whether choosing a track makes the server convert the stream (a mark, nothing more). */
  converts: (track: Track) => boolean;
  onSelect: (index: number) => void;
  onClose: () => void;
}

/** The sound panel (key T, V-5): «Deutsch · DTS · 5.1» with the marks «Standard», «Stream wird neu aufgebaut». */
export function AudioPanel({
  tracks,
  profileLanguage,
  selectedIndex,
  converts,
  onSelect,
  onClose,
}: AudioPanelProps) {
  const { t } = useTranslation();
  return (
    <PlayerPanel id={PANEL_IDS.audio} title={t('player.tracks.audio')} onClose={onClose}>
      <TrackList
        label={t('player.tracks.audio')}
        tracks={tracks}
        profileLanguage={profileLanguage}
        keyOf={(track) => audioKey(track.index)}
        marksOf={(track) => [
          ...(track.default ? (['default'] as const) : []),
          ...(converts(track) ? (['convert'] as const) : []),
        ]}
        selectedKey={selectedIndex === undefined ? null : audioKey(selectedIndex)}
        onSelect={(key) => onSelect(Number(key.slice('audio:'.length)))}
      />
    </PlayerPanel>
  );
}

interface SubtitlePanelProps {
  tracks: readonly Track[];
  profileLanguage: string | undefined;
  /** Id of the subtitle track the server delivers; undefined for none. */
  selectedId: number | undefined;
  /** A track id, or null for «Aus». */
  onSelect: (id: number | null) => void;
  onClose: () => void;
  /** The subtitle style (V-6): a button below the list leads to its page. */
  subtitleStyle?: { value: SubtitleStyle; onChange: (patch: Partial<SubtitleStyle>) => void };
}

/**
 * The subtitle panel (key U, V-5): «Aus» first, then the tracks with the marks «erzwungen», «SDH»
 * and «Bild-Untertitel, wird eingebrannt» (a picture subtitle starts a transcode). Below the list
 * the way to the subtitle style, a page of the same panel (back button and Backspace go up).
 */
export function SubtitlePanel({
  tracks,
  profileLanguage,
  selectedId,
  onSelect,
  onClose,
  subtitleStyle,
}: SubtitlePanelProps) {
  const { t } = useTranslation();
  const [styling, setStyling] = useState(false);
  if (styling && subtitleStyle) {
    return (
      <PlayerPanel
        id={PANEL_IDS.subtitle}
        title={t('player.subtitleStyle.title')}
        onClose={onClose}
        onBack={() => setStyling(false)}
      >
        <SubtitleStyleOptions value={subtitleStyle.value} onChange={subtitleStyle.onChange} />
      </PlayerPanel>
    );
  }
  return (
    <PlayerPanel id={PANEL_IDS.subtitle} title={t('player.tracks.subtitles')} onClose={onClose}>
      <TrackList
        label={t('player.tracks.subtitles')}
        tracks={tracks}
        profileLanguage={profileLanguage}
        keyOf={(track) => subtitleKey(track.id)}
        marksOf={(track) => [
          ...(track.forced ? (['forced'] as const) : []),
          ...(isSdh(track) ? (['sdh'] as const) : []),
          ...(track.text ? [] : (['burnIn'] as const)),
        ]}
        selectedKey={subtitleKey(selectedId)}
        onSelect={(key) => {
          const value = key.slice('subtitle:'.length);
          onSelect(value === 'off' ? null : Number(value));
        }}
        leading={<OptionRow id={subtitleKey(undefined)} label={t('player.tracks.off')} />}
      />
      {subtitleStyle && (
        <div className={styles.footer}>
          <AriaButton variant="ghost" icon="settings" block onPress={() => setStyling(true)}>
            {t('player.subtitleStyle.title')}
          </AriaButton>
        </div>
      )}
    </PlayerPanel>
  );
}
