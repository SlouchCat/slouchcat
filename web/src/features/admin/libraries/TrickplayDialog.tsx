import type { Library, LibraryTrickplay } from '@slouchcat/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { formatBytes } from '@/lib/format';
import { Notice, Skeleton } from '@/ui';
import { AriaButton, Form, FormDialog } from '@/ui/aria';
import { runJobType } from '../queries';
import {
  DEFAULT_TRICKPLAY,
  sameTrickplay,
  trickplayStores,
  TrickplayFields,
} from './TrickplayFields';
import styles from './Libraries.module.css';

/** The job type that makes the missing previews of every library «Im Voraus» (6.3, F-5). */
export const TRICKPLAY_BACKFILL_TYPE = 'trickplay.backfill';

interface TrickplayDialogProps {
  library: Library;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  onSaved: (library: Library) => void;
}

/**
 * «Vorschau beim Spulen» of a video library (concept 18, 9): the film and the series group with
 * the estimate of `getLibrary`, a warning when the change drops stored previews (F-12), and
 * «Jetzt erzeugen», which queues the backfill of every library «Im Voraus» (the button of the jobs
 * page, F-5). The page mounts it afresh at each opening, so it starts at the library's setting.
 * The server decides what happens with the setting; the dialog only says it.
 */
export function TrickplayDialog({ library, isOpen, onOpenChange, onSaved }: TrickplayDialogProps) {
  const { t, i18n } = useTranslation();
  const saved = library.trickplay ?? DEFAULT_TRICKPLAY;
  const [value, setValue] = useState<LibraryTrickplay>(saved);
  const detail = useQuery({
    queryKey: ['admin', 'library', library.id],
    queryFn: ({ signal }): Promise<Library> =>
      request(api.GET('/admin/libraries/{id}', { params: { path: { id: library.id } }, signal })),
    enabled: isOpen,
  });
  const estimate = detail.data?.trickplayEstimate;
  const save = useMutation({
    mutationFn: (trickplay: LibraryTrickplay): Promise<Library> =>
      request(
        api.PATCH('/admin/libraries/{id}', {
          params: { path: { id: library.id } },
          body: { trickplay },
        }),
      ),
    onSuccess: (updated) => {
      onOpenChange(false);
      onSaved(updated);
    },
  });
  const generate = useMutation({ mutationFn: () => runJobType(TRICKPLAY_BACKFILL_TYPE) });
  const errors = save.error ? formErrors(t, save.error) : null;

  // Stored previews of a kind go when its setting stops storing (6.6).
  const drops = (kind: keyof LibraryTrickplay) =>
    trickplayStores(saved[kind]) && !trickplayStores(value[kind]);
  const size = (kind: keyof LibraryTrickplay) =>
    estimate ? formatBytes(estimate[kind].storedBytes, i18n.language) : undefined;
  const dropMovies = drops('movies');
  const dropSeries = drops('series');
  const warning =
    dropMovies && dropSeries
      ? t('admin.libraries.trickplay.drop.both', {
          movies: size('movies') ?? '–',
          series: size('series') ?? '–',
        })
      : dropMovies
        ? t('admin.libraries.trickplay.drop.movies', { size: size('movies') ?? '–' })
        : dropSeries
          ? t('admin.libraries.trickplay.drop.series', { size: size('series') ?? '–' })
          : null;
  // The backfill makes previews only for settings «Im Voraus» that are saved.
  const canGenerate = saved.movies.mode === 'ahead' || saved.series.mode === 'ahead';

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (sameTrickplay(value, saved)) {
      onOpenChange(false);
      return;
    }
    save.mutate(value);
  };

  return (
    <FormDialog
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      title={t('admin.libraries.trickplay.title', { name: library.name })}
      description={t('admin.libraries.trickplay.description')}
      size="wide"
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TrickplayFields value={value} onChange={setValue} estimate={estimate} />
        {detail.isPending && <Skeleton shape="line" />}
        {detail.isError && (
          <Notice tone="warning" live={false}>
            {t('admin.libraries.trickplay.estimate.unavailable')}
          </Notice>
        )}
        {warning && (
          <Notice tone="warning" live={false}>
            {warning}
          </Notice>
        )}
        {canGenerate && (
          <div className={styles.trickplayGenerate}>
            <AriaButton
              icon="refresh"
              isPending={generate.isPending}
              onPress={() => generate.mutate()}
            >
              {t('admin.libraries.trickplay.generate.action')}
            </AriaButton>
            <span className={styles.muted}>{t('admin.libraries.trickplay.generate.hint')}</span>
          </div>
        )}
        {generate.isSuccess && <Notice>{t('admin.libraries.trickplay.generate.queued')}</Notice>}
        {generate.isError && <Notice tone="danger">{problemMessage(t, generate.error)}</Notice>}
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={() => onOpenChange(false)}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" isPending={save.isPending}>
            {t('admin.libraries.trickplay.submit')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
