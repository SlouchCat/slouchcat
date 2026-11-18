import type { ItemRef } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { quotaText } from '@/features/shared/quota';
import { intlLocale } from '@/i18n';
import { formatDate } from '@/lib/format';
import { Notice, Skeleton, Text, useSpatialHold } from '@/ui';
import { AriaButton, CheckboxGroup, focusOnMount, Form, FormDialog } from '@/ui/aria';
import {
  useCreateWish,
  wishQuotaQuery,
  wishSeasonsQuery,
  type ExternalTitleRef,
  type Wish,
  type WishCreate,
  type WishQuota,
  type WishSeasons,
} from './queries';
import { availableItem, wishProblem, wishProblemMessage } from './state';
import styles from './Wishes.module.css';

/** The choice «Alle fehlenden Staffeln» in the list of seasons. */
const ALL = 'all';

type Season = WishSeasons['seasons'][number];

interface RequestDialogProps {
  /** The title outside the library (preview page, search section, «Meine Wünsche»). */
  title: Pick<ExternalTitleRef, 'media' | 'tmdbId' | 'title' | 'year'>;
  /** Where the request comes from (WishCreate.source). */
  source: NonNullable<WishCreate['source']>;
  /** Whether Seerr approves the requests at once, when the page knows it (the preview's card). */
  autoApprove?: boolean;
  onClose: () => void;
  /** The wish was made or joined; the dialog closes. */
  onDone: (wish: Wish) => void;
  /** The title arrived in the library meanwhile (409 `already_available`). */
  onAvailable: (item: ItemRef) => void;
  /** The seasons chosen at first (a missing season pressed on the series page); else all. */
  preselect?: readonly number[];
  /** Seasons the dialog leaves out (announced ones on the series page, concept 25 M-06). */
  skip?: readonly number[];
}

/** The limit's warning when the chosen titles do not fit: none while they fit or it is unknown. */
function limitRefusal(
  t: TFunction,
  language: string,
  media: ExternalTitleRef['media'],
  quota: WishQuota | undefined,
  needed: number,
): string | null {
  const part = media === 'movie' ? quota?.movie : quota?.tv;
  if (!part || needed === 0 || part.remaining >= needed) return null;
  const date = part.nextFreeAt ? formatDate(part.nextFreeAt, language) : undefined;
  if (part.remaining > 0) {
    return t('wishes.dialog.tooManySeasons', { count: part.remaining, limit: part.limit });
  }
  return t(`wishes.dialog.reached.${media}`, {
    count: part.limit,
    days: part.days,
    date,
    context: date ? 'date' : undefined,
  });
}

/**
 * The dialog «Anfordern» (concept 25, M-13, F-10): a film is confirmed, a series chooses its
 * seasons from Seerr's list (missing ones request, «bereits angefordert» ones join another
 * profile's wish, available ones are locked), with the limit of the account and its refusal. It
 * holds the page's arrow keys while open; Escape and «Abbrechen» close it and the focus returns to
 * the button that opened it.
 */
export function RequestDialog({
  title,
  source,
  autoApprove,
  onClose,
  onDone,
  onAvailable,
  preselect,
  skip,
}: RequestDialogProps) {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  useSpatialHold(true);
  const series = title.media === 'show';
  const seasons = useQuery({ ...wishSeasonsQuery(title.tmdbId), enabled: series });
  const quotaRead = useQuery(wishQuotaQuery);
  // The result reaches the page also when the dialog was closed while the request was on its way.
  const create = useCreateWish({
    onSuccess: onDone,
    onError: (error) => {
      const item = availableItem(error);
      if (item) onAvailable(item);
    },
  });
  const form = useRef<HTMLFormElement>(null);
  const offered: readonly Season[] = (seasons.data?.seasons ?? []).filter(
    (season) => !skip?.includes(season.number),
  );
  const choosable = offered.filter((season) => season.state !== 'available');
  const [chosen, setChosen] = useState<readonly string[] | null>(null);
  // At first every season that can be chosen is chosen («Alle fehlenden Staffeln», concept 4.2),
  // or only the one pressed on the series page.
  const picked =
    chosen ??
    choosable
      .filter((season) => !preselect || preselect.includes(season.number))
      .map((season) => String(season.number));
  const all = choosable.length > 0 && picked.length === choosable.length;
  const fresh = offered.filter(
    (season) => season.state === 'missing' && picked.includes(String(season.number)),
  ).length;
  // A refused request names the limit as it stood then and the places it needed (the server
  // counts the seasons); otherwise the stored limit and the client's count.
  const refused = wishProblem(create.error);
  const overQuota = refused?.code === 'quota_exceeded';
  const quota = overQuota && refused.quota ? refused.quota : quotaRead.data;
  const needed = overQuota && refused.needed !== undefined ? refused.needed : series ? fresh : 1;
  const refusal = limitRefusal(t, language, title.media, quota, needed);
  // «Alle fehlenden Staffeln» only while the dialog left no season out; else the chosen ones.
  const allSeasons = all && offered.length === (seasons.data?.seasons.length ?? 0);
  const loaded = !series || seasons.data !== undefined;

  // The seasons arrive after the dialog opened: the focus goes to the first choice then
  // (DownloadDialog does the same), not to the dialog itself.
  useEffect(() => {
    const element = form.current;
    if (!series || !loaded || !element) return;
    const active = document.activeElement;
    const onDialog =
      !active || active === document.body || active.getAttribute('role') === 'dialog';
    if (!onDialog || element.contains(active)) return;
    // The pressed season's box when there is one, else the first choice.
    const first = preselect?.[0];
    const pressed =
      first === undefined
        ? null
        : element.querySelector<HTMLElement>(`input[value="${first}"]:not(:disabled)`);
    (pressed ?? element.querySelector<HTMLElement>('input:not(:disabled), button'))?.focus();
  }, [series, loaded, preselect]);

  const onChange = (next: string[]) => {
    // A new choice: the refusal of the last one no longer speaks for it.
    create.reset();
    const wasAll = picked.includes(ALL) || all;
    const nowAll = next.includes(ALL);
    if (nowAll !== wasAll) {
      setChosen(nowAll ? choosable.map((season) => String(season.number)) : []);
      return;
    }
    setChosen(next.filter((value) => value !== ALL));
  };

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (create.isPending || refusal) return;
    const body: WishCreate = { media: title.media, tmdbId: title.tmdbId, source };
    if (series) {
      if (allSeasons) body.allSeasons = true;
      else body.seasons = picked.map(Number).sort((a, b) => a - b);
    }
    create.mutate(body);
  };

  // A refused limit speaks through its warning; when the stored limit cannot say why, the
  // server's message does. Field errors go to their field, as in the other form dialogs.
  const errors = create.error && !(overQuota && refusal) ? formErrors(t, create.error) : null;
  const failure = errors?.form ? wishProblemMessage(t, create.error) : null;
  const chosenCount = picked.length;
  let submit: string;
  if (create.isPending) submit = t('wishes.dialog.submitting');
  else if (!series) submit = t('wishes.dialog.submit');
  else if (chosenCount === 0) submit = t('wishes.dialog.chooseSeasons');
  else if (fresh === 0) submit = t('wishes.dialog.join');
  else submit = t('wishes.dialog.submitSeasons', { count: chosenCount });

  const seasonText = (season: Season) => {
    const parts = [t('wishes.dialog.episodes', { count: season.episodeCount })];
    if (season.state !== 'missing') parts.push(t(`wishes.dialog.seasonState.${season.state}`));
    return parts.join(t('format.separator'));
  };
  const limitLine = quota ? quotaText(t, language, series ? 'tv' : 'movie', quota) : '';

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t(series ? 'wishes.dialog.titleShow' : 'wishes.dialog.titleMovie', {
        title: title.title,
      })}
      description={
        title.year
          ? t('wishes.dialog.intro', { year: title.year, kind: t(`wishes.kind.${title.media}`) })
          : t(`wishes.kind.${title.media}`)
      }
      initialFocus={series ? undefined : '[type="submit"]'}
    >
      {series && seasons.isPending && <Skeleton shape="block" />}
      {series && seasons.isError && (
        <div className={styles.form}>
          <Notice tone="danger">{wishProblemMessage(t, seasons.error)}</Notice>
          <div className={styles.formActions}>
            <AriaButton ref={focusOnMount} variant="ghost" onPress={onClose}>
              {t('common.close')}
            </AriaButton>
          </div>
        </div>
      )}
      {loaded && (
        <Form
          ref={form}
          className={styles.form}
          onSubmit={onSubmit}
          validationErrors={errors?.fields}
        >
          {failure && <Notice tone="danger">{failure}</Notice>}
          {autoApprove !== undefined && (
            <Text tone="secondary" variant="caption">
              {t(autoApprove ? 'wishes.dialog.autoApprove' : 'wishes.dialog.needsApproval')}
            </Text>
          )}
          {series &&
            (choosable.length > 0 ? (
              <CheckboxGroup
                name="seasons"
                label={t('wishes.dialog.seasons')}
                value={all ? [ALL, ...picked] : [...picked]}
                onChange={onChange}
                options={[
                  {
                    value: ALL,
                    label: t('wishes.dialog.allSeasons', { count: choosable.length }),
                  },
                  ...offered.map((season) => ({
                    value: String(season.number),
                    label: t('wishes.dialog.season', { number: season.number }),
                    description: seasonText(season),
                    isDisabled: season.state === 'available',
                  })),
                ]}
              />
            ) : (
              <Notice title={t('wishes.dialog.nothingLeft.title')}>
                {t('wishes.dialog.nothingLeft.body')}
              </Notice>
            ))}
          {limitLine && (
            <Text tone="secondary" variant="caption">
              {limitLine}
            </Text>
          )}
          {refusal && <Notice tone="warning" title={refusal} />}
          <div className={styles.formActions}>
            <AriaButton variant="ghost" onPress={onClose}>
              {t('common.cancel')}
            </AriaButton>
            <AriaButton
              type="submit"
              variant="primary"
              icon={series && fresh === 0 && chosenCount > 0 ? 'bell' : 'plus'}
              isDisabled={refusal !== null || (series && chosenCount === 0)}
              isPending={create.isPending}
            >
              {submit}
            </AriaButton>
          </div>
        </Form>
      )}
    </FormDialog>
  );
}
