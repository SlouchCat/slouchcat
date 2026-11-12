import type { AdminSettings, AdminSettingsUpdate, SettingSource } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useId, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { Chip, Text, TextLink, VisuallyHidden } from '@/ui';
import {
  AriaButton,
  Form,
  StatusRegion,
  TextField,
  useStatusMessage,
  type Drafts,
} from '@/ui/aria';
import {
  audiobookSourcesQuery,
  checkProviderKey,
  metadataProvidersQuery,
  type KeyProvider,
} from '../queries';
import styles from './Integrations.module.css';
import { Actions, keyDraft, Outcome, sourceTone, useSecretUpdate, type KeyName } from './shared';

/** One provider of the page: its id for «Prüfen» and its fields, the main key first. */
export interface Provider {
  id: KeyProvider;
  keys: readonly [KeyName, ...KeyName[]];
  /** Where a signed-in user gets the key (K12-9). An address, not text: it stays out of the locales. */
  url: string;
}

/**
 * The providers in the order of the page (concept 20, P-12, F-11): TMDB, TVDB, Fanart.tv,
 * YouTube, Trakt, Google Books (O-7, last: the one source of the audiobooks only). TVDB and
 * Fanart.tv carry a second field: the subscriber PIN, the personal key.
 */
export const PROVIDERS: readonly Provider[] = [
  { id: 'tmdb', keys: ['tmdb'], url: 'https://www.themoviedb.org/settings/api' },
  { id: 'tvdb', keys: ['tvdb', 'tvdbPin'], url: 'https://thetvdb.com/dashboard/account/apikey' },
  { id: 'fanart', keys: ['fanart', 'fanartClient'], url: 'https://fanart.tv/get-an-api-key/' },
  {
    id: 'youtube',
    keys: ['youtube'],
    url: 'https://console.cloud.google.com/apis/credentials',
  },
  { id: 'trakt', keys: ['trakt'], url: 'https://developer.trakt.tv/apps' },
  {
    id: 'googlebooks',
    keys: ['googleBooks'],
    url: 'https://console.cloud.google.com/apis/credentials',
  },
];

/** Every key name of the page, in the order of the providers. */
export const PROVIDER_KEYS: readonly KeyName[] = PROVIDERS.flatMap((provider) => provider.keys);

/** The state of a provider: that of its main key (the PIN and the personal key are optional). */
export const providerSource = (provider: Provider, keys: AdminSettings['keys']): SettingSource =>
  keys[provider.keys[0]].source;

/**
 * One provider as a card (mockup «Schlüssel»): folded, its name, what it is for, its state and a
 * button that opens it; open, its fields one below the other, one help sentence with one link, one
 * «Speichern» for all its fields, «Prüfen» (the server asks the provider with the configured key)
 * and «Löschen …». A value from deploy/.env locks the fields and says so once; «Prüfen» stays.
 * Typed input lives in the page's drafts, so folding the card keeps it.
 */
export function ProviderCard({
  provider,
  keys,
  drafts,
}: {
  provider: Provider;
  keys: AdminSettings['keys'];
  drafts: Drafts<string>;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const id = useId();
  const [open, setOpen] = useState(false);
  const [message, setMessage] = useStatusMessage();
  const name = t(`admin.integrations.providers.${provider.id}.name`);
  const source = providerSource(provider, keys);
  const env = source === 'env';
  const stored = provider.keys.filter((keyName) => keys[keyName].source === 'stored');
  const dirty = provider.keys.some((keyName) => drafts.isDirty(keyDraft(keyName)));
  const save = useSecretUpdate((body: AdminSettingsUpdate) => {
    if (body.clear) for (const keyName of provider.keys) drafts.discard(keyDraft(keyName));
    setMessage(t(body.clear ? 'admin.integrations.cleared' : 'admin.integrations.saved', { name }));
    // The audiobook sources show the same Google Books key, and the editor offers the source
    // only while it has one.
    if (provider.id === 'googlebooks') {
      void queryClient.invalidateQueries({ queryKey: audiobookSourcesQuery.queryKey });
      void queryClient.invalidateQueries({ queryKey: metadataProvidersQuery.queryKey });
    }
  });
  const check = useMutation({
    mutationFn: () => checkProviderKey(provider.id),
    onSuccess: (result) =>
      setMessage(
        result.ok
          ? t('admin.integrations.providers.check.ok', { name })
          : t(`admin.integrations.providers.check.${result.problem ?? 'failed'}`, {
              name,
              message: result.message ?? '',
            }),
      ),
    onError: () => setMessage(null),
  });
  const fieldOf = (keyName: KeyName) => `keys.${keyName}`;
  const errors = save.error ? formErrors(t, save.error, { tmdbApiKey: fieldOf('tmdb') }) : null;
  const keepHintId = `${id}-keep`;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    // Empty fields keep what is stored (decision 18): nothing typed, nothing to send.
    const sent = Object.fromEntries(
      provider.keys.flatMap((keyName) => {
        const value = drafts.valueOf(keyDraft(keyName)) ?? '';
        return value === '' ? [] : [[keyName, value.trim()] as const];
      }),
    );
    if (Object.keys(sent).length === 0) return;
    setMessage(null);
    void save.run({ keys: sent }).then((outcome) => {
      // What was typed while the save ran stays a draft (decision 18, addendum 3).
      if (!outcome.ok) return;
      for (const keyName of provider.keys) {
        if (sent[keyName] === undefined) continue;
        drafts.set(keyDraft(keyName), (current) =>
          current?.trim() === sent[keyName] ? '' : (current ?? ''),
        );
      }
    });
  };

  const action = open
    ? t('admin.integrations.providers.close')
    : t(
        `admin.integrations.providers.open.${env ? 'view' : source === 'stored' ? 'edit' : 'setup'}`,
      );
  const placeholder = (keyName: KeyName) => {
    const state = keys[keyName];
    if (state.source === 'env') return t('admin.integrations.source.env');
    if (state.source === 'none') return t('admin.integrations.source.none');
    return state.hint
      ? t('admin.integrations.providers.storedHint', { hint: state.hint })
      : t('admin.integrations.providers.stored');
  };

  return (
    <section className={styles.provider} aria-labelledby={`${id}-name`}>
      <div className={styles.providerHead}>
        <span className={styles.providerText}>
          <h3 id={`${id}-name`} className={styles.providerName}>
            {name}
          </h3>
          <Text tone="secondary">{t(`admin.integrations.providers.${provider.id}.purpose`)}</Text>
        </span>
        <span className={styles.actions}>
          {dirty && <Chip tone="warning">{t('admin.integrations.unsaved')}</Chip>}
          <Chip tone={sourceTone[source]}>{t(`admin.integrations.source.${source}`)}</Chip>
          <AriaButton
            variant="secondary"
            aria-expanded={open}
            aria-controls={open ? `${id}-body` : undefined}
            aria-label={t('admin.integrations.providers.toggleLabel', { action, name })}
            onPress={() => setOpen(!open)}
          >
            {action}
          </AriaButton>
        </span>
      </div>
      {open && (
        <div id={`${id}-body`} className={styles.providerBody}>
          <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
            {provider.keys.map((keyName) => (
              <TextField
                key={keyName}
                name={fieldOf(keyName)}
                type="password"
                label={t(`admin.integrations.providers.fields.${keyName}`)}
                placeholder={placeholder(keyName)}
                aria-describedby={keys[keyName].source === 'stored' ? keepHintId : undefined}
                autoComplete="off"
                spellCheck="false"
                mono
                isDisabled={env}
                value={drafts.valueOf(keyDraft(keyName)) ?? ''}
                onChange={(text) => drafts.set(keyDraft(keyName), text)}
                // Empty keeps the stored key; only blanks alone are no key.
                validate={(text) =>
                  env || text === '' || text.trim() ? null : t('errors.field.required')
                }
              />
            ))}
            <Text tone="secondary">
              {env
                ? t('admin.integrations.providers.envNote')
                : t(`admin.integrations.providers.${provider.id}.help`)}{' '}
              <TextLink href={provider.url} external>
                {t('admin.integrations.providers.link', { name })}
              </TextLink>
            </Text>
            {env ? (
              <span className={styles.actions}>
                <CheckButton
                  name={name}
                  isPending={check.isPending}
                  onPress={() => check.mutate()}
                />
              </span>
            ) : (
              <Actions
                name={name}
                saveLabel={t('admin.integrations.saveLabel', { name })}
                stored={stored.length > 0}
                isPending={save.isPending}
                onClear={() => {
                  setMessage(null);
                  return save.run({ clear: stored });
                }}
              >
                {source !== 'none' && (
                  <CheckButton
                    name={name}
                    isPending={check.isPending}
                    onPress={() => check.mutate()}
                  />
                )}
              </Actions>
            )}
          </Form>
          {stored.length > 0 && (
            <VisuallyHidden id={keepHintId}>{t('admin.integrations.keys.newValue')}</VisuallyHidden>
          )}
          <Outcome error={errors?.form ? save.error : check.error} message={message} />
        </div>
      )}
      {!open && <StatusRegion message={message} />}
    </section>
  );
}

/** «Prüfen»: asks the provider with the configured key, not with what is typed. */
function CheckButton({
  name,
  isPending,
  onPress,
}: {
  name: string;
  isPending: boolean;
  onPress: () => void;
}) {
  const { t } = useTranslation();
  return (
    <AriaButton
      variant="secondary"
      isPending={isPending}
      aria-label={t('admin.integrations.providers.checkLabel', { name })}
      onPress={onPress}
    >
      {t('admin.integrations.providers.checkAction')}
    </AriaButton>
  );
}
