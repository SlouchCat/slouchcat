import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { Chip, Notice, Text } from '@/ui';
import {
  afterRender,
  AriaButton,
  Form,
  StatusRegion,
  TextField,
  useStatusMessage,
  type Drafts,
} from '@/ui/aria';
import {
  importSeerrRequests,
  seerrRequestCountQuery,
  setProfileTagName,
  wishKeys,
  type WishAccountMapping,
} from '../queries';
import styles from './Wishes.module.css';

type Profile = NonNullable<WishAccountMapping['profiles']>[number];

/** What Radarr takes as a label (appendix I, N-113): the server checks it again. */
const TAG_NAME = /^[a-z0-9-]{1,64}$/;

/** The draft id of a profile's tag name in the page's drafts. */
export const tagDraft = (profileId: number) => `tag:${profileId}`;

/**
 * One profile with its tag name: «Ändern» opens the field in place (the same form as for a new
 * name), «Speichern» stores it, «Zurücksetzen» goes back to the name from the prefix. A name
 * another profile has is refused at the field (409 `tag_name_taken`). The typed name is a draft
 * of the page, so leaving asks first.
 */
function ProfileTag({ profile, drafts }: { profile: Profile; drafts: Drafts<string> }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const id = tagDraft(profile.profileId);
  const [editing, setEditing] = useState(false);
  const [message, say] = useStatusMessage();
  const changeRef = useRef<HTMLButtonElement>(null);
  const fieldRef = useRef<HTMLInputElement>(null);
  const open = editing || drafts.isDirty(id);
  const value = drafts.valueOf(id) ?? profile.tagName;
  const save = useMutation({
    mutationFn: (name: string | null) => setProfileTagName(profile.profileId, name),
    onMutate: () => say(null),
    onSuccess: async (next) => {
      drafts.discard(id);
      setEditing(false);
      say(t('admin.wishes.users.profiles.saved', { profile: profile.name, name: next.name }));
      await queryClient.invalidateQueries({ queryKey: wishKeys.all });
      await afterRender();
      changeRef.current?.focus();
    },
  });
  // A name of another profile or one the server refuses stands at the field.
  const refused =
    save.error && (isApiError(save.error, 409) || isApiError(save.error, 422))
      ? { name: problemMessage(t, save.error) }
      : undefined;
  const close = () => {
    drafts.discard(id);
    save.reset();
    setEditing(false);
    void afterRender().then(() => changeRef.current?.focus());
  };
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = value.trim();
    if (!TAG_NAME.test(name)) return;
    if (name === profile.tagName) {
      close();
      return;
    }
    save.mutate(name);
  };

  return (
    <li className={styles.profile}>
      <span className={styles.chips}>
        <span className={styles.profileName}>{profile.name}</span>
        <Chip tone="outline">{profile.tagName}</Chip>
        {profile.tagNameCustom && (
          <Chip tone="info">{t('admin.wishes.users.profiles.custom')}</Chip>
        )}
        {!open && (
          <AriaButton
            ref={changeRef}
            variant="ghost"
            size="sm"
            icon="pen"
            aria-label={t('admin.wishes.users.profiles.changeLabel', { profile: profile.name })}
            onPress={() => {
              say(null);
              setEditing(true);
              void afterRender().then(() => fieldRef.current?.focus());
            }}
          >
            {t('admin.wishes.users.profiles.change')}
          </AriaButton>
        )}
      </span>
      {open && (
        <Form className={styles.tagForm} onSubmit={onSubmit} validationErrors={refused}>
          <TextField
            name="name"
            label={t('admin.wishes.users.profiles.field', { profile: profile.name })}
            description={t('admin.wishes.users.profiles.fieldHelp')}
            mono
            autoComplete="off"
            spellCheck="false"
            inputRef={fieldRef}
            value={value}
            onChange={(text) => drafts.set(id, text)}
            validate={(text) =>
              TAG_NAME.test(text.trim()) ? null : t('admin.wishes.users.profiles.invalid')
            }
          />
          <div className={styles.actions}>
            <AriaButton
              type="submit"
              variant="primary"
              size="sm"
              isPending={save.isPending && save.variables !== null}
              aria-label={t('admin.wishes.users.profiles.saveLabel', { profile: profile.name })}
            >
              {t('admin.wishes.users.profiles.save')}
            </AriaButton>
            {profile.tagNameCustom && (
              <AriaButton
                variant="ghost"
                size="sm"
                isPending={save.isPending && save.variables === null}
                aria-label={t('admin.wishes.users.profiles.resetLabel', { profile: profile.name })}
                onPress={() => save.mutate(null)}
              >
                {t('admin.wishes.users.profiles.reset')}
              </AriaButton>
            )}
            <AriaButton variant="ghost" size="sm" onPress={close}>
              {t('admin.wishes.users.profiles.cancel')}
            </AriaButton>
          </div>
          {save.error && !refused ? (
            <Notice tone="danger">{problemMessage(t, save.error)}</Notice>
          ) : null}
        </Form>
      )}
      <StatusRegion message={message} />
    </li>
  );
}

/** The profiles of an account with their tag names in Radarr and Sonarr (M-24, N-113). */
export function ProfileTags({
  mapping,
  drafts,
  tagsOn,
}: {
  mapping: WishAccountMapping;
  drafts: Drafts<string>;
  /** «Profil-Tag in Radarr und Sonarr setzen» is on. */
  tagsOn: boolean;
}) {
  const { t } = useTranslation();
  const key = String(mapping.accountId);
  const profiles = mapping.profiles ?? [];
  return (
    <section className={styles.block} aria-labelledby={`profiles-${key}`}>
      <h3 id={`profiles-${key}`} className={styles.blockTitle}>
        {t('admin.wishes.users.profiles.title')}
      </h3>
      <Text variant="caption" tone="secondary">
        {tagsOn ? t('admin.wishes.users.profiles.help') : t('admin.wishes.users.profiles.tagsOff')}
      </Text>
      {profiles.length === 0 ? (
        <Text tone="secondary">{t('admin.wishes.users.profiles.none')}</Text>
      ) : (
        <ul className={styles.profiles}>
          {profiles.map((profile) => (
            <ProfileTag key={profile.profileId} profile={profile} drafts={drafts} />
          ))}
        </ul>
      )}
    </section>
  );
}

/**
 * «n Anfragen aus Seerr übernehmen» for an account with its Seerr user (F-21): the count comes
 * from Seerr before anything is taken over; the take-over runs as a job (202).
 */
export function TakeOver({ seerrUserId, name }: { seerrUserId: number; name: string }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const count = useQuery(seerrRequestCountQuery(seerrUserId));
  const [message, say] = useStatusMessage();
  const take = useMutation({
    mutationFn: () => importSeerrRequests(seerrUserId),
    onMutate: () => say(null),
    onSuccess: async () => {
      say(t('admin.wishes.users.takeOver.queued', { count: count.data?.count ?? 0, name }));
      await queryClient.invalidateQueries({ queryKey: wishKeys.all });
    },
  });
  return (
    <section className={styles.block} aria-labelledby={`take-over-${seerrUserId}`}>
      <h3 id={`take-over-${seerrUserId}`} className={styles.blockTitle}>
        {t('admin.wishes.users.takeOver.title')}
      </h3>
      <Text variant="caption" tone="secondary">
        {t('admin.wishes.users.takeOver.help')}
      </Text>
      {count.isPending && <Text tone="secondary">{t('admin.wishes.users.takeOver.counting')}</Text>}
      {count.isError && <Notice tone="danger">{problemMessage(t, count.error)}</Notice>}
      {count.data && count.data.count === 0 && (
        <Text tone="secondary">{t('admin.wishes.users.takeOver.none')}</Text>
      )}
      {count.data && count.data.count > 0 && (
        <div className={styles.actions}>
          <AriaButton icon="download" isPending={take.isPending} onPress={() => take.mutate()}>
            {t('admin.wishes.users.takeOver.action', { count: count.data.count })}
          </AriaButton>
        </div>
      )}
      {take.error ? <Notice tone="danger">{problemMessage(t, take.error)}</Notice> : null}
      <StatusRegion message={message} />
    </section>
  );
}
