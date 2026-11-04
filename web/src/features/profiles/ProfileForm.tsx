import type { PlaybackPrefs, Profile, ProfileRequest } from '@slouchcat/api-client';
import { accentNames, type AccentName } from '@slouchcat/design-tokens';
import { useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { refreshSession, type ActiveSession } from '@/app/session';
import { currentLanguage, isLanguage, type Language } from '@/i18n';
import { formText } from '@/lib/form';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { isAccentName, Notice } from '@/ui';
import { AccentPicker, AriaButton, Form, PinField, RadioGroup, TextField } from '@/ui/aria';
import { accentOptions, languageOptions, ratingOf, ratingOptions } from '../shared/choices';
import { LibraryChoice, type LibrarySelection } from '../shared/LibraryChoice';
import {
  isCapped,
  LimitFields,
  limitParts,
  limitsOf,
  limitValues,
  type LimitValues,
} from '../shared/LimitFields';
import { libraryIdsOf, type LibraryOption } from '../shared/useLibraryOptions';
import { myPlaybackPrefsQuery } from './playbackPrefs';
import { PlaybackPrefsFields } from './PlaybackPrefsFields';
import styles from './ProfileForm.module.css';

interface ProfileFormProps {
  /** The profile to edit; none creates a new one. */
  profile?: Profile;
  /** All profiles of the account (the new one starts with a colour no other uses). */
  profiles: readonly Profile[];
  session: ActiveSession;
  /** The library choices; the form is only shown once the list is known. */
  libraries: readonly LibraryOption[];
  /** The profile was saved (before the dialog closes). */
  onSaved?: () => void;
  onClose: () => void;
}

/**
 * Create or edit a profile (K-2): name, rating, language, accent, libraries, the playback languages
 * and subtitles (concept 13 11.4), a PIN on creation.
 */
export function ProfileForm({
  profile,
  profiles,
  session,
  libraries,
  onSaved,
  onClose,
}: ProfileFormProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const accountMax = session.account.maxRating;

  const firstFreeAccent: AccentName =
    accentNames.find((name) => !profiles.some((other) => other.accent === name)) ?? accentNames[0];
  // The stored values, as they are: the form never rewrites a rating (the server decides what is
  // allowed, and answers `above_account_limit` for a value over the account's ceiling).
  const [rating, setRating] = useState(String(profile?.maxRating ?? accountMax));
  const [language, setLanguage] = useState<Language>(profile?.language ?? currentLanguage());
  const [metadataLanguage, setMetadataLanguage] = useState(profile?.metadataLanguage ?? '');
  // The metadata languages a profile may choose (ADR 0018): the field shows only when the server
  // offers more than the interface languages, or the profile has a value to reset.
  const available = session.availableMetadataLanguages ?? ['de', 'en'];
  const extraLanguages = available.filter((code) => code !== 'de' && code !== 'en');
  const showMetadataLanguage = extraLanguages.length > 0 || metadataLanguage !== '';
  const languageName = (code: string) =>
    new Intl.DisplayNames([language], { type: 'language' }).of(code) ?? code;
  const [accent, setAccent] = useState<string>(profile?.accent ?? firstFreeAccent);
  const [selection, setSelection] = useState<LibrarySelection>({
    all: profile?.allLibraries ?? true,
    ids: (profile?.libraryIds ?? []).map(String),
  });
  const [pin, setPin] = useState('');
  const [limits, setLimits] = useState<LimitValues>(() => limitValues(profile?.limits));
  // The playback preferences as the profile holds them. A new profile leaves them to the server
  // until they are changed: it starts them from the language chosen above (F-12), so the fields
  // follow that language meanwhile.
  const [prefs, setPrefs] = useState<PlaybackPrefs | null>(() =>
    profile
      ? {
          audioLanguages: profile.audioLanguages,
          subtitleMode: profile.subtitleMode,
          subtitleLanguages: profile.subtitleLanguages,
        }
      : null,
  );
  const shownPrefs: PlaybackPrefs = prefs ?? {
    audioLanguages: [language],
    subtitleMode: 'forced',
    subtitleLanguages: [language],
  };

  const save = useTransientMutation(
    (body: ProfileRequest): Promise<Profile> =>
      profile
        ? request(api.PATCH('/profiles/{id}', { params: { path: { id: profile.id } }, body }))
        : request(api.POST('/profiles', { body })),
    async (saved) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['profiles'] }),
        queryClient.invalidateQueries({ queryKey: ['auth'] }),
        // «Meine Wiedergabe» and the sound panel read the active profile's lists from there.
        ...(saved.id === session.profile.id
          ? [queryClient.invalidateQueries({ queryKey: myPlaybackPrefsQuery.queryKey })]
          : []),
      ]);
      // Editing the profile in use is like switching to it: the catalog is read again, and name,
      // language and colour of the app follow.
      if (saved.id === session.profile.id) await refreshSession(queryClient);
      onSaved?.();
      onClose();
    },
  );
  const errors = save.error ? formErrors(t, save.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    void save.run({
      name: formText(data, 'name'),
      maxRating: ratingOf(rating),
      language,
      ...(showMetadataLanguage || profile?.metadataLanguage ? { metadataLanguage } : {}),
      ...(isAccentName(accent) ? { accent } : {}),
      allLibraries: selection.all,
      libraryIds: selection.all ? [] : libraryIdsOf(selection.ids),
      limits: limitsOf(limits),
      ...(prefs ?? {}),
      ...(!profile && pin ? { pin } : {}),
    });
  };

  const submitLabel = profile
    ? t(save.isPending ? 'profiles.settings.form.saving' : 'profiles.settings.form.save')
    : t(save.isPending ? 'profiles.settings.form.creating' : 'profiles.settings.form.create');

  return (
    <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
      {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
      <TextField
        name="name"
        label={t('profiles.settings.form.name')}
        defaultValue={profile?.name}
        maxLength={60}
        autoComplete="off"
        isRequired
        validate={(value) => (value.trim() ? null : t('errors.field.required'))}
        autoFocus
      />
      <RadioGroup
        name="maxRating"
        label={t('profiles.settings.form.rating')}
        description={t('profiles.settings.form.ratingHint')}
        value={rating}
        onChange={setRating}
        options={ratingOptions(t, accountMax)}
      />
      <RadioGroup
        name="language"
        label={t('profiles.settings.form.language')}
        value={language}
        onChange={(value) => isLanguage(value) && setLanguage(value)}
        options={languageOptions(t)}
      />
      {showMetadataLanguage && (
        <RadioGroup
          name="metadataLanguage"
          label={t('profiles.settings.form.metadataLanguage')}
          description={t('profiles.settings.form.metadataLanguageHint')}
          value={metadataLanguage}
          onChange={setMetadataLanguage}
          options={[
            { value: '', label: t('profiles.settings.form.likeUi') },
            ...available
              .filter((code) => code !== language)
              .map((code) => ({ value: code, label: languageName(code) })),
            ...(metadataLanguage && !available.includes(metadataLanguage)
              ? [{ value: metadataLanguage, label: languageName(metadataLanguage) }]
              : []),
          ]}
        />
      )}
      <AccentPicker
        name="accent"
        label={t('accent.label')}
        value={accent}
        onChange={setAccent}
        options={accentOptions(t)}
      />
      <LibraryChoice
        label={t('profiles.settings.form.libraries')}
        value={selection}
        onChange={setSelection}
        options={libraries}
      />
      <fieldset className={styles.group}>
        <legend className={styles.legend}>{t('profiles.settings.form.playback')}</legend>
        <PlaybackPrefsFields value={shownPrefs} onChange={setPrefs} errors={errors?.fields} />
      </fieldset>
      <fieldset className={styles.group}>
        <legend className={styles.legend}>{t('limits.title')}</legend>
        <LimitFields value={limits} onChange={setLimits} />
        {profile && isCapped(profile.limits, profile.effectiveLimits) && (
          <Notice>
            {t('limits.capped', { summary: limitParts(t, profile.effectiveLimits).join(' · ') })}
          </Notice>
        )}
      </fieldset>
      {!profile && (
        <PinField
          name="pin"
          label={t('profiles.settings.form.pin')}
          description={t('profiles.pin.hint')}
          invalidMessage={t('errors.field.pin_format')}
          value={pin}
          onChange={setPin}
          isOptional
        />
      )}
      <div className={styles.actions}>
        <AriaButton variant="ghost" onPress={onClose}>
          {t('common.cancel')}
        </AriaButton>
        <AriaButton type="submit" variant="primary" isPending={save.isPending}>
          {submitLabel}
        </AriaButton>
      </div>
    </Form>
  );
}
