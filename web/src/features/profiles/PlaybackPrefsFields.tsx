import type { PlaybackPrefs, SubtitleMode } from '@slouchcat/api-client';
import { useId, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { languageName } from '@/features/player/labels';
import { RadioGroup, RankedChoiceList } from '@/ui/aria';
import { MAX_PLAYBACK_LANGUAGES, PLAYBACK_LANGUAGES } from './playbackPrefs';
import styles from './PlaybackPrefsFields.module.css';

const SUBTITLE_MODES: readonly SubtitleMode[] = ['off', 'forced', 'foreign', 'always'];

const isSubtitleMode = (value: string): value is SubtitleMode =>
  SUBTITLE_MODES.includes(value as SubtitleMode);

interface LanguageListProps {
  legend: string;
  hint: string;
  value: readonly string[];
  onChange: (value: string[]) => void;
  rankLabel: (index: number) => string;
  addLabel: string;
  /** The server's message for the list (422 `invalid_value`, `too_long`). */
  error?: string;
}

/**
 * One ordered language list (RankedChoiceList): places with a select each, «×» and the add
 * button. A full list (ten languages) offers only its own languages, so its places can still swap
 * but a new language needs a free place first.
 */
function LanguageList({
  legend,
  hint,
  value,
  onChange,
  rankLabel,
  addLabel,
  error,
}: LanguageListProps) {
  const { t, i18n } = useTranslation();
  const hintId = useId();
  const errorId = useId();
  const full = value.length >= MAX_PLAYBACK_LANGUAGES;
  const choices = useMemo(() => {
    const codes = full ? [...value] : [...new Set([...PLAYBACK_LANGUAGES, ...value])];
    return codes
      .map((code) => ({ value: code, label: languageName(code, i18n.language) ?? code }))
      .sort((a, b) => a.label.localeCompare(b.label, i18n.language));
  }, [full, value, i18n.language]);
  return (
    <fieldset className={styles.group} aria-describedby={error ? `${hintId} ${errorId}` : hintId}>
      <legend className={styles.legend}>{legend}</legend>
      <p id={hintId} className={styles.hint}>
        {hint}
        {value.length === 0 && ` ${t('settings.playback.none')}`}
        {full && ` ${t('settings.playback.limit', { count: MAX_PLAYBACK_LANGUAGES })}`}
      </p>
      {error && (
        <p id={errorId} className={styles.error}>
          {error}
        </p>
      )}
      <RankedChoiceList
        choices={choices}
        value={value}
        onChange={onChange}
        rankLabel={rankLabel}
        addLabel={addLabel}
      />
    </fieldset>
  );
}

interface PlaybackPrefsFieldsProps {
  value: PlaybackPrefs;
  onChange: (value: PlaybackPrefs) => void;
  /**
   * The server's messages by field (formErrors); the subtitle rule takes its own from the form's
   * `validationErrors` under the name `subtitleMode`.
   */
  errors?: Readonly<Record<string, string>>;
}

/**
 * The language and subtitle preferences of a profile (concept 13 11.4, F-13): the audio languages
 * in order, the subtitle rule and the subtitle languages in order. «Meine Wiedergabe» shows them
 * for the active profile, the profile form of the primary profile for any profile. The server
 * chooses the tracks from them (rule 12); the fields only hold the values.
 */
export function PlaybackPrefsFields({ value, onChange, errors }: PlaybackPrefsFieldsProps) {
  const { t } = useTranslation();
  return (
    <div className={styles.fields}>
      <LanguageList
        legend={t('settings.playback.audio.legend')}
        hint={t('settings.playback.audio.hint')}
        value={value.audioLanguages}
        onChange={(audioLanguages) => onChange({ ...value, audioLanguages })}
        rankLabel={(index) => t('settings.playback.audio.rank', { position: index + 1 })}
        addLabel={t('settings.playback.audio.more')}
        error={errors?.audioLanguages}
      />
      <RadioGroup
        name="subtitleMode"
        label={t('settings.playback.subtitleMode.label')}
        description={t(`settings.playback.subtitleMode.${value.subtitleMode}Hint`)}
        orientation="vertical"
        value={value.subtitleMode}
        onChange={(mode) => isSubtitleMode(mode) && onChange({ ...value, subtitleMode: mode })}
        options={SUBTITLE_MODES.map((mode) => ({
          value: mode,
          label: t(`settings.playback.subtitleMode.${mode}`),
        }))}
      />
      <LanguageList
        legend={t('settings.playback.subtitles.legend')}
        hint={t('settings.playback.subtitles.hint')}
        value={value.subtitleLanguages}
        onChange={(subtitleLanguages) => onChange({ ...value, subtitleLanguages })}
        rankLabel={(index) => t('settings.playback.subtitles.rank', { position: index + 1 })}
        addLabel={t('settings.playback.subtitles.more')}
        error={errors?.subtitleLanguages}
      />
    </div>
  );
}
