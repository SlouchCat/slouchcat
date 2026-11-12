import type { ItemDeletionPreview, ItemDeletionResult } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { formatBytes } from '@/lib/format';
import { Notice } from '@/ui';
import { AriaButton, FormDialog, RadioGroup, TextField } from '@/ui/aria';
import { fullPath } from '../maintenance/shared';
import { itemDeletionQuery } from '../queries';
import styles from './DeleteItemDialog.module.css';

/** The kinds whose files the dialog names by folder (series, season, podcast, section 10.2). */
const FOLDER_KINDS = new Set(['series', 'season', 'podcast']);

type Mode = 'library';

interface DeleteItemDialogProps {
  itemId: string;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  /** After the server deleted the title; the holder leaves the page or closes the editor. */
  onDeleted: (result: ItemDeletionResult, preview: ItemDeletionPreview, message: string) => void;
}

/**
 * «Löschen» of a title (concept 20 P-9, mockup «Jobs, Löschen, Duplikate»): the dialog reads the
 * preview of the server each time it opens and asks what should happen. It names the files a film
 * has with their whole path and size, the folder, the number and the total of a series, season or
 * podcast. With more than one file the title has to be typed (any case); while a profile plays one
 * of the titles the dialog names it and the button stays off (O-13). The server decides every
 * rule again; the dialog only shows its answer.
 */
export function DeleteItemDialog({
  itemId,
  isOpen,
  onOpenChange,
  onDeleted,
}: DeleteItemDialogProps) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const preview = useQuery({ ...itemDeletionQuery(itemId), enabled: isOpen });
  const [mode, setMode] = useState<Mode>('library');
  const [typed, setTyped] = useState('');
  const data = preview.data;
  const remove = useMutation({
    mutationFn: (body: { mode: Mode; confirmTitle?: string }): Promise<ItemDeletionResult> =>
      request(api.POST('/admin/items/{id}/deletion', { params: { path: { id: itemId } }, body })),
    onSuccess: async (result) => {
      if (!data) return;
      setTyped('');
      await queryClient.invalidateQueries();
      onDeleted(result, data, t(`admin.itemDelete.done.${mode}`, { title: data.confirmTitle }));
    },
  });
  const close = (open: boolean) => {
    if (!open) {
      setTyped('');
      remove.reset();
    }
    onOpenChange(open);
  };

  const playing = data?.playingProfiles ?? [];
  const matches =
    !data?.typeTitle || typed.trim().toLowerCase() === data.confirmTitle.trim().toLowerCase();
  const size = formatBytes(data?.totalBytes ?? 0, i18n.language);
  const byFolder = data !== undefined && FOLDER_KINDS.has(data.kind) && data.folder !== undefined;

  return (
    // The dialog opens with the preview, so its first field takes the focus (FormDialog focuses
    // once, when its content mounts); the request is a small local read.
    <FormDialog
      isOpen={isOpen && !preview.isPending}
      onOpenChange={close}
      title={
        data
          ? t('admin.itemDelete.title', { title: data.confirmTitle })
          : t('admin.itemDelete.loading')
      }
      size="wide"
    >
      {preview.isError && <Notice tone="danger" title={problemMessage(t, preview.error)} />}
      {data && (
        <form
          className={styles.form}
          onSubmit={(event) => {
            event.preventDefault();
            if (playing.length > 0 || !matches) return;
            remove.mutate({ mode, ...(data.typeTitle ? { confirmTitle: typed.trim() } : {}) });
          }}
        >
          {playing.length > 0 && (
            <Notice
              tone="danger"
              title={t('admin.itemDelete.playing', {
                names: playing.join(t('format.listSeparator')),
              })}
            />
          )}
          <RadioGroup
            label={t('admin.itemDelete.choice')}
            orientation="vertical"
            value={mode}
            onChange={(value) => setMode(value as Mode)}
            options={[{ value: 'library', label: t('admin.itemDelete.mode.library') }]}
            description={t('admin.itemDelete.libraryHint')}
          />
          <section className={styles.files} aria-label={t('admin.itemDelete.files')}>
            <p className={styles.affected}>
              {data.fileCount === 0
                ? t('admin.itemDelete.noFiles')
                : t('admin.itemDelete.affected', { count: data.fileCount, size })}
            </p>
            {byFolder && data.folder ? (
              <p className={styles.path}>
                <span className={styles.muted}>{t('admin.itemDelete.folder')}: </span>
                {fullPath(data.folder.rootPath, data.folder.path, true)}
              </p>
            ) : (
              data.files.length > 0 && (
                <ul className={styles.list}>
                  {data.files.map((file) => (
                    <li key={`${file.rootPath}/${file.path}`} className={styles.file}>
                      <span className={styles.path}>{fullPath(file.rootPath, file.path)}</span>
                      <span className={styles.muted}>
                        {formatBytes(file.sizeBytes, i18n.language)}
                      </span>
                    </li>
                  ))}
                </ul>
              )
            )}
          </section>
          <p className={styles.note}>{t('admin.itemDelete.everywhere')}</p>
          <p className={styles.note}>{t('admin.itemDelete.arr')}</p>
          {data.typeTitle && (
            <TextField
              label={t('admin.itemDelete.typeLabel')}
              value={typed}
              onChange={setTyped}
              autoComplete="off"
              spellCheck="false"
              description={
                matches ? undefined : t('admin.itemDelete.typeHint', { title: data.confirmTitle })
              }
            />
          )}
          {remove.isError && <Notice tone="danger" title={problemMessage(t, remove.error)} />}
          <div className={styles.actions}>
            <AriaButton variant="ghost" onPress={() => close(false)}>
              {t('common.cancel')}
            </AriaButton>
            <AriaButton
              type="submit"
              variant="danger"
              isDisabled={playing.length > 0 || !matches}
              isPending={remove.isPending}
            >
              {t(`admin.itemDelete.confirm.${mode}`)}
            </AriaButton>
          </div>
        </form>
      )}
    </FormDialog>
  );
}
